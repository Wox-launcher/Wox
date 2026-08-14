---
name: wox-test-ensurance
description: Use when the user invokes Wox test assurance or asks to run Wox tests; direct invocation is explicit permission to execute the current core, Go UI unit, and native smoke suites and fix failures until they pass.
---

# Wox Test Ensurance

## Overview

Drive Wox test assurance from the repository root until the core tests, headless Go UI tests, and native Go UI smoke tests pass. Treat failing output as evidence, fix the real cause first, and only change tests when the implementation is demonstrably correct and the test is stale, over-specified, or invalid.

## Invocation Contract

If this skill is explicitly invoked, the user has already authorized the complete Wox test assurance workflow: `make test`, `make test-go-ui-unit`, and `make smoke`. Do not stop after summarizing a plan or ask for extra confirmation.

If the user asks for one exact command, such as `make test`, run only that requested command and fix its failures. Do not silently expand an exact command request into native UI smoke testing.

Use the active Wox checkout containing the root `Makefile` and `wox.core` directory. Do not assume a fixed drive or checkout path.

## Preflight

Before running the suite, ensure no local Wox instance or VS Code-launched Wox debug process from the active checkout is still running. Force-stop matching processes; stale processes can hold ports or keep old state alive, and native smoke tests must own the Wox process they exercise.

Use a Windows PowerShell check like this from the repository root:

```powershell
$repo = (Resolve-Path ".").Path
$woxProcesses = Get-CimInstance Win32_Process | Where-Object {
    $name = $_.Name
    $cmd = $_.CommandLine
    $exe = $_.ExecutablePath
    $isWoxBinary = $name -in @("Wox.exe", "wox.exe", "wox.core.exe", "__debug_bin.exe")
    $isWoxDebug = $name -in @("dlv.exe", "__debug_bin.exe") -and $cmd -like "*$repo*"
    $isRepoRuntime = ($exe -like "$repo*") -and ($name -like "*wox*")
    $isWoxBinary -or $isWoxDebug -or $isRepoRuntime
}
$woxProcesses | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
```

If stopped processes are found, mention them briefly in the working update or final summary.

## Workflow

1. Confirm the current directory is the Wox repository root, then begin without asking whether to run tests.
2. Run the preflight process cleanup and force-stop any matching Wox/runtime debug process.
3. Run `make test` for the core integration suite.
4. Run `make test-go-ui-unit` for retained-widget, automation-contract, launcher, and driver tests. This suite does not open a native window.
5. Run `make smoke` for the real single-process Go UI. On headless Linux, set `GO_UI_SMOKE_RUNNER="xvfb-run -a"`.
6. For each failure, capture the package or smoke case, test name, command output, and relevant logs, then investigate and fix the root cause.
7. After any fix, format only the files touched according to Wox project style, then rerun the affected command.
8. Before claiming completion, rerun every command required by the invocation contract from the repository root. Smoke cases may be targeted while diagnosing, but explicit test assurance ends with the complete `make smoke` suite.

## Failure Triage

Prefer this order:

1. Check whether recent implementation changes broke product behavior, API contracts, async ordering, platform handling, or persisted state.
2. Inspect the failing test's intended behavior and compare it with the current product contract.
3. Fix production code when the test exposes a real regression.
4. Change an existing test only when evidence shows the product behavior is correct and the assertion/setup is outdated, flaky, or testing the wrong layer.
5. Do not add new tests unless the user explicitly asks for new test coverage.

Use targeted reruns only while narrowing a failure. They are not final proof. The final proof is every suite required by the invocation contract passing.

## Wox-Specific Notes

- Wox runs in mixed Windows and WSL environments. Verify the shell, path, and repo root before treating a runner failure as a code failure.
- If a shell shim or external tool prevents the real command from starting, isolate the environment problem, use the closest repo runner only for diagnosis, and do not report success until the requested `make` command passes.

## Completion Checklist

- Every command required by the invocation contract passed from the Wox repository root.
- Explicit test assurance includes successful `make test`, `make test-go-ui-unit`, and `make smoke` runs.
- Any code or test edits are locally formatted with the relevant project formatter.
- The final response includes the exact commands run and whether they passed.
