---
name: wox-generate-smoke-test-case
description: Analyze Wox feature code and plugin-exposed settings, recommend high-value native UI smoke scenarios and user flows, then generate focused Go UI smoke test cases using the automation driver and shared-process harness. Use when the user asks to identify smoke coverage for a Wox feature or plugin, or to design, add, write, or generate a smoke case under wox.core/test/smoke.
---

# Wox Generate Smoke Test Case

Understand the feature before choosing coverage. Recommend the smallest high-value real-process smoke flow, then reuse the current smoke harness and product automation surface instead of building test-only paths.

## Discover the Feature

1. Read the repository `AGENTS.md` and relevant `README.md` files. Inspect `git status`, unstaged changes, staged changes, and relevant untracked files without modifying unrelated work.
2. Locate the feature's real user entry point, then trace the code end to end: UI component, interaction handler, controller or service, persisted state or side effect, visible result, error path, and cleanup. Search every caller of the shared function before deciding where coverage belongs.
3. Inspect the smoke infrastructure and nearest existing coverage:
   - `wox.core/test/smoke/harness.go`
   - `wox.core/test/automationdriver/client.go`
   - The closest existing case under `wox.core/test/smoke/`
   - The UI code that owns the relevant automation IDs and actions
4. Identify the main functional points, platform branches, important state transitions, failure recovery, and existing unit or smoke coverage. Base the list on concrete code paths, not UI labels or assumptions.

### Inspect Plugin Settings

When the target is a plugin:

1. Find every exposed setting in the plugin metadata `SettingDefinitions`, dynamic setting callbacks, and query/action forms. Record its key, type, default, validators, options, and visibility conditions.
2. Search every read of each setting, including `GetSetting` calls, initialization, reload callbacks, and persistence code. Map the setting to the behavior it actually changes.
3. Build a setting coverage matrix with: setting, affected user behavior, important values or boundaries, existing coverage, smoke value, automation feasibility, and recommendation.
4. Recommend a smoke case when a setting changes query results, actions, enablement, persistence or reload behavior, platform integration, validation or recovery, or another user-visible cross-process boundary.
5. Skip separate smoke coverage for headings, labels, layout-only fields, equivalent options already covered by one representative value, and branches better protected by unit tests. Avoid a case for every setting or every option.
6. Group settings into one user flow only when they naturally belong to the same task and failures remain easy to diagnose. Keep independent high-risk behaviors in separate cases.
7. Never require real secrets, uncontrolled network services, or unavailable hardware. Prefer deterministic missing/invalid configuration and local persistence behavior unless the dependency is explicitly controlled by the test environment.

## Recommend Coverage

When the user asks for coverage design, present:

- **Code map:** Name the main source files, functions, and responsibilities.
- **Functional points:** Summarize the primary user-visible behaviors and important boundaries.
- **Candidate cases:** Recommend no more than five flows, ordered by value. For each, include priority, user risk, existing coverage gap, prerequisites, actions, observable expected result, and automation feasibility.
- **Plugin setting matrix:** For plugin requests, show which exposed settings deserve smoke coverage, which do not, and the source-backed reason for each decision.
- **Recommended first case:** Select the smallest flow that covers the highest-risk real integration boundary and explain briefly why it belongs in smoke rather than a unit test.
- **Automation gap:** Identify any missing stable automation ID or action required by the recommended flow.

When the user already specified the exact flow, keep the pre-edit design brief: present only the code map, one observable behavior contract, and any automation gap. Do not expand an approved flow into an unnecessary candidate list or full setting matrix.

Recommend smoke coverage only for real-process native UI, lifecycle, persistence, plugin integration, platform, accessibility, or rendering boundaries. Prefer unit tests for local pure logic and skip behavior already covered at the same boundary.

If the user asked only for design, stop after the recommendation. If the user already named or approved an exact flow, continue. Otherwise ask which recommended case to generate before editing product or test code.

## Generate the Case

