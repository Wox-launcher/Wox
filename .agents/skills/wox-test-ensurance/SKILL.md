---
name: wox-test-ensurance
description: Use when the user invokes Wox test assurance or asks to run Wox `make test` and `make smoke`; direct invocation is explicit permission to execute both suites and fix failures until they pass.
---

# Wox Test Ensurance

## Overview

Drive Wox test assurance from the repository root until both `make test` and `make smoke` pass. Treat failing output as evidence, fix the real cause first, and only change tests when the implementation is demonstrably correct and the test is stale, over-specified, or invalid.

## Invocation Contract

If this skill is explicitly invoked, the user has already asked to run the required test and smoke commands. Do not stop after summarizing a plan or ask for extra confirmation based on AGENTS.md test restrictions; the skill invocation is the explicit request that satisfies that rule.

Start execution immediately from `C:\dev\Wox` unless the user provides another Wox checkout path.

## Preflight

Before running any test command, ensure no local Wox instance or VS Code-launched Wox debug process is still running. Force-stop matching processes before `make test`; stale core/UI processes can hold ports, keep old state alive, or block smoke launch/build output.

Use a Windows PowerShell check like this from the Wox checkout, adjusting `$repo` only when the user provides another path:

```powershell
$repo = (Resolve-Path "C:\dev\Wox").Path
$woxProcesses = Get-CimInstance Win32_Process | Where-Object {
    $name = $_.Name
    $cmd = $_.CommandLine
    $exe = $_.ExecutablePath
    $isWoxBinary = $name -in @("Wox.exe", "wox.exe", "wox-ui.exe", "wox.core.exe", "__debug_bin.exe")
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
3. Run `make test` first and capture the failing package, test name, command output, and relevant logs.
4. If `make test` fails, investigate and fix it before starting smoke verification.
5. Run `make smoke` after `make test` is green.
6. If `make smoke` fails, investigate from the real failing smoke path and logs.
7. After any fix, format only the files touched according to Wox project style, then rerun the affected command.
8. Before claiming completion, rerun the full `make test` and full `make smoke` from the repository root in that order.

## Failure Triage

Prefer this order:

1. Check whether recent implementation changes broke product behavior, API contracts, async ordering, platform handling, or persisted state.
2. Inspect the failing test's intended behavior and compare it with the current product contract.
3. Fix production code when the test exposes a real regression.
4. Change an existing test only when evidence shows the product behavior is correct and the assertion/setup is outdated, flaky, or testing the wrong layer.
5. Do not add new unit tests or smoke tests unless the user explicitly asks for new test coverage.

Use targeted reruns only while narrowing a failure. They are not final proof. The final proof is the complete `make test` and `make smoke` sequence passing.

## Wox-Specific Notes

- Keep fixes narrow. If a smoke failure is in smoke orchestration or helpers, prefer a minimal test/harness fix unless logs point into product behavior.
- Wox runs in mixed Windows and WSL environments. Verify the shell, path, and repo root before treating a runner failure as a code failure.
- If `make smoke` hangs near build or launch, check whether a stale `wox-ui.exe` from the repo build output is still running before changing tests.
- If a shell shim or external tool prevents the real command from starting, isolate the environment problem, use the closest repo runner only for diagnosis, and do not report success until the requested `make` command passes.
- For Dart edits, format with the workspace line length: `dart format --line-length 180 <files>`.

## Completion Checklist

- `make test` passed from the Wox repository root.
- `make smoke` passed from the Wox repository root.
- Any code or test edits are locally formatted with the relevant project formatter.
- The final response includes the exact commands run and whether they passed.
