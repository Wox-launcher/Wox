# WoxView JSON protocol

WoxView renders a versioned JSON tree with Wox widgets. It is independent of a
particular plugin language or React. Plugin windows accept the serialized tree as
`WindowContent{Type: "woxview", Data: json}`. `Content.Update` replaces the tree
without replacing the document; `Content.Set` replaces the document and its state.
The window must be focusable and activated for keyboard input. Window content is
WoxView-only; use `text` and `markdown` nodes to display those formats.

Launcher results can also set `PreviewType` to `woxview` and `PreviewData` to a
serialized tree. Interactive previews need a unique `WoxViewID` and a callback
owned by the result's query. The Node `asPreview()` and Python `as_preview()`
adapters create and bind this session, dispatch events with their arguments,
and publish invalidations through `UpdateResult`. Result replacement or query
release discards the old callbacks. A query response can set `ResultPreviewWidthRatio`
to `0` for a full-width preview.

The adapters also accept show, hide and dispose hooks. Each visible period has
a cancellation signal (Node `AbortSignal`, Python `asyncio.Event`); it is
triggered before hide, and disposal hides a still-visible session first. Hooks
must arrange cleanup of plugin-owned timers and tasks, and should return promptly.

```json
{
  "version": 1,
  "root": {
    "type": "form",
    "props": { "style": { "padding": 16, "gap": 8 }, "onSubmit": { "$handler": "save" } },
    "children": [
      { "type": "text-field", "key": "title", "props": { "name": "title", "label": "Title", "onChange": { "$handler": "titleChanged" } } },
      { "type": "select", "key": "priority", "props": { "name": "priority", "label": "Priority", "value": "normal", "options": [
        { "value": "normal", "label": "Normal" }, { "value": "high", "label": "High" }
      ], "onChange": { "$handler": "priorityChanged" } } },
      { "type": "checkbox", "key": "enabled", "props": { "name": "enabled", "label": "Enabled", "value": true } },
      { "type": "submit", "key": "save", "props": { "label": "Save", "variant": "primary" } }
    ]
  }
}
```

## Tree and layout

Each node has `type`, optional `key`, optional `props`, and optional `children`.
Unknown types, fields, enum values and invalid child placement are rejected with
a path. Trees are limited to 4 MiB, 64 levels and 5,000 nodes; handler IDs are
1–128 bytes and keys at most 256 bytes. Sibling keys must be unique. Keep keys
stable across updates: identity includes parent, key and node type; unkeyed nodes
use type and position. Removal, type changes and document replacement discard
retained state. An update that fails validation leaves the previous tree intact.

`view`, `column`, `row`, `grid`, `stack`, `scroll`, `lazy-list`, and `form` accept children.
`text` accepts only direct `span` children; spans and other nodes are leaves.
View and column lay out vertically; row is horizontal. Stack
uses child `style.top`, `left`, `right`, `bottom`; opposite anchors stretch.
Scroll is vertical by default, or horizontal when `horizontal: true`.
`spacer` takes optional `size`, or expands when omitted in a flex layout.

`row.wrap: true` uses natural-size children and starts a new line when the
available logical width is exhausted. `style.gap` controls both item and line
gaps. Flex weights, expanding spacers, and main/cross-axis alignment apply to
ordinary rows; wrapped runs currently use the widget's start alignment.

`grid` requires `columns` (1-5000) and accepts `gap` (0-16384 logical units) for
both columns and rows. Each column has equal width; row height comes from its
tallest child. `style.padding` reduces the space available to cells, and
`style.alignItems` controls alignment within each row. Use `grid.gap` for grid
spacing; `style.gap` remains the flex/wrap spacing property. Supply a bounded
width, such as `style.width` when placing a grid inside a horizontal scroller.

A scroll node fills its available viewport unless `style.width` / `style.height`
is set; use `style.flex: 1` inside a column when it should take the remaining
height. Horizontal scroll lays children in a row and vertical scroll in a column.
Standalone horizontal scroll maps an ordinary wheel to its X axis. Inside another
scroller, it uses horizontal wheel/trackpad deltas and lets vertical wheel movement
reach its ancestor.

`stickToEnd` defaults to false. When enabled, initial layout starts at the trailing
edge and follows content/viewport size changes while the reader stays within one
logical unit of that edge. Scrolling away pauses following; returning to the edge
resumes it. Changing false to true explicitly moves to the current end. Switching
scroll axes resets the offset and uses the newly measured axis geometry.

`onEndReached` emits `[]` after layout when the distance to the trailing edge is
at most `endThreshold` (0-16384 logical units, default 0). It also fires for empty
or undersized content, allowing initial loading. It fires once per visit, rearming
when the reader leaves the threshold, the scroll range or direct child count
changes, or the handler ID changes. Repeated frames and identical tree updates
do not resend it. Remove the handler while loading or when there are no more
items; appending a short page can immediately request another page to fill the
viewport.

Every node accepts `props.style` (spans accept only the inline subset below).
Sizes are logical units. Layout fields are
`width`, `height`, `minWidth`, `maxWidth`, `minHeight`, `maxHeight`, `flex`,
`padding`, `margin`, `gap`, `alignItems` (`start`, `center`, `end`, `stretch`),
`alignSelf` (the same values, overriding the parent Flex alignment for this child),
and `justifyContent` (`start`, `center`, `end`, `space-between`). Padding and
margin accept a number, dimension token, or an object of `top`, `right`, `bottom`, `left`,
`horizontal`, `vertical`; explicit sides override shorthand. Decoration fields
are `backgroundColor`, `borderColor`, `borderWidth`, `borderRadius`; text fields
are `color`, `fontSize`, `fontWeight` (`regular`, `semibold`, `bold`),
`fontFamily` (`ui`, `monospace`), `italic`, `lineHeight`, `maxLines`, and
`textAlign` (`start`, `center`). Fields apply where meaningful for a component.
Colors accept `#RGB`, `#RRGGBB`, `#RRGGBBAA`, or theme tokens `$text`,
`$textSecondary`, `$accent`, `$accentText`, `$background`, `$surface`, `$border`,
`$success`, `$warning`, `$error`, `$hover`, `$selected`, `$selectedText`, `$focus`,
`$disabled`, `$link`, `$overlay`, and `$warningText`. Zero sizes use component defaults.