1. State one observable behavior contract for the selected flow.
2. Search all smoke packages for an existing helper before adding a local one. Promote a helper to `automationdriver` or `smoke` when a second consumer uses the same interaction contract; keep plugin-specific behavior in the leaf package. Avoid page-object layers or table-editor DSLs until repeated cases justify them.
3. Reuse existing automation IDs, actions, `smoke.Case`, and helpers. Before driving a control, confirm its semantics expose a stable ID, role, label, current value or checked state, selected state when relevant, and the correct action. If that contract is missing, modify the smallest shared owning UI component to add a stable automation ID or semantic action, then add a focused component test for that contract. The semantic action must follow the real user interaction path; do not add a test-only bypass around product behavior.
4. Add the case under a functional path such as `wox.core/test/smoke/launcher/query/plugin/calculator/`. Use the next unused three-digit number without renumbering existing cases:
   - File: `NNN_descriptive_name_test.go`
   - Function: `TestNNNDescriptiveName`
   - Build constraint: `//go:build wox_ui_smoke`
5. Immediately above every generated `TestNNN...` function, add a concise English doc comment that states the user-visible intent, the ordered UI flow, and the final evidence that proves the behavior. Mention prerequisites or cleanup only when they are part of the contract. Describe product behavior rather than automation implementation details. Use this shape:

   ```go
   // TestNNNDescriptiveName verifies <user-visible behavior and boundary>.
   // Flow: <entry> -> <important actions or state transitions> -> <observable result>.
   // Evidence: <real UI, runtime, persisted artifact, or log assertion that proves success>.
   func TestNNNDescriptiveName(t *testing.T) {
   ```

6. Drive the UI through `automationdriver.Client`. Wait for stable semantic state with `client.WaitFor`; never use fixed sleeps to guess readiness. Use explicit semantic actions and postconditions such as `Selected`, `Checked`, `Value`, status, or disappearance. Do not use node bounds, pointer coordinates, or screenshot pixels when the owning UI can expose a semantic route. Coordinate interaction is allowed only for an unavoidable native or platform surface that cannot expose semantics; document why, resolve coordinates from a current semantic node instead of hard-coding them, and assert the resulting functional state rather than geometry.
7. Treat query and result state as generation-bound. Setting the same query value may be a no-op, and refreshes may replace dynamic result IDs. Force a real query transition when freshness matters, wait for both the input value and `launcher.results=complete`, and resolve dynamic IDs again after refresh instead of retaining them across generations.
8. Let `smoke.Case` own before/after reset and the shared client, but do not assume reset restores plugin settings or desktop side effects. Explicitly restore changed settings and clean external state. Reopen settings or inspect persisted data before exercising runtime behavior; a locally updated control does not prove an asynchronous save completed.
9. Keep assertions user-visible and deterministic. Prefer evidence in this order: real runtime artifact or fresh log slice, reloaded persisted value, then live semantic state. Treat relevant snapshot diagnostics as failures. Do not weaken assertions or add retries merely to hide a race.
10. Do not launch another Wox process or create a second data directory inside a case. Use `automationdriver.SharedDataDirectoryEnvironment` only when the behavior must inspect real persisted output, and poll artifacts independently of UI generation changes.
11. Format every touched Go file with the repository formatter. Run the new case from the repository root:

   ```text
   make smoke <functional/path/NNN>
   ```

12. After the targeted case passes, run `make smoke` to catch leaked state and shared-process cleanup failures. If the full suite fails in an existing case, distinguish that baseline failure from the new selector instead of attributing it to the new coverage. If execution is blocked by the environment, report the exact blocker and do not claim runtime coverage.

## Failure Handling

- Start from the exact command output, logs, snapshot, and failing state.
- On a wait failure, capture the current values, checked states, status nodes, diagnostics, and relevant fresh log or artifact contents. Report the observed boundary instead of only the timeout.
- Fix the shared product or automation boundary when that is the root cause; do not patch every case around it.
- Add diagnostic logging only when the failure path cannot be identified with certainty, then use the new evidence before changing behavior.
- Preserve unrelated worktree changes and keep the diff limited to the case plus any strictly required automation exposure.

## Completion

Report the behavior covered, the created case selector, every production or automation file changed to support it, and the exact verification commands with pass or failure status.
