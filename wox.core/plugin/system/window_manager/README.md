# Window management

Windows and macOS support rectangle placement and saved workspace layouts.
Hyprland exposes a smaller set of native compositor operations: focus a listed
window, swap a tiled window with a neighbor above, below, to the left, or to the right, toggle floating or fullscreen, maximize, move to another display, and move to the numbered workspaces shown by Omarchy’s bar. Swaps are offered only when an adjacent tiled window is on the same workspace and monitor. Moving to a workspace follows the window there; its current workspace is omitted from the results. Rectangle
placement, minimization, and saved workspace layouts are not exposed there.

The launcher captures the Hyprland window address and PID before it appears.
Every action verifies both against the current compositor client list. Display
navigation orders monitors by logical desktop position and uses the captured
window's monitor, so a launcher opened on another display does not change the
target. Display-move results are shown only when at least two monitors are connected. Maximize toggles the target window's maximized state. Hyprland remains responsible for tiling and compositor geometry.