Dimensional values accept numbers in 0-16384 or the following tokens. The Go
contract uses `Size{Value: n}` or `Size{Token: SizeSpace2}`; JSON keeps numbers and
strings, including when the document is serialized again.

| Token | Normal-density logical size |
| --- | --- |
| `$radiusSm`, `$radiusMd` | 4, 8 |
| `$space1`, `$space2`, `$space3`, `$space4` | 4, 8, 12, 16 |
| `$fontSm`, `$fontMd`, `$fontLg`, `$fontTitle` | 12, 13, 16, 22 |

Tokens apply to style dimensions, anchors, gaps, padding/margin sides, borders,
font size and line height, as well as `grid.gap`, `spacer.size`,
`scroll.endThreshold`, `lazy-list.itemHeight`, and table column `width`.
`lazy-list.itemHeight` must remain positive. Ratios, flex weights, opacity,
counts and data values remain numeric; tokens are not CSS expressions.
Explicit zero sides override padding/margin shorthand or the shared border width.

Tokens resolve on each build from `BuildProps.Theme.DensityScale`, including live
Interface size changes. Geometry uses rounded logical units; display DPI is a
separate renderer conversion. Numeric geometry retains its existing logical-unit
behavior. Font size tokens supply normal-density bases, and the owning text
widget or shared control scales them once, as it already does for numeric fonts.
Line-height tokens follow density once as well; numeric line heights preserve
the existing behavior of their text component. Resolving a token never changes
the source document or resets field values. Virtual-row commands use the current
resolved row extent, including when density changes after a command is queued.

Every color prop, including a span color, also accepts a complete light/dark pair:
`{"light":"#222","dark":"#eee"}`. Each branch accepts a literal or token; both
must be nonempty and valid, and pairs cannot nest. A scalar keeps its existing
behavior; omission, an empty string, or null uses the component fallback. Explicit
transparent literals remain transparent. The pair is selected from
`BuildProps.Theme.Dark` on every build, then its token is resolved from that same
theme. The tree and locally edited field values survive appearance changes.

`$selected`/`$selectedText` use the control selection pair; `$focus` is the focus
color, `$disabled` is disabled foreground, `$hover` is hover fill, `$link` is link
foreground, and `$overlay` is overlay fill. `$warning` is a warning badge fill
and `$warningText` its contrasting foreground. A token changes paint only; it
does not select, disable, focus, or otherwise change the node's behavior.
Plugin windows supply the current overlay appearance and Wox theme palette;
other embedders explicitly supply their surface's `ControlTheme` (Dark defaults
to false). Settings keeps its independent fixed dark palette.

Plugin-window status roles come from the optional Wox theme fields `SuccessColor`,
`WarningColor`, `WarningTextColor`, and `ErrorColor`. Unauthored values use the
theme's appearance-aware status defaults; token resolution never writes settings.

Plugin windows also emit `theme_changed` with `Data: {"Dark": bool}` and their
current `DocumentID`. It is a window event, not a node handler. Use it when image
assets need to change; colors and tokens already resolve from the current theme.
The notification covers hidden windows and same-appearance palette changes.
Creation emits only `ready`. Document replacement and close revoke stale
notifications. See the
[window event contract](../pluginwindow/README.md) for SDK payload details.
`alignSelf` applies to ordinary row/column/view/form/scroll Flex children and
combines with `flex` weights. Omission inherits the parent; `start` explicitly
opts out of a parent's stretch. Wrapped rows, grids, and stacks retain their own
layout rules. Stretch fills a bounded cross axis; an unbounded axis keeps the
child's intrinsic extent.

`aspectRatio` is an optional positive width/height ratio, at most 16384. A supplied
width derives height, or a supplied height derives width. With neither, it fits
the available width and then height; a horizontal scroller's unbounded width
instead uses its bounded height. Both explicit dimensions take precedence over
the ratio. Min/max and parent constraints remain authoritative; conflicting
constraints can prevent the preferred ratio. Fully unbounded boxes without
dimensions retain natural sizing. The ratio describes the box including padding
and excluding margin. Scroll and lazy-list use the resulting box as their viewport.

`overflow` accepts `visible` (default) or `hidden`. Hidden clips painting and input
to the node's laid-out box, before outer margin, without changing its natural
size. This is rectangular clipping; `borderRadius` still describes box chrome.
Visible overflow does not enlarge ancestor pointer bounds.

`floating: true` uses the native floating-material container treatment. It keeps
ordinary layout and sibling ordering; use a stack or a popup control to compose
an overlay. It does not create a window or move content outside ancestor clipping.

`borderTopWidth`, `borderRightWidth`, `borderBottomWidth`, and `borderLeftWidth`
are optional widths in 0-16384 logical units. An explicit zero removes that side;
other sides inherit `borderWidth` (default 0). With no side overrides, the existing
one-unit default for a supplied `borderColor` still applies. All sides use
`borderColor`, falling back to the theme border. Side borders paint inside the
rounded silhouette and above content, with horizontal edges owning corners to
avoid double alpha blending. These box properties also apply to shared leaf
controls; their existing control appearance remains the default.

### Interaction-state styles

All nodes, including inline spans, accept optional `hoverStyle`, `pressedStyle`, `focusStyle`, and
`disabledStyle` paint patches. Each patch accepts only `color`, `backgroundColor`,
`borderColor`, `borderWidth`, `borderRadius`, the four side border widths, and
`opacity`. Layout, typography, clipping and floating properties are rejected.
Colors and dimensions accept the same literal/token forms as ordinary style.
Omission/null leaves a property unchanged; explicit transparent colors and zero
opacity/borders/radius remain meaningful.

Active patches overlay the base style in hover, pressed, then focus order.
Disabled applies only its own patch, suppressing the other states, and follows
the control's existing disabled rules, including inherited form disabled and
submit availability. These properties do not themselves disable a control.
Focus tracks entry into the node's subtree. Press currently tracks primary-pointer
input, including input consumed by descendants, and cancels on release, window
leave/blur, drag handoff or removal. State updates stay in Core and emit no plugin
events unless the producer separately supplied event handlers.

