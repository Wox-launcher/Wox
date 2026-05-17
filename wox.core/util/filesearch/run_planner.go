package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wox/util"
)

// RunPlanner builds one sealed indexing workload shape in memory before execution.
// The previous root-centric flow mixed discovery, recursive counting, and writes
// in one loop. The current planner only seals ownership boundaries; streaming
// execution owns the real recursive walk and live file counts.
type RunPlanner struct {
	policy         *policyState
	budget         splitBudget
	onProgress     func(RunPlannerProgress)
	rootExclusions map[string][]string
	// Tests use this hook to assert when subtree scans actually hit the
	// filesystem, so planner optimizations can prove they removed redundant
	// rescans without changing the sealed plan shape.
	onSubtreeScan func(scopePath string)

	// planningRootBuffers are intentionally released after sealing so the planner
	// does not retain a second copy of a giant scope tree through execution.
	planningRootBuffers []*runPlannerRootBuffer
}

// RunPlannerProgress reports the planner-owned stages before execution starts.
// Execution progress is reported elsewhere, so this lightweight callback only
// exists to surface planning as a first-class phase before streaming execution.
type RunPlannerProgress struct {
	Stage     RunStage
	Root      RootRecord
	RootIndex int
	RootTotal int
	ScopePath string
}

type runPlannerRootBuffer struct {
	root      RootRecord
	rootScope *runPlannerScopeBuffer
}

type runPlannerScopeBuffer struct {
	scopePath       string
	scopeKind       ScopeKind
	parentScopePath string
	totals          PlanTotals
	splitRequired   bool
	children        []*runPlannerScopeBuffer
}

func NewRunPlanner(policy *policyState) *RunPlanner {
	if policy == nil {
		policy = newPolicyState(Policy{})
	}
	return &RunPlanner{
		policy:         policy,
		budget:         defaultSplitBudget(),
		rootExclusions: map[string][]string{},
	}
}

func (p *RunPlanner) SetProgressCallback(callback func(RunPlannerProgress)) {
	if p == nil {
		return
	}
	p.onProgress = callback
}

func (p *RunPlanner) SetRootExclusions(exclusions map[string][]string) {
	if p == nil {
		return
	}
	p.rootExclusions = copyRootExclusions(exclusions)
}

func (p *RunPlanner) PlanFullRun(ctx context.Context, roots []RootRecord) (RunPlan, error) {
	if p == nil {
		p = NewRunPlanner(nil)
	}
	if p.policy == nil {
		p.policy = newPolicyState(Policy{})
	}

	// Phase 1: planning. The old root-centric loop only knew about one whole
	// root at execution time. We still build the structural frontier here, but
	// leave recursive counting to the streaming executor so large roots are not
	// walked twice before search results become available.
	rootBuffers, err := p.planRoots(ctx, roots)
	if err != nil {
		return RunPlan{}, err
	}
	p.planningRootBuffers = rootBuffers

	for _, rootBuffer := range p.planningRootBuffers {
		if rootBuffer == nil || rootBuffer.rootScope == nil {
			continue
		}
		// Optimization: full indexing uses estimated work units because the real
		// file count is discovered by the streaming executor. Splitting the root
		// into direct files plus top-level subtree leaves keeps execution
		// diagnostics useful while still avoiding the duplicate count-then-apply
		// filesystem walk.
		applyEstimatedTotalsForStreamingScopes(rootBuffer.rootScope)
	}

	// Phase 2: seal. We convert the planner-owned buffers into immutable plan
	// structs, deep-copy them with RunPlan.Seal, then drop the planner buffers so
	// execution does not keep the same giant scope tree alive twice.
	draft, err := p.buildDraftPlan(RunKindFull, "full-plan", "full-run")
	if err != nil {
		p.planningRootBuffers = nil
		return RunPlan{}, err
	}
	sealed := draft.Seal()
	p.planningRootBuffers = nil

	return sealed, nil
}

func streamingRunEstimatedTotals() PlanTotals {
	return PlanTotals{
		DirectoryCount:      1,
		IndexableEntryCount: 1,
		PlannedScanUnits:    1,
		PlannedWriteUnits:   1,
	}
}

func applyEstimatedTotalsForStreamingScopes(scope *runPlannerScopeBuffer) PlanTotals {
	if scope == nil {
		return PlanTotals{}
	}
	if len(scope.children) == 0 {
		scope.totals = streamingRunEstimatedTotals()
		return scope.totals
	}

	totals := PlanTotals{}
	for _, child := range scope.children {
		totals = mergePlanTotals(totals, applyEstimatedTotalsForStreamingScopes(child))
	}
	scope.totals = totals
	return totals
}

