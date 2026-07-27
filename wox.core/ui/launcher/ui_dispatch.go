package launcher

import "fmt"

// runOnUI synchronously transfers mutable launcher state work to the native UI thread.
// Tests that construct App directly omit uiCall and therefore execute inline.
func (a *App) runOnUI(operation string, fn func()) error {
	if fn == nil {
		return nil
	}
	if a.uiCall == nil {
		fn()
		return nil
	}
	if err := a.uiCall(fn); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}