Mounted state is retained by node identity. Changing/removing paint props does
not reset field values; the source tree remains immutable. Local updates dirty
cached ancestor boundaries. Patches resolve against the current theme each build.
Buttons, links, icon buttons, text/number fields, select triggers, toggles, sliders,
hotkey recorders, code blocks, tables, and tags use explicit shared-control paint
so hover defaults do not replace an authored state color. Explicit fills are
painted once even when the outer style supplies borders. Choice groups and inline
lists paint their group fill behind transparent unselected rows; selected rows
retain their selection surface and check marks. Table state borders affect the
outer frame; internal grid separators keep their existing theme and thickness.
State decorations preserve control layout, keyboard focus, and activation when
paint wrappers or the state-style props themselves are added or removed.

Inline spans share state across their measured, wrapped fragments. Empty space
between fragments does not count as a hover target. Span backgrounds and borders
follow each fragment's line box; opacity composites its glyphs and decorations
together. Fragment observers do not intercept text selection or copying, and
state remains local to the paragraph's keyed WoxView state. Removing/reordering
spans retains only live keys. A declared span action without an event delivery
callback uses its disabled patch, matching the existing link availability policy.
Native selection colors take precedence over span backgrounds, borders and text
colors so selection remains legible. Span opacity still applies to selected
glyphs; selection highlighting and keyboard focus indicators remain native UI.

`opacity` accepts 0-1, defaults to 1, and applies once to the combined paint of
the node and its descendants. Overlapping children do not accumulate the group's
alpha independently; nested groups multiply. It preserves visible overflow,
ancestor clipping, layout dimensions, retained field state, and input/focus.
At zero the subtree is not painted, but it still participates in layout and
interaction. Disable controls separately when they should not accept input.
Native popup surfaces managed outside the document subtree retain their own
appearance. Opacity changes are local rendering state, not additional RPC events.

`shadow` accepts `sm`, `md`, or `lg`; omission or the empty string means none.
These fixed outer-shadow presets use density-scaled softness and vertical offset
(2/1, 4/2, and 8/4 normal-density logical units), with a 25% black tint. The
silhouette follows `borderRadius`. Shadows do not consume layout space or expand
hit targets. They surround the border box before margin, remain visible outside
the node's own `overflow: "hidden"`, respect ancestor clipping, and participate
in group opacity. A transparent box keeps a transparent interior. This does not
open a popup or change stacking order; later siblings can paint over the shadow.

Block-node styles accept `translateX`, `translateY`, `scale`, and `rotate`.
Translations are signed logical units (-16384 to 16384) or dimension tokens;
numeric values do not follow Interface size, while tokens do. `scale` defaults to
1, accepts signed factors in the same numeric range, and explicit 0 collapses
paint. `rotate` is clockwise degrees, defaults to 0, and accepts -16384 to 16384.
Scale and rotation apply around the border-box center, excluding outer margin,
followed by translation. Nested transforms compose locally inside their parents.
They include child paint, shadow and focus chrome without changing layout, input,
scroll extents or accessibility geometry. In particular, use disabled controls to
stop input; moving or collapsing paint does not disable the original target.
Ancestor clips remain fixed and transformed clips retain their exact silhouette.
Inline span styles and interaction-state patches do not accept transform fields.

Nodes can declare implicit transitions, driven entirely by Core:

```json
"transition": {
  "properties": ["opacity", "backgroundColor", "width", "height", "translateX", "translateY", "scale"],
  "duration": 200,
  "curve": "easeInOut"
}
```

`properties` is a nonempty, unique subset of these seven names. Duration defaults
to 200 milliseconds, accepts 0 through 60000, and explicit 0 applies immediately.
Curves are `linear`, `easeOutBack`, and `easeInOut` (the default cubic curve).
The first mount starts at the target; subsequent changes with the same node
identity interpolate from the current sample, including mid-flight retargets.
Changing the key or removing and later remounting a node starts a fresh timeline.
Removing the transition cancels it. Timing changes apply to the next target
change. A new View never inherits another View's timelines in the same Host.

Transitions consume the effective local hover/press/focus/disabled style, resolve
theme colors and dimension tokens before sampling, and leave the document
immutable. RGBA channels interpolate independently. Back-curve overshoot is
clamped for opacity/color channels and kept positive for explicit dimensions.
Numeric dimensions remain logical units; tokens follow Interface size. Width and
height interpolate between explicit positive sizes; zero/omitted natural sizes
snap and reset their channel. Parent layout constraints still apply. Transforms
retain their paint-only semantics throughout the transition.

Inline spans support `opacity` and `backgroundColor` transitions through their
state styles. All wrapped fragments share one timeline, including selectable
text; selection retains its native foreground and highlight.

Shared controls resolve their own native background before interpolation, including
hover, selection, disabled state and theme changes. When `backgroundColor` is in
the transition, an authored fill targets that control's surface; omitting it
returns smoothly to the control-owned fill. Transparent input defaults remain
transparent. Outer decoration does not composite the same fill a second time.
Table headers and bodies retain their different native fills; choice groups keep
selected rows distinct.

All nodes, including spans, accept `enter: "fade"`, `enter: "slideUp"`, or
`enter: "scale"`. The object spelling adds timing:

```json
"enter": {"type": "slideUp", "duration": 200}
```

Duration defaults to 200 milliseconds and accepts 0 through 60000; zero displays
the target immediately. Entry uses the Core ease-in-out cubic curve. All three
presets fade from zero to the effective opacity. `slideUp` also moves upward from
12 density-scaled logical units below the target. `scale` starts at 95% of the
effective scale. Existing transforms and opacity compose with this temporary
pose, after implicit transitions and before loop presets. Layout, focus and hit
geometry stay at their target positions, even while paint is transparent.

Only the first mount starts an entry. Echoing or changing its declaration on an
already-mounted key does not restart or retime it; remove/reinsert the node or
change its key to replay. Removing `enter` cancels it immediately. Adding it to
an already-mounted node does not start an animation. Lazy-list rows enter when
mounted in the visible viewport. Inline fragments share one timeline and retain
the paragraph clip and selection behavior.

`exit` accepts the same shorthand/object presets and timing as `enter`:

```json
"exit": {"type": "fade", "duration": 200}
```

When a node disappears from the tree, View retains its last painted commands and
immutable images until the exit finishes. The removed control immediately leaves
layout, focus and interaction; its snapshot paints above current content without
intercepting input. Event capabilities are revoked when the tree changes, even if
another live node uses the same handler ID. Old callbacks cannot become valid
again when a key is reinserted.

