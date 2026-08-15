---
name: wox-visual-style
description: Design, implement, refactor, or review the visual style of Wox's portable Go UI under wox.core/ui. Use for Settings, dialogs, forms, tables, catalogs, management pages, shared Wox controls, typography, semantic colors, spacing, alignment, icons, control sizing, and default, hover, pressed, selected, focused, disabled, loading, empty, and error states. Also use to audit visual consistency or decide whether appearance belongs in launcher/component or launcher/view. Treat the Launcher query surface and Action Panel as explicit special contracts instead of forcing the ordinary Settings control-size system onto them.
---

# Wox Visual Style

Apply one coherent visual language to Wox Go UI without changing business behavior accidentally.

## Load the visual contract

Read [references/visual-style-guide.md](references/visual-style-guide.md) completely before designing, reviewing, or editing UI. Treat it as the sole visual policy for Wox Go UI. Treat production components and tests as implementation evidence; when they disagree with the guide, report or fix the implementation drift instead of silently redefining the guide from current code.

Also read the repository `AGENTS.md`, `README.md`, `Wox.code-workspace`, and `wox.core/ui/README.md` before refactors.

## Classify the surface

Classify the requested surface before applying size rules:

- Apply the ordinary control system to Settings, dialogs, forms, tables, catalogs, onboarding management controls, and other application-management pages.
- Keep the Launcher query, its accessories, results, toolbar, refinements, and Glance under launcher density and theme geometry.
- Keep the complete Action Panel, including its filter and rows, under its own geometry contract.
- Keep native window controls and platform-owned dialogs under the platform contract.

Do not use a special-surface exception to justify page-local styling elsewhere.

## Inspect the current contract

1. Inspect the nearest shared `Wox*` component and its focused tests.
2. Inspect `launcher/component/wox_theme.go`, `launcher/component/typography.go`, and `launcher/theme.go` when colors or type are involved.
3. Inspect relevant views for repeated literals, primitive interactive controls, and inconsistent state handling.
4. Check whether the same semantic control exists on other production pages.
5. Record current values separately from target values when auditing known drift.

Useful focused searches include:

```bash
rg -n 'Wox(Button|TextField|Dropdown|Checkbox|Switch|IconButton)' wox.core/ui/launcher
rg -n 'woxwidget\.Gesture\{' wox.core/ui/launcher/view
rg -n 'TextStyle\{Size:|Height:|Radius:|Color: woxui\.Color' wox.core/ui/launcher/view
```

Interpret results in context. Primitive gestures are valid for page-specific pointer regions and tooltips; they are a defect when they recreate a reusable control contract.

## Design or review the change

1. Identify each element's semantic role, size tier, visual state, and alignment group.
2. Use semantic theme roles and shared typography instead of raw colors or page-local type scales.
3. Use the ordinary 32-unit control height only inside its defined scope. Preserve special Launcher and Action Panel geometry.
4. Align controls by interaction frame and centerline. Preserve the internal geometry of checkboxes and switches.
5. Cover every applicable state from the state matrix, including non-color cues and accessibility semantics.
6. Prefer an existing categorized SVG from `wox.core/common/icons.go`; add a reusable icon there before introducing a local asset.
7. Check light and dark themes, long or translated text, narrow widths, scrolling, logical units, DPI boundaries, and disabled/error content.

Express alignment through layout primitives first: use `Align`, Flex main/cross-axis alignment, `Expanded`, or `Constrained` to center and place controls. Do not hand-calculate offsets such as `(rowHeight-controlHeight)/2`, or use padding as a substitute for alignment; improve the owning layout/component boundary when the existing primitives cannot express the relationship.

When reviewing only, report concrete violations with the owning layer and recommended shared fix. Do not mutate files unless the user requested implementation.

## Implement at the correct layer

- Put reusable geometry, appearance, pointer states, focus treatment, and accessibility in `launcher/component`.
- Keep page hierarchy, responsive composition, and genuinely page-specific spacing in `launcher/view`.
- Keep mutable interaction state below `launcher.App` and preserve `widget.Boundary.Build` purity.
- Centralize stable metrics used by multiple production surfaces. Do not add a token for a single local constraint.
- Keep ordinary control size explicit through shared defaults or a semantic size API; do not copy the same height into call sites.
- Keep Launcher and Action Panel metrics independent so ordinary component changes cannot alter their geometry accidentally.
- Prefer layout components over manual alignment math. Center controls with `Align` or Flex alignment and reserve padding for actual content insets, not positional compensation.
- Add English intent comments only for non-obvious visual constraints or exceptions.

Preserve existing behavior unless the request includes interaction changes. A visual refactor must not alter values, callbacks, focus order, IME ownership, keyboard activation, scrolling, or window lifecycle.

## Validate proportionally

1. Format changed Go files with `gofmt`.
2. Run the narrowest affected component and view tests from `wox.core`.
3. Run `GOCACHE=/tmp/wox-go-cache make test-go-ui-unit` from the repository root when shared components, widget behavior, or broad launcher UI changed.
4. Add focused source tests for size tiers, alignment, states, semantics, and special-surface isolation.
5. Review the diff for duplicated literals and accidental Launcher or Action Panel changes.

Source inspection is the default. Request permission before launching Wox for runtime comparison unless the user already requested it. Escalate to serial screenshots when font measurement, DPI, animation, clipping, compositing, native focus, or platform rendering leaves a meaningful visual ambiguity.

## Report completion

Report:

- surface classification and size tier;
- shared component or view ownership;
- visual states covered;
- known implementation drift found or fixed;
- formatting and tests actually run;
- whether validation was source-only or included an approved runtime comparison.
