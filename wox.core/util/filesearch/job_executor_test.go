package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestJobExecutorOrderIsStable(t *testing.T) {
	rootOnePath := filepath.Join(t.TempDir(), "root-one")
	rootTwoPath := filepath.Join(t.TempDir(), "root-two")
	mustWriteTestFile(t, filepath.Join(rootOnePath, "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(rootOnePath, "beta.txt"), "beta")
	mustWriteTestFile(t, filepath.Join(rootOnePath, "gamma.txt"), "gamma")
	mustWriteTestFile(t, filepath.Join(rootTwoPath, "nested", "gamma.txt"), "gamma")

	rootOne := testRunPlannerRootRecord("root-one", rootOnePath)
	rootTwo := testRunPlannerRootRecord("root-two", rootTwoPath)
	plan := RunPlan{
		PlanID: "plan-order",
		RunID:  "run-order",
		Kind:   RunKindFull,
		RootPlans: []RootPlan{{
			RootID:   rootOne.ID,
			RootPath: rootOne.Path,
		}, {
			RootID:   rootTwo.ID,
			RootPath: rootTwo.Path,
		}},
		Jobs: []Job{{
			JobID:             "job-subtree",
			RootID:            rootTwo.ID,
			RootPath:          rootTwo.Path,
			ScopePath:         rootTwo.Path,
			Kind:              JobKindSubtree,
			PlannedScanUnits:  2,
			PlannedWriteUnits: 2,
			PlannedTotalUnits: 4,
			Status:            JobStatusPending,
			OrderIndex:        11,
		}, {
			JobID:             "job-direct-files",
			RootID:            rootOne.ID,
			RootPath:          rootOne.Path,
			ScopePath:         rootOne.Path,
			Kind:              JobKindDirectFiles,
			PlannedScanUnits:  4,
			PlannedWriteUnits: 4,
			PlannedTotalUnits: 8,
			Status:            JobStatusPending,
			OrderIndex:        3,
		}, {
			JobID:             "job-finalize",
			RootID:            rootTwo.ID,
			RootPath:          rootTwo.Path,
			ScopePath:         rootTwo.Path,
			Kind:              JobKindFinalizeRoot,
			PlannedWriteUnits: 1,
			PlannedTotalUnits: 1,
			Status:            JobStatusPending,
			OrderIndex:        19,
		}},
		TotalWorkUnits: 13,
	}

	builder := NewSnapshotBuilder(newPolicyState(Policy{}))
	directFilesBatch, err := builder.BuildDirectFilesJobSnapshot(context.Background(), rootOne, plan.Jobs[1])
	if err != nil {
		t.Fatalf("build direct-files snapshot: %v", err)
	}

	if got, want := snapshotEntryPaths(directFilesBatch), []string{
		rootOnePath,
		filepath.Join(rootOnePath, "alpha.txt"),
		filepath.Join(rootOnePath, "beta.txt"),
		filepath.Join(rootOnePath, "gamma.txt"),
	}; !equalPaths(got, want) {
		t.Fatalf("unexpected direct-files entry paths: got %v want %v", got, want)
	}
	if got, want := snapshotDirectoryPaths(directFilesBatch), []string{rootOnePath}; !equalPaths(got, want) {
		t.Fatalf("unexpected direct-files directory paths: got %v want %v", got, want)
	}

	executor := NewJobExecutor(builder)
	completedOrder := make([]int, 0, len(plan.Jobs))
	snapshots := make([]StatusSnapshot, 0, len(plan.Jobs)*2+1)
	run, executedJobs, err := executor.ExecuteRun(context.Background(), plan, []RootRecord{rootOne, rootTwo}, func(snapshot StatusSnapshot, job Job) {
		snapshots = append(snapshots, snapshot)
		if job.Status == JobStatusCompleted {
			completedOrder = append(completedOrder, job.OrderIndex)
		}
	})
	if err != nil {
		t.Fatalf("execute run: %v", err)
	}

	if got, want := run.Status, RunStatusCompleted; got != want {
		t.Fatalf("unexpected run status: got %s want %s", got, want)
	}

	gotOrder := completedOrder
	wantOrder := []int{11, 3, 19}
	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("unexpected completed job count: got %d want %d", len(gotOrder), len(wantOrder))
	}
	for index := range wantOrder {
		if gotOrder[index] != wantOrder[index] {
			t.Fatalf("unexpected completed order at %d: got %v want %v", index, gotOrder, wantOrder)
		}
		if executedJobs[index].OrderIndex != wantOrder[index] {
			t.Fatalf("unexpected stored order index at %d: got %d want %d", index, executedJobs[index].OrderIndex, wantOrder[index])
		}
		if executedJobs[index].Status != JobStatusCompleted {
			t.Fatalf("expected executed job %q to be completed, got %s", executedJobs[index].JobID, executedJobs[index].Status)
		}
	}

	sawFinalizing := false
	for _, snapshot := range snapshots {
		if snapshot.ActiveJobKind == JobKindFinalizeRoot && snapshot.ActiveRunStatus == RunStatusFinalizing && snapshot.ActiveStage == RunStageFinalizing {
			sawFinalizing = true
			break
		}
	}
	if !sawFinalizing {
		t.Fatal("expected finalize job snapshots to expose finalizing run state")
	}
	if len(snapshots) == 0 {
		t.Fatal("expected snapshots")
	}
	lastSnapshot := snapshots[len(snapshots)-1]
	if got, want := lastSnapshot.ActiveRunStatus, RunStatusCompleted; got != want {
		t.Fatalf("expected terminal completed snapshot, got %s want %s", got, want)
	}
}

func TestJobExecutorProgressNeverDecreasesAcrossRoots(t *testing.T) {
	rootOnePath := filepath.Join(t.TempDir(), "root-one")
	rootTwoPath := filepath.Join(t.TempDir(), "root-two")
	mustWriteTestFile(t, filepath.Join(rootOnePath, "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(rootTwoPath, "beta.txt"), "beta")

	rootOne := testRunPlannerRootRecord("root-one", rootOnePath)
	rootTwo := testRunPlannerRootRecord("root-two", rootTwoPath)
	plan := RunPlan{
		PlanID: "plan-progress",
		RunID:  "run-progress",
		Kind:   RunKindFull,
		RootPlans: []RootPlan{{
			RootID:   rootOne.ID,
			RootPath: rootOne.Path,
		}, {
			RootID:   rootTwo.ID,
			RootPath: rootTwo.Path,
		}},
		Jobs: []Job{
			testFinalizeJob(rootOne, 0),
			testFinalizeJob(rootOne, 1),
			testFinalizeJob(rootTwo, 2),
			testFinalizeJob(rootTwo, 3),
		},
		TotalWorkUnits: 4,
	}

	executor := NewJobExecutor(nil)
	progresses := make([]int64, 0, 8)
	_, _, err := executor.ExecuteRun(context.Background(), plan, []RootRecord{rootOne, rootTwo}, func(snapshot StatusSnapshot, _ Job) {
		progresses = append(progresses, snapshot.ProgressCurrent)
	})
	if err != nil {
		t.Fatalf("execute run: %v", err)
	}

	if len(progresses) == 0 {
		t.Fatal("expected progress snapshots")
	}
	for index := 1; index < len(progresses); index++ {
		if progresses[index] < progresses[index-1] {
			t.Fatalf("global progress decreased at snapshot %d: got %d after %d", index, progresses[index], progresses[index-1])
		}
	}
	for _, snapshot := range progressesForCompatibility(t, plan, []RootRecord{rootOne, rootTwo}) {
		if snapshot.ActiveRunStatus == RunStatusCompleted {
			continue
		}
		if got, want := snapshot.ActiveProgressTotal, int64(1); got != want {
			t.Fatalf("expected active progress to stay root-scoped for finalize jobs: got %d want %d", got, want)
		}
		if snapshot.ActiveProgressCurrent > snapshot.ActiveProgressTotal {
			t.Fatalf("active progress overflowed its scoped total: got %d/%d", snapshot.ActiveProgressCurrent, snapshot.ActiveProgressTotal)
		}
		if snapshot.RunProgressTotal != plan.TotalWorkUnits {
			t.Fatalf("unexpected run progress total: got %d want %d", snapshot.RunProgressTotal, plan.TotalWorkUnits)
		}
	}
}

func TestJobExecutorNinetyNinePercentMeansSmallKnownRemainder(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root")
	mustWriteTestFile(t, filepath.Join(rootPath, "alpha.txt"), "alpha")
	root := testRunPlannerRootRecord("root", rootPath)

	jobs := make([]Job, 0, 200)
	for index := 0; index < 200; index++ {
		jobs = append(jobs, testFinalizeJob(root, index))
	}

	plan := RunPlan{
		PlanID: "plan-ninety-nine",
		RunID:  "run-ninety-nine",
		Kind:   RunKindFull,
		RootPlans: []RootPlan{{
			RootID:   root.ID,
			RootPath: root.Path,
		}},
		Jobs:           jobs,
		TotalWorkUnits: int64(len(jobs)),
	}

	executor := NewJobExecutor(nil)
	sawNinetyNine := false
	_, _, err := executor.ExecuteRun(context.Background(), plan, []RootRecord{root}, func(snapshot StatusSnapshot, _ Job) {
		if snapshot.ProgressTotal == 0 {
			return
		}
		percent := (snapshot.ProgressCurrent * 100) / snapshot.ProgressTotal
		if percent != 99 {
			return
		}

		sawNinetyNine = true
		remaining := snapshot.RunProgressTotal - snapshot.RunProgressCurrent
		if remaining*100 > snapshot.RunProgressTotal {
			t.Fatalf("99%% reported too early: remaining=%d total=%d current=%d/%d", remaining, snapshot.RunProgressTotal, snapshot.ProgressCurrent, snapshot.ProgressTotal)
		}
	})
	if err != nil {
		t.Fatalf("execute run: %v", err)
	}
	if !sawNinetyNine {
		t.Fatal("expected executor to report 99% before completion")
	}
}

func TestJobExecutorFinalizeRootAdvancesCursorAfterPriorJobs(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-finalize")
	nestedPath := filepath.Join(rootPath, "nested")
	mustWriteTestFile(t, filepath.Join(rootPath, "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(nestedPath, "beta.txt"), "beta")

	root := testRunPlannerRootRecord("root-finalize", rootPath)
	directFilesJob := Job{
		JobID:             "job-direct-files",
		RootID:            root.ID,
		RootPath:          root.Path,
		ScopePath:         root.Path,
		Kind:              JobKindDirectFiles,
		PlannedScanUnits:  1,
		PlannedWriteUnits: 1,
		PlannedTotalUnits: 2,
		Status:            JobStatusPending,
		OrderIndex:        0,
	}
	subtreeJob := Job{
		JobID:             "job-subtree",
		RootID:            root.ID,
		RootPath:          root.Path,
		ScopePath:         nestedPath,
		Kind:              JobKindSubtree,
		PlannedScanUnits:  1,
		PlannedWriteUnits: 1,
		PlannedTotalUnits: 2,
		Status:            JobStatusPending,
		OrderIndex:        1,
	}
	finalizeJob := testFinalizeJob(root, 2)
	plan := RunPlan{
		PlanID: "plan-finalize-order",
		RunID:  "run-finalize-order",
		Kind:   RunKindFull,
		RootPlans: []RootPlan{{
			RootID:   root.ID,
			RootPath: root.Path,
		}},
		Jobs:           []Job{directFilesJob, subtreeJob, finalizeJob},
		TotalWorkUnits: directFilesJob.PlannedTotalUnits + subtreeJob.PlannedTotalUnits + finalizeJob.PlannedTotalUnits,
	}

	executor := NewJobExecutor(NewSnapshotBuilder(newPolicyState(Policy{})))
	completedKinds := make([]JobKind, 0, len(plan.Jobs))
	finalizeStartProgress := int64(-1)
	_, _, err := executor.ExecuteRun(context.Background(), plan, []RootRecord{root}, func(snapshot StatusSnapshot, job Job) {
		if job.Status == JobStatusCompleted {
			completedKinds = append(completedKinds, job.Kind)
		}
		if snapshot.ActiveJobKind == JobKindFinalizeRoot && snapshot.ActiveRunStatus == RunStatusFinalizing && job.Status == JobStatusRunning {
			finalizeStartProgress = snapshot.RunProgressCurrent
		}
	})
	if err != nil {
		t.Fatalf("execute run: %v", err)
	}

	if finalizeStartProgress != directFilesJob.PlannedTotalUnits+subtreeJob.PlannedTotalUnits {
		t.Fatalf(
			"expected finalize job to start after prior jobs completed, got progress %d want %d",
			finalizeStartProgress,
			directFilesJob.PlannedTotalUnits+subtreeJob.PlannedTotalUnits,
		)
	}
	if got, want := completedKinds, []JobKind{JobKindDirectFiles, JobKindSubtree, JobKindFinalizeRoot}; len(got) != len(want) {
		t.Fatalf("unexpected completed job kinds: got %v want %v", got, want)
	} else {
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("unexpected completed job kinds: got %v want %v", got, want)
			}
		}
	}
}

func TestJobExecutorBatchesSmallFullRunSubtreeApplies(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-batch")
	scopeA := filepath.Join(rootPath, "scope-a")
	scopeB := filepath.Join(rootPath, "scope-b")
	mustWriteTestFile(t, filepath.Join(scopeA, "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(scopeB, "beta.txt"), "beta")

	root := testRunPlannerRootRecord("root-batch", rootPath)
	jobA := Job{
		JobID:             "job-subtree-a",
		RootID:            root.ID,
		RootPath:          root.Path,
		ScopePath:         scopeA,
		Kind:              JobKindSubtree,
		PlannedScanUnits:  1,
		PlannedWriteUnits: 1,
		PlannedTotalUnits: 2,
		Status:            JobStatusPending,
		OrderIndex:        0,
	}
	jobB := Job{
		JobID:             "job-subtree-b",
		RootID:            root.ID,
		RootPath:          root.Path,
		ScopePath:         scopeB,
		Kind:              JobKindSubtree,
		PlannedScanUnits:  1,
		PlannedWriteUnits: 1,
		PlannedTotalUnits: 2,
		Status:            JobStatusPending,
		OrderIndex:        1,
	}
	finalizeJob := testFinalizeJob(root, 2)
	plan := RunPlan{
		PlanID: "plan-batch-full",
		RunID:  "run-batch-full",
		Kind:   RunKindFull,
		RootPlans: []RootPlan{{
			RootID:   root.ID,
			RootPath: root.Path,
		}},
		Jobs:           []Job{jobA, jobB, finalizeJob},
		TotalWorkUnits: jobA.PlannedTotalUnits + jobB.PlannedTotalUnits + finalizeJob.PlannedTotalUnits,
	}

	executor := NewJobExecutor(NewSnapshotBuilder(newPolicyState(Policy{})))
	executor.SetSubtreeBatchConfig(subtreeApplyBatchConfig{
		MaxJobCount:        4,
		MaxJobTotalUnits:   4,
		MaxBatchTotalUnits: 8,
	})

	singleScopes := []string{}
	batchScopes := [][]string{}
	executor.SetApplyFunc(func(_ context.Context, _ RootRecord, job Job, _ *SubtreeSnapshotBatch) error {
		if job.Kind == JobKindSubtree {
			singleScopes = append(singleScopes, job.ScopePath)
		}
		return nil
	})
	executor.SetSubtreeBatchApplyFunc(func(_ context.Context, _ RootRecord, batches []SubtreeSnapshotBatch) error {
		scopes := make([]string, 0, len(batches))
		for _, batch := range batches {
			scopes = append(scopes, batch.ScopePath)
		}
		batchScopes = append(batchScopes, scopes)
		return nil
	})

	finalizeStartProgress := int64(-1)
	_, executedJobs, err := executor.ExecuteRun(context.Background(), plan, []RootRecord{root}, func(snapshot StatusSnapshot, job Job) {
		if snapshot.ActiveJobKind == JobKindFinalizeRoot && snapshot.ActiveRunStatus == RunStatusFinalizing && job.Status == JobStatusRunning {
			finalizeStartProgress = snapshot.RunProgressCurrent
		}
	})
	if err != nil {
		t.Fatalf("execute run with batched subtree apply: %v", err)
	}

	if len(singleScopes) != 0 {
		t.Fatalf("expected full-run subtree jobs to avoid per-job apply, got %v", singleScopes)
	}
	if got, want := len(batchScopes), 1; got != want {
		t.Fatalf("expected one batched subtree apply, got %d want %d", got, want)
	}
	if got, want := batchScopes[0], []string{scopeA, scopeB}; !equalPaths(got, want) {
		t.Fatalf("unexpected batched subtree scopes: got %v want %v", got, want)
	}
	if finalizeStartProgress != jobA.PlannedTotalUnits+jobB.PlannedTotalUnits {
		t.Fatalf(
			"expected finalize to start after batched subtree jobs completed, got progress %d want %d",
			finalizeStartProgress,
			jobA.PlannedTotalUnits+jobB.PlannedTotalUnits,
		)
	}
	for _, job := range executedJobs {
		if job.Status != JobStatusCompleted {
			t.Fatalf("expected executed job %q to be completed, got %s", job.JobID, job.Status)
		}
	}
}

func TestJobExecutorIncrementalRunKeepsSubtreeAppliesPerJob(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-incremental")
	scopeA := filepath.Join(rootPath, "scope-a")
	scopeB := filepath.Join(rootPath, "scope-b")
	mustWriteTestFile(t, filepath.Join(scopeA, "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(scopeB, "beta.txt"), "beta")

	root := testRunPlannerRootRecord("root-incremental", rootPath)
	jobA := Job{
		JobID:             "job-subtree-a",
		RootID:            root.ID,
		RootPath:          root.Path,
		ScopePath:         scopeA,
		Kind:              JobKindSubtree,
		PlannedScanUnits:  1,
		PlannedWriteUnits: 1,
		PlannedTotalUnits: 2,
		Status:            JobStatusPending,
		OrderIndex:        0,
	}
	jobB := Job{
		JobID:             "job-subtree-b",
		RootID:            root.ID,
		RootPath:          root.Path,
		ScopePath:         scopeB,
		Kind:              JobKindSubtree,
		PlannedScanUnits:  1,
		PlannedWriteUnits: 1,
		PlannedTotalUnits: 2,
		Status:            JobStatusPending,
		OrderIndex:        1,
	}
	plan := RunPlan{
		PlanID: "plan-batch-incremental",
		RunID:  "run-batch-incremental",
		Kind:   RunKindIncremental,
		RootPlans: []RootPlan{{
			RootID:   root.ID,
			RootPath: root.Path,
		}},
		Jobs:           []Job{jobA, jobB},
		TotalWorkUnits: jobA.PlannedTotalUnits + jobB.PlannedTotalUnits,
	}

	executor := NewJobExecutor(NewSnapshotBuilder(newPolicyState(Policy{})))
	executor.SetSubtreeBatchConfig(subtreeApplyBatchConfig{
		MaxJobCount:        4,
		MaxJobTotalUnits:   4,
		MaxBatchTotalUnits: 8,
	})

	singleScopes := []string{}
	batchCalls := 0
	executor.SetApplyFunc(func(_ context.Context, _ RootRecord, job Job, _ *SubtreeSnapshotBatch) error {
		if job.Kind == JobKindSubtree {
			singleScopes = append(singleScopes, job.ScopePath)
		}
		return nil
	})
	executor.SetSubtreeBatchApplyFunc(func(_ context.Context, _ RootRecord, _ []SubtreeSnapshotBatch) error {
		batchCalls++
		return nil
	})

	if _, _, err := executor.ExecuteRun(context.Background(), plan, []RootRecord{root}, nil); err != nil {
		t.Fatalf("execute incremental run: %v", err)
	}

	if batchCalls != 0 {
		t.Fatalf("expected incremental subtree apply to stay per-job, got %d batched calls", batchCalls)
	}
	if got, want := singleScopes, []string{scopeA, scopeB}; !equalPaths(got, want) {
		t.Fatalf("unexpected per-job subtree scopes: got %v want %v", got, want)
	}
}

func testFinalizeJob(root RootRecord, orderIndex int) Job {
	return Job{
		JobID:             fmt.Sprintf("%s-job-%03d", root.ID, orderIndex),
		RootID:            root.ID,
		RootPath:          root.Path,
		ScopePath:         root.Path,
		Kind:              JobKindFinalizeRoot,
		PlannedWriteUnits: 1,
		PlannedTotalUnits: 1,
		Status:            JobStatusPending,
		OrderIndex:        orderIndex,
	}
}

func snapshotEntryPaths(batch SubtreeSnapshotBatch) []string {
	paths := make([]string, 0, len(batch.Entries))
	for _, entry := range batch.Entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

func snapshotDirectoryPaths(batch SubtreeSnapshotBatch) []string {
	paths := make([]string, 0, len(batch.Directories))
	for _, directory := range batch.Directories {
		paths = append(paths, directory.Path)
	}
	return paths
}

func equalPaths(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func progressesForCompatibility(t *testing.T, plan RunPlan, roots []RootRecord) []StatusSnapshot {
	t.Helper()

	snapshots := make([]StatusSnapshot, 0, len(plan.Jobs)*2+1)
	_, _, err := NewJobExecutor(nil).ExecuteRun(context.Background(), plan, roots, func(snapshot StatusSnapshot, _ Job) {
		snapshots = append(snapshots, snapshot)
	})
	if err != nil {
		t.Fatalf("execute compatibility run: %v", err)
	}
	return snapshots
}