func (p *RunPlanner) PlanIncrementalRun(ctx context.Context, roots []RootRecord, batches []ReconcileBatch) (RunPlan, error) {
	if p == nil {
		p = NewRunPlanner(nil)
	}
	if p.policy == nil {
		p.policy = newPolicyState(Policy{})
	}

	rootBuffers, err := p.planIncrementalRoots(ctx, roots, batches)
	if err != nil {
		return RunPlan{}, err
	}
	p.planningRootBuffers = rootBuffers

	for _, rootBuffer := range p.planningRootBuffers {
		if rootBuffer == nil || rootBuffer.rootScope == nil {
			continue
		}
		// Optimization: incremental indexing also skips exact planning. Dirty
		// scopes are already the caller's best available boundary, so walking them
		// once to count work and again to apply work only delays reconciliation.
		applyEstimatedTotalsForStreamingScopes(rootBuffer.rootScope)
	}

	draft, err := p.buildDraftPlan(RunKindIncremental, "incremental-plan", "incremental-run")
	if err != nil {
		p.planningRootBuffers = nil
		return RunPlan{}, err
	}
	sealed := draft.Seal()
	p.planningRootBuffers = nil

	return sealed, nil
}

func (p *RunPlanner) planRoots(ctx context.Context, roots []RootRecord) ([]*runPlannerRootBuffer, error) {
	buffers := make([]*runPlannerRootBuffer, 0, len(roots))
	for index, root := range roots {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		p.emitProgress(RunPlannerProgress{
			Stage:     RunStagePlanning,
			Root:      root,
			RootIndex: index + 1,
			RootTotal: len(roots),
			ScopePath: filepath.Clean(root.Path),
		})

		buffer, err := p.planRoot(ctx, root)
		if err != nil {
			return nil, wrapRunPlannerRootError(root.ID, err)
		}
		buffers = append(buffers, buffer)
	}
	return buffers, nil
}

func (p *RunPlanner) planIncrementalRoots(ctx context.Context, roots []RootRecord, batches []ReconcileBatch) ([]*runPlannerRootBuffer, error) {
	rootByID := make(map[string]RootRecord, len(roots))
	for _, root := range roots {
		rootByID[root.ID] = root
	}

	type rootDraft struct {
		root      RootRecord
		rootScope *runPlannerScopeBuffer
		children  []*runPlannerScopeBuffer
	}

	rootDrafts := make(map[string]*rootDraft, len(batches))
	rootOrder := make([]string, 0, len(batches))
	for _, batch := range batches {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		root, ok := rootByID[batch.RootID]
		if !ok {
			return nil, fmt.Errorf("incremental planner root %q not found", batch.RootID)
		}

		draft, exists := rootDrafts[root.ID]
		if !exists {
			draft = &rootDraft{root: root}
			rootDrafts[root.ID] = draft
			rootOrder = append(rootOrder, root.ID)
		}

		if batch.Mode == ReconcileModeRoot || len(batch.Paths) == 0 {
			// Bug fix: a root-level incremental reconcile used to collapse the
			// whole configured root into one subtree job. For large home roots that
			// made an "incremental" pass slower than a full pass, because full
			// planning already splits the root into top-level streaming scopes.
			// Reusing the full root planner here keeps root-reconcile semantics while
			// preserving the same bounded job shape as full indexing.
			rootBuffer, err := p.planRoot(ctx, root)
			if err != nil {
				return nil, wrapRunPlannerRootError(root.ID, err)
			}
			draft.rootScope = rootBuffer.rootScope
			draft.children = nil
			continue
		}
		if draft.rootScope != nil {
			// A root-level reconcile supersedes any narrower dirty paths already
			// coalesced for this root. Keep the broader split plan so the final
			// reconcile still prunes stale rows across the full root.
			continue
		}

		for _, scopePath := range batch.Paths {
			cleanScope := filepath.Clean(scopePath)
			if !pathWithinScope(root.Path, cleanScope) {
				return nil, &runRootError{
					RootID: root.ID,
					Err:    fmt.Errorf("incremental scope path %q is outside root path %q", cleanScope, root.Path),
				}
			}
			scopeKind := ScopeKindSubtree
			if batch.Mode == ReconcileModeSubtree && cleanScope == filepath.Clean(root.Path) {
				// DirtyQueue collapses file changes to their parent directory. When
				// that parent is the configured root, the smallest correct retry is a
				// direct-files job for the root directory, not a recursive root scan.
				scopeKind = ScopeKindDirectFiles
			}
			draft.children = append(draft.children, &runPlannerScopeBuffer{
				scopePath:       cleanScope,
				scopeKind:       scopeKind,
				parentScopePath: filepath.Clean(root.Path),
			})
		}
	}

	buffers := make([]*runPlannerRootBuffer, 0, len(rootOrder))
	for index, rootID := range rootOrder {
		draft := rootDrafts[rootID]
		if draft == nil {
			continue
		}

		p.emitProgress(RunPlannerProgress{
			Stage:     RunStagePlanning,
			Root:      draft.root,
			RootIndex: index + 1,
			RootTotal: len(rootOrder),
			ScopePath: filepath.Clean(draft.root.Path),
		})

		rootScope := draft.rootScope
		if rootScope == nil {
			rootScope = &runPlannerScopeBuffer{
				scopePath: filepath.Clean(draft.root.Path),
				scopeKind: ScopeKindSubtree,
			}
		}

		if draft.rootScope == nil && len(draft.children) == 1 && filepath.Clean(draft.children[0].scopePath) == filepath.Clean(draft.root.Path) {
			rootScope = draft.children[0]
		} else if draft.rootScope == nil && len(draft.children) > 0 {
			rootScope.splitRequired = true
			rootScope.children = dedupePlannerScopes(draft.children)
		}

		buffers = append(buffers, &runPlannerRootBuffer{
			root:      draft.root,
			rootScope: rootScope,
		})
	}

	return buffers, nil
}