`fade` reduces the snapshot's existing opacity to zero. `slideUp` also moves it
upward by 12 density-scaled logical units; `scale` shrinks it to 95% around its
captured paint bounds. Clips, internal ancestor opacity and the current pose of
entry/loop/transition animations are frozen in the snapshot. Display scaling and
the embedding surface's outer clip/transform remain live. Inline fragments share
one exit clock and composite together in their original order, including the
editor's selection background. Scale uses their combined paint bounds. Removed
nodes form foreground layers ordered by removal batch and first capture. Backdrop materials use their
recorded style and the renderer's current backdrop.

If an exiting parent already contains a removed child, that child's separate exit
is suppressed. Reintroducing the same node identity cancels its previous exit.
Deleting another node does not restart existing exits. Only nodes painted in the
last frame have snapshots: deleting an unmounted lazy-list row or a fully hidden
subtree cannot resurrect earlier visible content. Zero duration skips capture;
removing `exit` from a live node disables its future exit. Document replacement,
namespace changes and View unmount release pending snapshots immediately.

All nodes, including spans, support one looping preset:

```json
"animation": {"type": "pulse", "duration": 1200}
```

`type` is required and accepts `pulse`, `spin`, or `shimmer`. Duration defaults to
1200 milliseconds and accepts 1 through 60000. Remove `animation` (or set it to
null) to stop immediately. No keyframes or per-frame plugin events are exposed.

- `pulse` smoothly multiplies the effective opacity from 1 down to 0.65 and back.
- `spin` adds one clockwise revolution per period to the effective rotation,
  around the same border-box center used by static transforms.
- `shimmer` sweeps a soft neutral band across the border box, excluding margin.
  It uses a dark band in light appearance and a light band in dark appearance.
  The band obeys node opacity, transforms and ancestor clipping.

Presets compose after implicit transitions. A continuing key keeps its phase
across document updates; changing the key, preset type or duration restarts it.
Deleting/remounting a node starts a fresh timeline, including lazy-list rows
unmounted outside the viewport. Core schedules local repaint regions and stops
the timeline when the node disappears. Layout, scrolling, focus and input
geometry retain their existing behavior throughout playback.

Wrapped span fragments share one phase. Each fragment spins around its own
center within the paragraph clip; shimmer skips selected text, preserving native
selection colors. Pulse composes with span state opacity.

Windows and Linux affine rendering have offscreen native coverage; macOS source
still requires native compilation and execution before the cross-platform
completion gate can pass.

Text capability audit: portable `runtime.TextStyle` supports size, weight, family,
and italic. It has no ellipsis, noWrap, underline, strikethrough, or letterSpacing
fields, so those names are not accepted in the general style contract. Ordinary
text uses the existing `maxLines` wrapping/truncation behavior; `span` separately
supports underline through the shared rich-text component. This avoids advertising
font features the runtime cannot consistently measure and render.

## Components

| Type | Props besides `style` |
| --- | --- |
| `view`, `column`, `stack` | `label`, `onPress` for an accessible clickable container |
| `row` | Container props plus `wrap` (default false); wrapped runs use `style.gap` |
| `grid` | Container props plus required `columns` and optional `gap` |
| `scroll` | Container props plus `horizontal`, `stickToEnd`, `onEndReached`, `endThreshold` |
| `lazy-list` | Container props plus required positive `itemHeight`, `stickToEnd`, `onEndReached`; each child requires a stable key |
| `text` | `text`, `selectable`, optional direct `span` children |
| `span` | `text`, `onPress`; inline style subset and common interactions; only allowed inside `text` |
| `markdown` | `content`, `onLinkPress`; links open externally only when the handler is absent |
| `image` | exactly one of `icon` (Wox catalog name) or `source` (`imageType`, `imageData`); `fit`: `contain`, `cover`, `fill`. Sources support `base64`, `svg`, `absolute`; catalog icons accept `iconColorMode` |
| `line-chart` | `values`: up to 4096 evenly spaced numbers in −1e100..1e100; optional `pointLabels` supplies one label per value for the hover readout and X axis; optional accessible `label`. `chart` configures axes, grid, fill and hover; see below. `style.color` sets the line color and `style.height` defaults to 160 logical units |
| `morph-path` | `keyframes`, `progress`, `duration`, `curve`; filled vector outlines interpolated by Core |
| `spacer` | `size` |
| `button` | `label`, optional catalog `icon` and `iconColorMode`, `variant` (`secondary`, `primary`, `outline`, `text`, `surface`), `disabled`, `onPress` |
| `icon-button` | `icon`, optional `iconColorMode`, accessible `label`, `disabled`, `onPress` |
| `text-field` | `name`, accessible `label`, `value`, `placeholder`, `maxLines`, `disabled`, `autofocus`, `password`, `onChange`, `onSubmit`, `changeDebounce` (0–60000 ms, default 0) |
| `number-field` | `name`, accessible `label`, numeric `value`, optional `min`/`max`, `step` (default 1), `placeholder`, `disabled`, `autofocus`, `onChange`, `onSubmit` |
| `hotkey` | `name`, accessible `label`, `placeholder`, shortcut-string `value` (default empty), `disabled`, `onChange`; records normal local key combinations without global registration |
| `switch`, `checkbox` | `name`, accessible `label`, boolean `value`, `disabled`, `onChange` |
| `slider` | `name`, accessible `label`, `value`, `min`, `max`, `step`, `disabled`, `onChange`; range defaults to 0–1 and value is clamped to the range |
| `select`, `list` | `name`, accessible `label`, string `value`, `disabled`, `onChange`, `options`: up to 500 `{value, label, disabled?}` objects with unique nonempty values. Empty value means no selection; select options open in a floating, scrollable menu |
| `radio-group` | Same field props and `options` as `select`; renders equal-width segments |
| `tabs` | `name`, accessible `label`, string `value`, `disabled`, `onChange`, `items`: up to 500 `{value, label, disabled?}` objects. The producer renders content according to the selected value; the tab strip accepts no children |
| `multi-select`, `tags` | `name`, accessible `label`, string-array `value` (default `[]`), `disabled`, `onChange`, `options` as above; wrapping selectable tags with a check indicator. Values must be unique members of `options` |
| `tag`, `badge` | Translatable `text`; use `style.color` for a semantic color. `tag` uses shared metadata sizing and `badge` uses the compact table status tier |
| `table` | Accessible `label`, `columns`: `{key,label,width?}`, `rows`: `{key,cells,actions?}`, `disabled`; each action has `{key,label,icon?,disabled?,onPress}`. Cells are strings in column order; row actions receive `[rowKey]` |
| `code` | Literal `code`, optional display `language`; native monospace selection and a local copy button that preserves source bytes |
| `link` | Required absolute `url` (`http`, `https`, `mailto`), optional translated `label`, `disabled`, `onPress`; handler overrides external opening and receives `[]` |
| `form` | `onSubmit`, `disabled`, `submitting`, children; named descendant fields have unique names. Nested forms are rejected |
| `submit` | Button props except `onPress`; must be inside a form and invokes its `onSubmit` |
| `progress` | `value` clamped to 0–1, `indeterminate` |
| `loading` | Rotating ring; `style.width` sets its diameter (default 16 logical units), `style.color` sets its color (default `$text`) |
| `divider` | `vertical` |

