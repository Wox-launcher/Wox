# Desktop capture

This package owns capture resources and complete Windows GDI, DXGI and WinRT
bridges. It receives physical HWND/rectangle capabilities and returns image
buffers; it does not create Wox windows, schedule selection queries, select UI
elements or retain UI sessions.

The window backend owns logical-window DPI conversion and automation image
export. Native packed buffers live in `../graphics`. Tests and hardware fixtures
remain with capture; hardware measurements create generic Win32 fixtures and are
enabled only by their existing environment variables.