func (p *RunPlanner) planRoot(ctx context.Context, root RootRecord) (*runPlannerRootBuffer, error) {
	cleanRootPath := filepath.Clean(root.Path)
	if root.ID == "" {
		return nil, fmt.Errorf("root id is required")
	}
	if root.Path == "" {
		return nil, fmt.Errorf("root path is required")
	}
	info, err := os.Stat(cleanRootPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path %q is not a directory", cleanRootPath)
	}

	rootScope := &runPlannerScopeBuffer{
		scopePath: cleanRootPath,
		scopeKind: ScopeKindSubtree,
	}
	children, err := p.planRootTopLevelScopes(ctx, root, cleanRootPath)
	if err != nil {
		return nil, err
	}
	if len(children) > 0 {
		// Optimization: a default home root should not execute as one opaque
		// /Users/name job. Top-level leaves expose which folders are slow and let the
		// streaming walker process each accepted subtree independently.
		rootScope.splitRequired = true
		rootScope.children = children
	}

	return &runPlannerRootBuffer{
		root:      root,
		rootScope: rootScope,
	}, nil
}

func (p *RunPlanner) planRootTopLevelScopes(ctx context.Context, root RootRecord, cleanRootPath string) ([]*runPlannerScopeBuffer, error) {
	children := []*runPlannerScopeBuffer{{
		scopePath:       cleanRootPath,
		scopeKind:       ScopeKindDirectFiles,
		parentScopePath: cleanRootPath,
	}}

	dirEntries, err := os.ReadDir(cleanRootPath)
	if err != nil {
		return nil, fmt.Errorf("read root top-level scopes %q: %w", cleanRootPath, err)
	}

	policyContext := p.policy.newTraversalContext(root, cleanRootPath)
	for _, dirEntry := range dirEntries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		childPath := filepath.Join(cleanRootPath, dirEntry.Name())
		isDir, _, infoErr := strictDirEntryType(cleanRootPath, dirEntry)
		if infoErr != nil {
			if os.IsNotExist(infoErr) {
				continue
			}
			if shouldSkipUnreadableTraversalError(infoErr) {
				util.GetLogger().Warn(ctx, "filesearch skipped unreadable root child "+childPath+": "+infoErr.Error())
				continue
			}
			return nil, infoErr
		}
		if !isDir {
			continue
		}
		if p.isExcludedPath(root.ID, childPath) {
			continue
		}
		// Optimization: the launcher's default ~/ root should not behave like
		// find(1) over protected app databases. The root-aware system path policy
		// prunes noisy home directories here, while explicit custom roots under the
		// same paths still produce their own top-level plan.
		if shouldSkipSystemPathForRoot(root, childPath, true) {
			continue
		}
		if !policyContext.ShouldIndexPath(childPath, true) {
			continue
		}

		children = append(children, &runPlannerScopeBuffer{
			scopePath:       childPath,
			scopeKind:       ScopeKindSubtree,
			parentScopePath: cleanRootPath,
		})
	}
	return children, nil
}

func (p *RunPlanner) isExcludedPath(rootID string, path string) bool {
	if p == nil || len(p.rootExclusions) == 0 {
		return false
	}
	for _, excludedPath := range p.rootExclusions[rootID] {
		if pathWithinScope(excludedPath, path) {
			return true
		}
	}
	return false
}