For a Windows transparent plugin window using `PassThroughTransparent`, any
node can opt its bounds into the native pointer region with `inputRegion: true`.
This adds no visible paint and permits interactions in intentionally empty space;
ordinary transparent pixels remain click-through.

### Line charts

The optional `chart` object changes only the options it specifies. Omitting it
keeps the original two-unit line, no axes or fill, and the local hover crosshair,
point, and top readout. Values remain evenly spaced; `pointLabels`, when given,
must have the same length as `values`.

| Option | Meaning and default |
| --- | --- |
| `showGrid`, `gridLines`, `gridColor` | Horizontal grid; off by default. Line count defaults to 5 (2–12); color defaults to `$border`. |
| `showXAxis`, `xTicks` | Sample labels from `pointLabels` below the plot; off by default. Maximum ticks defaults to 5 (2–12) and is reduced for narrow plots. |
| `yAxis`, `yAxisWidth` | `none` (default), `left`, or `right`; labels use the data minimum and maximum. Width defaults to 80 density-scaled logical units and can be set from 0–512 logical units. Labels are clipped to the reserved width. |
| `lineWidth`, `showPoints` | Width in logical units (default 2, greater than 0 and at most 16); individual points are off by default. The line uses `style.color`. |
| `fill`, `fillColor` | Gradient area below the line; off by default. The default is the line color at 22% opacity; an authored color may include its own alpha. |
| `plotPadding` | Plot inset in logical units (0–128); defaults to 2. Axis space is reserved separately. Use `style.padding` for space outside the component. |
| `hover`, `crosshair`, `hoverPoint`, `tooltip` | Independently enable or disable hover and its visual elements. Each defaults to true; `hover: false` disables the entire hover interaction. |
| `tooltipPosition` | `top` (default, original readout) or `floating` (near the selected point). |
| `valueDecimals`, `valuePrefix`, `valueSuffix` | Numeric labels use 2 decimals by default; choose 0–6 decimals and optional prefix/suffix (up to 32 characters each). Very large or small values use scientific notation when formatted. |

```json
{
  "type": "line-chart",
  "props": {
    "values": [130.03, 125.10, 122.40, 124.68, 123.62],
    "pointLabels": ["09:30", "11:00", "12:00", "12:45", "16:00"],
    "label": "Price",
    "style": {"height": 220, "color": "#ee6065"},
    "chart": {"showGrid": true, "showXAxis": true, "yAxis": "right", "fill": true, "tooltipPosition": "floating", "valueSuffix": " USD"}
  }
}
```

Time-range controls are composed separately, for example with `tabs` or
`radio-group`. The plugin handles their change event and supplies the selected
range's values and labels; the chart does not fetch or aggregate data.

### Morphing vector paths

`morph-path` is a decorative, filled outline that interpolates matching path
commands as `progress` changes. Coordinates use a normalized 1×1 view box:
`M` and `L` take `[x,y]`, `Q` takes `[cx,cy,x,y]`, `C` takes
`[cx1,cy1,cx2,cy2,x,y]`, and `Z` closes the current subpath. All paths must
have the same 1–128 commands, with identical operations and arities, and begin
with `M`; coordinates must be between -4 and 4. Start another subpath with `M`
after `Z`. The fill implicitly closes any open subpath. The node has no stroke.

`keyframes` contains 2–18 outlines as `{at, path}`. Positions must be strictly
increasing, beginning at 0 and ending at 1, with positions at least 1e-7 apart.
With three or more keyframes,
coordinates use monotone cubic interpolation to keep velocity continuous without
overshooting keyframes. With two keyframes, coordinates interpolate linearly.

```json
{
  "type": "morph-path", "key": "shape",
  "props": {
    "keyframes": [
      {"at":0,"path":[{"op":"M","values":[0,0]}, {"op":"L","values":[1,0]}, {"op":"L","values":[0.5,1]}, {"op":"Z"}]},
      {"at":1,"path":[{"op":"M","values":[0,0]}, {"op":"L","values":[1,0]}, {"op":"L","values":[1,1]}, {"op":"Z"}]}
    ],
    "progress": 1, "duration": 200, "curve": "easeInOut",
    "style": {"width": 32, "height": 32, "color": "$accent"}
  }
}
```

`progress` is 0–1 and defaults to 0. A first mount renders the target immediately;
later updates with the same key interpolate from the current progress, including
mid-animation retargeting. `duration` defaults to 200 ms, accepts 0–60000, and
the curve uses the same `linear`, `easeInOut`, and `easeOutBack` values as
transitions. Overshoot is clipped to 0–1. A change to the paths themselves takes
effect immediately; keep paths stable while retargeting progress. Fill uses
`style.color` (including theme tokens and light/dark pairs), defaulting to text.
The outline is rasterized locally; its longest raster dimension is capped at
1024 pixels. Very large sizes may appear softer. This is a paint-only shape:
its interaction and accessibility bounds remain rectangular. Overlay a keyed
button when the shape is a control.

