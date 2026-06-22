package filesearch

// runRootError keeps the failing root attached to planner or executor errors.
// Incremental retries need that identity so the failed root can be escalated to
// a conservative root-level retry while untouched roots keep their queued paths.
type runRootError struct {
	RootID string
	Err    error
}

func (e *runRootError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *runRootError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// RunPlan holds one sealed indexing workload.
// Root-local progress was not enough because a single logical root can fan out
// into many bounded execution jobs, so this plan owns the immutable workload
// shape after planning finishes.
type RunPlan struct {
	PlanID         string
	RunID          string
	Kind           RunKind
	RootPlans      []RootPlan
	Jobs           []Job
	TotalWorkUnits int64
	// EstimatedTotals contains structural work estimates only. Full and
	// incremental runs stream their real file counts during execution, so UI
	// summaries must not treat these values as authoritative indexed counts.
	EstimatedTotals PlanTotals
}

// Seal returns a deep-copied workload snapshot so later planner mutations do
// not change the active run after sealing.
func (p RunPlan) Seal() RunPlan {
	sealed := p
	sealed.RootPlans = make([]RootPlan, len(p.RootPlans))
	for i := range p.RootPlans {
		sealed.RootPlans[i] = p.RootPlans[i].seal()
	}
	sealed.Jobs = make([]Job, len(p.Jobs))
	for i := range p.Jobs {
		sealed.Jobs[i] = p.Jobs[i].seal()
	}
	return sealed
}

// Run tracks one execution attempt against a sealed run plan.
// Root-local progress was not enough once one root could expand into many jobs,
// so run state owns the active stage and global progress indicators.
type Run struct {
	RunID                  string
	PlanID                 string
	Status                 RunStatus
	Stage                  RunStage
	CompletedWorkUnits     int64
	TotalWorkUnits         int64
	CompletedEntryCount    int64
	CompletedFileCount     int64
	ActiveJobID            string
	QueuedIncrementalCount int
	LastError              string
}

// RootPlan describes one configured root inside a sealed run plan.
// Root-local progress was not sufficient because one logical root can fan out
// into many execution jobs, so the planner freezes this structure before
// execution and keeps its traversal buffers immutable.
type RootPlan struct {
	RootID             string
	RootPath           string
	Strategy           RootPlanStrategy
	ScopeTree          *ScopeNode
	Totals             PlanTotals
	Jobs               []JobRef
	SplitPolicyVersion int
}

func (p RootPlan) seal() RootPlan {
	sealed := p
	if p.ScopeTree != nil {
		scope := p.ScopeTree.seal()
		sealed.ScopeTree = &scope
	}
	if p.Jobs != nil {
		sealed.Jobs = make([]JobRef, len(p.Jobs))
		copy(sealed.Jobs, p.Jobs)
	}
	return sealed
}

// ScopeNode captures one planned filesystem scope before it becomes a job.
// Root-local progress was not enough because the planner can split one root
// into multiple execution units, so the tree is sealed before execution.
type ScopeNode struct {
	ScopePath           string
	ScopeKind           ScopeKind
	ParentScopePath     string
	Children            []ScopeNode
	DirectoryCount      int64
	FileCount           int64
	IndexableEntryCount int64
	SkippedCount        int64
	PlannedScanUnits    int64
	PlannedWriteUnits   int64
	SplitRequired       bool
	Sealed              bool
}

func (n ScopeNode) seal() ScopeNode {
	sealed := n
	if n.Children != nil {
		sealed.Children = make([]ScopeNode, len(n.Children))
		for i := range n.Children {
			sealed.Children[i] = n.Children[i].seal()
		}
	}
	sealed.Sealed = true
	return sealed
}

// Job is one bounded execution unit inside a sealed run plan.
// Root-local progress was not enough because one root now becomes a sequence of
// jobs, and each job needs its own stable kind and progress budget.
type Job struct {
	JobID             string
	RootID            string
	RootPath          string
	ScopePath         string
	Kind              JobKind
	DirectDeltas      []PathDelta
	PlannedScanUnits  int64
	PlannedWriteUnits int64
	PlannedTotalUnits int64
	Status            JobStatus
	OrderIndex        int
}

func (j Job) seal() Job {
	sealed := j
	if j.DirectDeltas != nil {
		sealed.DirectDeltas = make([]PathDelta, len(j.DirectDeltas))
		copy(sealed.DirectDeltas, j.DirectDeltas)
	}
	return sealed
}

// PlanTotals freezes the counted work for a root or the whole run.
// The sealed totals are used so the denominator does not drift after planning.
type PlanTotals struct {
	DirectoryCount      int64
	FileCount           int64
	IndexableEntryCount int64
	SkippedCount        int64
	PlannedScanUnits    int64
	PlannedWriteUnits   int64
}

// JobRef points to a job in a root plan without duplicating the full payload.
type JobRef struct {
	JobID      string
	OrderIndex int
}

// RunKind identifies whether the sealed workload is full or incremental.
type RunKind string

const (
	RunKindFull        RunKind = "full"
	RunKindIncremental RunKind = "incremental"
)

// RunStatus tracks the life-cycle of one active or historical run.
type RunStatus string

const (
	RunStatusPlanning   RunStatus = "planning"
	RunStatusExecuting  RunStatus = "executing"
	RunStatusFinalizing RunStatus = "finalizing"
	RunStatusCompleted  RunStatus = "completed"
	RunStatusFailed     RunStatus = "failed"
	RunStatusCanceled   RunStatus = "canceled"
)

// RunStage is the active stage reported to the UI while a run is executing.
type RunStage string

const (
	RunStagePlanning   RunStage = "planning"
	RunStageExecuting  RunStage = "executing"
	RunStageFinalizing RunStage = "finalizing"
)

// RootPlanStrategy records how the planner split one configured root.
type RootPlanStrategy string

const (
	RootPlanStrategySingle    RootPlanStrategy = "single"
	RootPlanStrategySegmented RootPlanStrategy = "segmented"
)

// ScopeKind identifies whether a planned scope is recursive or only direct files.
type ScopeKind string

const (
	ScopeKindDirectFiles ScopeKind = "direct_files"
	ScopeKindSubtree     ScopeKind = "subtree"
)

// JobKind identifies the bounded work unit being executed.
type JobKind string

const (
	JobKindDirectFiles  JobKind = "direct_files"
	JobKindDirectDelta  JobKind = "direct_delta"
	JobKindSubtree      JobKind = "subtree"
	JobKindFinalizeRoot JobKind = "finalize_root"
)

// JobStatus tracks the state of a sealed job during execution.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusSkipped   JobStatus = "skipped"
)