func (p *RunPlanner) buildDraftPlan(kind RunKind, planID string, runID string) (RunPlan, error) {
	plan := RunPlan{
		PlanID:    planID,
		RunID:     runID,
		Kind:      kind,
		RootPlans: make([]RootPlan, 0, len(p.planningRootBuffers)),
		Jobs:      make([]Job, 0),
	}
	orderIndex := 0
	for _, rootBuffer := range p.planningRootBuffers {
		if rootBuffer == nil || rootBuffer.rootScope == nil {
			continue
		}

		leafScopes := collectLeafScopes(rootBuffer.rootScope)
		rootPlan := RootPlan{
			RootID:             rootBuffer.root.ID,
			RootPath:           filepath.Clean(rootBuffer.root.Path),
			ScopeTree:          rootBuffer.rootScope.toScopeNode(),
			Totals:             rootBuffer.rootScope.totals,
			Jobs:               make([]JobRef, 0, len(leafScopes)+1),
			SplitPolicyVersion: runPlannerSplitPolicyVersionV1,
		}
		if len(leafScopes) <= 1 {
			rootPlan.Strategy = RootPlanStrategySingle
		} else {
			rootPlan.Strategy = RootPlanStrategySegmented
		}

		plan.EstimatedTotals = mergePlanTotals(plan.EstimatedTotals, rootPlan.Totals)

		// The grouped full-run leaf experiment did not reduce the dominant SQLite
		// cost enough to justify a wider multi-scope job contract. Returning to
		// one sealed job per planned leaf keeps the planner/executor/data path
		// straightforward while preserving the SQLite path index improvement that
		// did measurably reduce collect_diff cost.
		for _, leaf := range leafScopes {
			if leaf == nil || leaf.totals.IndexableEntryCount == 0 {
				continue
			}

			job := Job{
				JobID:             fmt.Sprintf("%s-job-%03d", rootBuffer.root.ID, orderIndex),
				RootID:            rootBuffer.root.ID,
				RootPath:          rootPlan.RootPath,
				ScopePath:         leaf.scopePath,
				Kind:              jobKindForScope(leaf.scopeKind),
				PlannedScanUnits:  leaf.totals.PlannedScanUnits,
				PlannedWriteUnits: leaf.totals.PlannedWriteUnits,
				Status:            JobStatusPending,
				OrderIndex:        orderIndex,
			}
			job.PlannedTotalUnits = job.PlannedScanUnits + job.PlannedWriteUnits
			plan.TotalWorkUnits += job.PlannedTotalUnits
			plan.Jobs = append(plan.Jobs, job)
			rootPlan.Jobs = append(rootPlan.Jobs, JobRef{
				JobID:      job.JobID,
				OrderIndex: job.OrderIndex,
			})
			orderIndex++
		}

		finalizeJob := Job{
			JobID:             fmt.Sprintf("%s-job-%03d", rootBuffer.root.ID, orderIndex),
			RootID:            rootBuffer.root.ID,
			RootPath:          rootPlan.RootPath,
			ScopePath:         rootPlan.RootPath,
			Kind:              JobKindFinalizeRoot,
			PlannedWriteUnits: 1,
			PlannedTotalUnits: 1,
			Status:            JobStatusPending,
			OrderIndex:        orderIndex,
		}
		plan.TotalWorkUnits += finalizeJob.PlannedTotalUnits
		plan.Jobs = append(plan.Jobs, finalizeJob)
		rootPlan.Jobs = append(rootPlan.Jobs, JobRef{
			JobID:      finalizeJob.JobID,
			OrderIndex: finalizeJob.OrderIndex,
		})
		orderIndex++

		plan.RootPlans = append(plan.RootPlans, rootPlan)
	}

	return plan, nil
}

func (p *RunPlanner) emitProgress(progress RunPlannerProgress) {
	if p == nil || p.onProgress == nil {
		return
	}
	p.onProgress(progress)
}

func wrapRunPlannerRootError(rootID string, err error) error {
	if err == nil || rootID == "" {
		return err
	}
	var rootErr *runRootError
	if errors.As(err, &rootErr) && rootErr != nil {
		return err
	}
	return &runRootError{
		RootID: rootID,
		Err:    err,
	}
}