Catalog icons preserve their authored colors and gradients by default (or with
`iconColorMode: "original"`). The explicit Wox SVG theme variable still resolves
to white in dark themes and black in light themes. Set `iconColorMode: "tint"`
for a single-color glyph that follows the control's foreground. Disabled
original-color button icons fade without losing their colors. `iconColorMode`
does not apply to image `source`, which already preserves authored colors.

All value fields (`text-field`, `switch`, `checkbox`, `slider`, `select`, `list`,
`radio-group`, `tabs`, `multi-select`, `tags`, `number-field`, `hotkey`)
also accept `revision` (unsigned integer, default 0), `required` (default false),
and `error` (default empty). `required` adds a visible label and `*` to the
accessible name; it does not block submission. `error` appears below the control
in `$error`. The producer validates submitted values and returns field errors in
the next tree.

Radio groups, tabs, and selectable tags support arrow keys, Home/End, and initial-letter
navigation, skipping disabled choices. Enter/Space activates the focused item; navigation
alone does not change the value. Multi-select/tags append new selections to the value
array and remove deselected values while preserving the order of the others. Removing
an option prunes it from local selections, including offscreen fields; it does not
discard unrelated selections. Use `revision` to force an empty array or another selection.
Option labels are translated, while option values and event values remain literal.

Number fields keep incomplete drafts (such as `-`) in the native editor without emitting
invalid values. Finite in-range edits emit `[number]`. Enter or focus loss clamps to the
range and snaps to `step` using `min` (or zero) as the origin; invalid drafts revert to
the last accepted value. Up/Down steps the value. Enter invokes the field's `onSubmit`
with `[number]`, or submits the enclosing form. Range changes also clamp retained
offscreen values. `min`/`max` are optional, and `step: 0` uses the default of 1.

Hotkey fields accept the shared normal Wox shortcut grammar. While focused, recording
owns matching keys before ancestor shortcuts. Escape or plain Enter ends recording;
Tab/Shift+Tab moves focus; Backspace clears the value. Modified navigation keys can be
recorded, while composition, repeats, and modifier-only events do not produce changes.
This is local combination entry; native hold/double-modifier triggers and global
availability checks are outside the field's contract.

Tables require 1–64 columns, at most 1000 rows and 10000 cells, and at most 32 actions
per row. Column keys and row keys are unique within their collections; action keys are
unique within a row, and all keys are nonempty and at most 256 UTF-8 bytes. Each row has
exactly one string per column. Missing column widths share available space with a
120-unit density-scaled minimum; wide tables scroll horizontally. `style.height` bounds
the vertical viewport while the header stays fixed. Action labels appear as tooltips,
and omitted action icons use `action.execute`. Labels and cell text can use `i18n:`;
row keys, handlers, URLs, hotkey values, and code source remain literal.

A form's `disabled` disables every descendant field and submit control.
`submitting` disables submit controls and shows a loading indicator while leaving
fields editable. Both states suppress implicit keyboard submission. Single-line
fields with no own `onSubmit` submit their enclosing form on Enter; multiline
fields use Cmd+Enter on macOS and Ctrl+Enter elsewhere, keeping plain Enter for
newlines. An explicit field `onSubmit` takes precedence and still sends `[string]`.

## Events and updates

An event handler is an opaque `{ "$handler": "id" }` owned by the producer;
no code is executed from JSON. A plugin window emits `woxview_event` with
`WindowID`, `DocumentID`, and `Data: {"Handler":"id","Args":[...],"Key":"node-key"}`.
`Key` is the triggering node's explicit key (empty for an unkeyed node).
Press and end-reached events send `[]`, text change or submit sends `[string]`, switch/checkbox sends
`[boolean]`, slider/number-field send `[number]`, and select/list/radio-group/tabs/hotkey send `[string]`.
Multi-select/tags send `[["value1", "value2"]]` (one array-valued argument). Form
submit sends `[{"fieldName": value, ...}]` containing all named descendant
fields, including disabled fields, with their current UI values. Field change
handlers are optional. Validation and persistence are plugin responsibilities.

Fields update locally while callbacks travel to the plugin. Values live in the
View rather than being discovered during Build, so cached subtrees still submit
the user's edits. Removed field identities are discarded by `SetTree`.

- Controlled mode: echo `onChange` values in subsequent trees. With the same
  `revision`, recent delayed echoes acknowledge edits without rewinding newer text.
- Uncontrolled mode: leave `value` unchanged to retain local edits.
- Reset/force mode: increment that field's `revision` and send the desired `value`.
  For example, `{ "value": "", "revision": 1 }` clears an edited text field even
  if its initial value was already empty. This clears pending echo history and
  moves the caret to the end without emitting `onChange`.

A field's `name` affects the form snapshot; `key` controls retained widget
identity. Keep revision values in subsequent echoes and serialize updates (or
use `Sequence` below), since revision is a reset signal, not an ordering token.

Callbacks are serialized per window and delivered off the UI thread. A bounded
queue can drop events if the callback is too slow; an `error` event reports this
when delivery resumes, so the plugin can rebuild or ask the user to retry. Closing
or replacing a document invalidates its queued events. Updating a tree also drops
queued WoxView callbacks whose handler ID no longer appears in the current tree.
To prevent an asynchronous
result from updating a replacement document, pass its `DocumentID` to
`WindowContentUpdate`. Producers that may complete updates out of order can pass
monotonically increasing nonzero `Sequence` values per document; stale and
unsequenced updates are rejected once sequencing starts. Invalid or stale updates
return an error and leave the current tree unchanged.

## SDK wire types

Node.js and Python expose types generated from this schema. Both use the exact
JSON property names and values, including `onPress`, `$handler` and `version`.
These describe serializable protocol data; callback functions and application
state remain in the plugin. The same [counter fixture](testdata/sdk-counter.json)
is used by the Go parser and both SDK tests.

TypeScript imports `WoxViewTree`, `WoxViewNode`, `WoxViewHandler`,
`WoxViewEventData`, and component-specific types from `@wox-launcher/wox-plugin`:

```typescript
import type { WoxViewTree, WoxViewEventData } from "@wox-launcher/wox-plugin"

const tree: WoxViewTree = {
  version: 1,
  root: { type: "column", children: [
    { type: "text", props: { text: "Count: 0" } },
    { type: "button", key: "increment", props: { label: "Add", onPress: { $handler: "increment" } } }
  ] }
}
const data = JSON.stringify(tree)
// Pass data in CreateWindow's { Type: "woxview", Data: data }.
// In OnEvent, inspect `event.Type === "woxview_event"` and cast
// `event.Data` to WoxViewEventData before dispatching its Handler and Args.
```

