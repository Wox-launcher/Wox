# Desktop window ownership

This package owns external desktop-window discovery, activation, management,
Explorer and file-dialog automation, and reusable native window capabilities.
It does not own Wox UI sessions, visibility policy, focus restoration timers,
or editor focus decisions.

On Windows, `AttachThreadInput` temporarily joins the foreground input queue and
optionally the target window's queue. Its native bridge is also used by external
window management. The caller must remain on the same OS thread through `Close`;
only successful attachments are detached, in reverse order, and cleanup is
idempotent. A zero target attaches only the foreground queue. The caller explicitly
chooses the direction, and cleanup uses the same ordered thread pair. Wox's owned
window fallback uses `QueuesToCurrentThread` (foreground to current); external
window activation uses `CurrentThreadToQueues` (current to foreground and target).
The capability neither activates a window nor selects an editor.

Wox runtime keeps its own activation sequence and focuses only its own surface.
External-window activation preserves the target application's child-editor
focus. Both policies use this package's shared attachment capability, without
native UI headers, callbacks, or window/session types crossing the boundary.

The thread-input fixture uses fake Win32 queues to cover both argument directions, partial failure,
same-thread targets, duplicate queues, and repeated cleanup without changing
live desktop focus. Live activation tests are separate.