func (s *runPlannerScopeBuffer) toScopeNode() *ScopeNode {
	if s == nil {
		return nil
	}

	node := &ScopeNode{
		ScopePath:           s.scopePath,
		ScopeKind:           s.scopeKind,
		ParentScopePath:     s.parentScopePath,
		DirectoryCount:      s.totals.DirectoryCount,
		FileCount:           s.totals.FileCount,
		IndexableEntryCount: s.totals.IndexableEntryCount,
		SkippedCount:        s.totals.SkippedCount,
		PlannedScanUnits:    s.totals.PlannedScanUnits,
		PlannedWriteUnits:   s.totals.PlannedWriteUnits,
		SplitRequired:       s.splitRequired,
	}
	if len(s.children) == 0 {
		return node
	}

	node.Children = make([]ScopeNode, 0, len(s.children))
	for _, child := range s.children {
		if child == nil {
			continue
		}
		sealedChild := child.toScopeNode()
		if sealedChild == nil {
			continue
		}
		node.Children = append(node.Children, *sealedChild)
	}
	return node
}

func collectLeafScopes(scope *runPlannerScopeBuffer) []*runPlannerScopeBuffer {
	if scope == nil {
		return nil
	}
	if len(scope.children) == 0 {
		return []*runPlannerScopeBuffer{scope}
	}

	leaves := make([]*runPlannerScopeBuffer, 0)
	for _, child := range scope.children {
		leaves = append(leaves, collectLeafScopes(child)...)
	}
	return leaves
}

func dedupePlannerScopes(scopes []*runPlannerScopeBuffer) []*runPlannerScopeBuffer {
	if len(scopes) == 0 {
		return nil
	}

	seen := make(map[string]*runPlannerScopeBuffer, len(scopes))
	order := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		key := filepath.Clean(scope.scopePath) + "|" + string(scope.scopeKind)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = scope
		order = append(order, key)
	}

	result := make([]*runPlannerScopeBuffer, 0, len(order))
	for _, key := range order {
		result = append(result, seen[key])
	}
	return result
}

func mergePlanTotals(left PlanTotals, right PlanTotals) PlanTotals {
	left.DirectoryCount += right.DirectoryCount
	left.FileCount += right.FileCount
	left.IndexableEntryCount += right.IndexableEntryCount
	left.SkippedCount += right.SkippedCount
	left.PlannedScanUnits += right.PlannedScanUnits
	left.PlannedWriteUnits += right.PlannedWriteUnits
	return left
}

func normalizeSplitBudget(budget splitBudget) splitBudget {
	defaults := defaultSplitBudget()
	if budget.LeafEntryBudget <= 0 {
		budget.LeafEntryBudget = defaults.LeafEntryBudget
	}
	if budget.LeafWriteBudget <= 0 {
		budget.LeafWriteBudget = defaults.LeafWriteBudget
	}
	if budget.LeafMemoryBudget <= 0 {
		budget.LeafMemoryBudget = defaults.LeafMemoryBudget
	}
	if budget.DirectFileBatchSize <= 0 {
		budget.DirectFileBatchSize = defaults.DirectFileBatchSize
	}
	return budget
}

func jobKindForScope(scopeKind ScopeKind) JobKind {
	switch scopeKind {
	case ScopeKindDirectFiles:
		return JobKindDirectFiles
	default:
		return JobKindSubtree
	}
}

func strictDirEntryInfo(parentPath string, dirEntry os.DirEntry) (os.FileInfo, error) {
	// Planning now only needs root-level structure, but unreadable metadata still
	// changes the sealed scope boundary. Failing fast here keeps the workload
	// shape truthful instead of pretending an unknown entry never existed.
	info, err := dirEntry.Info()
	if err != nil {
		return nil, fmt.Errorf("read metadata for %q: %w", filepath.Join(parentPath, dirEntry.Name()), err)
	}
	return info, nil
}

func strictDirEntryType(parentPath string, dirEntry os.DirEntry) (bool, os.FileInfo, error) {
	// Planner and snapshot traversals used to call Info() for every child even
	// when DirEntry.Type() already proved whether the child was a file or a
	// directory. Reusing the cheap type bits removes a large amount of metadata
	// I/O during planning and streaming walks, while symlinks and unknown entries
	// still fall back to Info() so the previous target-kind behavior stays intact.
	modeType := dirEntry.Type()
	if modeType != 0 && modeType&os.ModeSymlink == 0 {
		return dirEntry.IsDir(), nil, nil
	}

	info, err := strictDirEntryInfo(parentPath, dirEntry)
	if err != nil {
		return false, nil, err
	}
	return info.IsDir(), info, nil
}

func shouldSkipUnreadableTraversalError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "access is denied") ||
		strings.Contains(message, "permission denied") ||
		strings.Contains(message, "operation not permitted")
}