Python uses the same object and wire spelling:

```python
import json
from wox_plugin import WoxViewTree

tree: WoxViewTree = {
    "version": 1,
    "root": {"type": "column", "children": [
        {"type": "text", "props": {"text": "Count: 0"}},
        {"type": "button", "key": "increment", "props": {"label": "Add", "onPress": {"$handler": "increment"}}},
    ]},
}
data = json.dumps(tree, ensure_ascii=False)
# Pass data in WindowContent(WindowContentType.WOXVIEW, data).
```

Python exports `WoxViewTree`, `WoxViewNode`, `WoxViewHandler` and
`WoxViewEventData` from `wox_plugin`; all component prop types are available
from `wox_plugin.models.woxview`. Types support Python 3.10 and newer.

For either language, install the window callback during creation, dispatch events
by `Handler`, and pass `Args` to the plugin's own callback. `ready` may arrive
before creation returns. Keep each document's handler registry and state scoped
to its window, discard them on close or replacement, and avoid running slow work
in the event callback. For tree changes use `Content.Update` / `content.update`,
with that document's ID and increasing sequence; check the result. `Content.Set`
replaces the document instead. These SDK types do not install callbacks, manage
state or perform runtime validation automatically.

Single-file Python plugins can import these types when the host includes the
updated SDK. Single-file JavaScript can use the identical plain object without
any runtime package dependency. For editor type checking, a development-only SDK
installation can supply `/** @typedef {import("@wox-launcher/wox-plugin").WoxViewTree} WoxViewTree */`
and `/** @type {WoxViewTree} */`; type-only JSDoc is not a runtime import.

Regenerate SDK declarations after changing the Go catalog or props with
`python wox.core/ui/woxview/generate_sdk_types.py` from the repository root;
pass `--check` to verify checked-in output. The script runs
`go run ./ui/woxview/cmd/schema` from `wox.core`, so Go and the dependencies
required to build WoxView must be installed. It reads `JSONSchema()` directly,
without reading the checked-in test fixture. Static types do not express every
schema or cross-node constraint: for example, Python cannot express the lazy-list
child-key intersection. Core validates all trees, including limits, identities,
forms, ranges and catalog icons. See Schema and compatibility below.

## Debounced changes

Text fields can set `changeDebounce: 200` to emit only the latest `onChange` after
200 ms without another edit. The local editor and form value update immediately;
zero retains immediate delivery. Blur and submit flush pending changes before
their subsequent events, so no delayed edit arrives after its form submission.
Virtualizing a row does not discard its value or notification. Removing a field,
changing its handler or debounce interval, disabling it or its form, applying a
revision reset or authoritative value, replacing the document, or unmounting the
View cancels that field's pending notification. Stale value echoes do not cancel
newer local edits. Core owns the timer; no per-frame RPC is involved.

## Content commands

`window.Content.Command(ctx, {DocumentID?, Command, Key})` in Go/Node, or
`window.content.command(ctx, WindowContentCommandOption(...))` in Python, queues
an operation on the current WoxView document. `Command` is one of:

| Command | Behavior |
|---|---|
| `focus` | Focus the first enabled focusable control in the keyed node's subtree |
| `scrollTo` | Minimally scroll ancestors to reveal the keyed node, without requesting focus |
| `selectAll` | Focus and select all text in the first enabled text control in that subtree, including read-only code |

`Key` is the explicit JSON node key, not an internal widget path. It must identify
exactly one node across this document; duplicate keys in separate branches remain
legal for rendering but are ambiguous command targets. Lazy rows are materialized
from the outermost list inward. Commands follow stable keys across row reordering;
moving a node to another parent or changing its type replaces the target.

`Success` means accepted. Commands run FIFO after layout, including commands from
the initial ready callback before the first frame. At most 64 may be pending.
Hidden windows execute on their next layout. Unknown commands, missing/ambiguous
keys, stale `DocumentID`, unsupported content, and a full queue fail immediately.
Execution failures (removed/disabled targets, no suitable control, or an active
modal menu) emit `error` with that document's ID and `Data: {Command, Key}`.
`Content.Set` and close discard the previous document's deferred work. Omitting
`DocumentID` targets the current document at acceptance, never a later replacement.

Focus and selection require `Focusable: true` and do not activate the native
window; use `Show({Activate: true})` when activation is intended. `scrollTo` also
works in nonfocusable windows. Coordinates and lazy row extents remain logical
units at every display scale. Commands are RPC options, not JSON tree props.

## Translation

Embedders may supply `BuildProps.Translate func(string) string`. During Build,
WoxView passes display strings starting with `i18n:` to this function, including
the prefix. Supported display props are `text`, `label`, `placeholder`,
`options.label`, and `error`. Literal strings (including whitespace) remain
unchanged. With no hook, the original string is displayed. Missing-key fallback
belongs to the embedding translator.

Translation uses a copy of presentation props; it never mutates the validated
tree or translates field values, names, option values, node keys, handler IDs,
Markdown content, or event arguments. Rebuilding with another locale updates
labels without resetting local edits. Hosts using `widget.Boundary` must include
the locale or translation revision in Boundary props so a locale change rebuilds
the cached subtree.

Plugin windows receive their owner's existing plugin/Wox translation lookup
through the internal window factory. Future settings embedders can supply
Wox's translator without coupling WoxView to plugin or global i18n state.

## Schema and compatibility

`JSONSchema()` generates Draft 2020-12 JSON Schema from the Go catalog, props
tags and event structure, which define the protocol. `testdata/woxview.schema.json` is a golden test
fixture for detecting contract changes, not the SDK generator's input. Tests
compare it byte-for-byte; after an intentional contract change, regenerate it from
`wox.core` with `WOX_UPDATE_SCHEMA=1 go test ./ui/woxview -run TestJSONSchemaGolden`
(PowerShell: `$env:WOX_UPDATE_SCHEMA='1'; go test ./ui/woxview -run TestJSONSchemaGolden`).
Unset the variable before ordinary verification.

`Parse` and `NewTree` return `*ValidationError` on validation failure. Use
`errors.As(err, &validation)` to inspect `Path` and `Message`. Paths follow the
existing diagnostic notation, for example `root/column[0]/text.props`; `$` denotes
the document, and `version` / `root` identify document fields. `Message` contains
the complete existing diagnostic (including any path prefix), and `Error()`
returns it unchanged. Plugin window `ErrMsg` therefore keeps its original text.

The schema describes canonical JSON field names, types, enums and local bounds.
Core validation additionally checks catalog icon names, unique sibling keys and
option values, selected-option membership, range relationships, form ownership,
unique field names, byte limits and recursive depth/node limits. Go's decoder
also accepts legacy case-insensitive field spelling. Schema length constraints
count Unicode characters; Core's key/handler limits count UTF-8 bytes.

Backward-compatible additions with defaults that preserve existing behavior stay
on `version: 1`. An incompatible change requires version 2 and a Core decoder
that supports both versions during migration.

To add a component, register its props/build function in `catalog.go`, implement
`handlerOwner` for callback props and `fieldOwner` for form values, and add enum
values where applicable. Update this component table and the schema golden, then
cover invalid input and rendering/interaction. `Tree.handlerSet` is built during
validation, so queued-event ownership is a constant-time lookup on the UI thread.

## Virtual lists

`lazy-list` is a vertical viewport with fixed `itemHeight` in logical units
(greater than 0, at most 16384). Every direct child must have a nonempty unique
key. Only visible items and the widget's small overscan region are built; rows
are clipped to the declared height. Set `style.height` or give the list bounded
space in its parent. `stickToEnd` and `onEndReached` share scroll-node behavior.

Field values and their pending producer echoes belong to the document, not to
visible widgets. Scrolling a field out of view and back preserves its edit.
Producer updates, including revision resets, also apply to offscreen fields and
form snapshots. Removing a field from the tree discards its value and echo state.

## Inline rich text

`text` concatenates its own `text` with each direct `span` child's translated
`text`, preserving spaces and newlines without inserting separators. A span
cannot be a root, appear inside another container, or contain children. Its
`style` accepts only `color`, `fontWeight`, `fontFamily`, `italic`, and `underline`.
Unspecified color, weight, family, and italic inherit from the parent text;
explicit `regular`, `ui`, or `false` can override inherited traits.

```json
{"type":"text","props":{"text":"Read ","selectable":true},"children":[
  {"type":"span","props":{"text":"the guide","style":{"fontWeight":"semibold"}}},
  {"type":"span","key":"details","props":{"text":" or open details","onPress":{"$handler":"details"}}}
]}
```

An `onPress` span is a link: it defaults to underline and, when no explicit
parent/span color is supplied, the accent color. `underline: false` overrides
that default. Click or Enter/Space emits `[]` with the span's `Data.Key`; the
handler decides what to open. A wrapped link has one keyboard focus stop and
separate pointer regions for its visible fragments. Fully clipped links cannot
receive focus. Common tooltips, shortcuts, and context menus also apply to spans.

The parent owns wrapping, line height, centering, and `maxLines` clipping.
`selectable: true` uses the same font measurements for text, selection, and links;
dragging from ordinary text selects across styled spans. Copy preserves the
concatenated text, including whitespace and Unicode. Select-all includes clipped
text without scrolling the text away from its link regions.

## Common interactions

Every node accepts `tooltip`, `onHoverChange`, `onDoubleTap`, `onContextMenu`,
and `onFocusChange` alongside its component-specific props. Tooltip display text
uses the same `i18n:` translation hook as labels. Hints use a window-level floating
surface, flip above their trigger when needed, stay inside the logical viewport,
and preserve the trigger's layout, click handling, and keyboard focus. They do
not replace an existing interactive overlay.

Hover and focus events send `[boolean]`; double-tap and context-menu events send
`[]`. Containers observe descendant hover, double-taps, and secondary presses
without replacing child control behavior. Focus reports entry/exit of the subtree,
so moving between two fields in the same observed group does not blur that group.
Merely adding a focus listener does not create another Tab stop. `onLinkPress`
on Markdown sends `[url]` and suppresses automatic external opening.

## Choice menus, context menus, and shortcuts

Select menus render above the window content and never expand the document layout
or inherit an ancestor's scroll clipping. Arrow keys skip disabled rows; Home/End
jump to the first/last enabled row. Typing an initial letter cycles matching labels.
Enter/Space activates the focused row. Escape, Tab, or an outside click dismisses
the popup without selecting; focus returns to the trigger. Long menus scroll the
focused row into view. Inline `list` uses the same row navigation; give it
`style.height` for a bounded scrollable viewport. Removing/disabling a select
closes its menu; updating options or translations refreshes an open menu.

Every node accepts `contextMenu: [{label, icon?, onPress, disabled?}]` (at most 500
items). Labels and handlers are required; icons use the Wox catalog. Labels pass
through the `i18n:` display translation hook. Secondary click opens a local Wox
popup; only activating an enabled item emits its `onPress` with `[]`. The event's
`Key` identifies the node owning the context menu. An optional `onContextMenu`
event still reports the secondary press. Open menus update with the tree and
cannot outlive their owner.

Every node also accepts `shortcuts: [{key, onPress}]` (at most 128 entries).
Shortcut syntax reuses Wox's normal hotkey parser; `mod` means Command on macOS
and Control elsewhere. Equivalent duplicate keys in one scope are rejected.
Shortcuts fire on key-down, outside IME composition, while focus is in the node's
subtree. The nearest matching scope wins before ordinary control key handling.
Root shortcuts also work when no control has focus. Modal popups isolate their
keys from background scopes. Matching emits `[]` with the declaring node's `Key`;
no additional focus stop is created.

## Pending change delivery

Value changes are marked internally as coalescible. When a plugin callback is
slow, pending changes to the same handler and keyed node in the same document
are replaced by their latest value before queue capacity is checked. Different
nodes and documents remain independent. Discrete events such as press and submit
are ordering barriers: an edit after submit never overwrites an edit queued before
it. In-flight callbacks are not cancelled. The queue remains bounded, and capacity
failures for new, nonmergeable events still report an error. These internal hints
are not serialized into the public event payload.
