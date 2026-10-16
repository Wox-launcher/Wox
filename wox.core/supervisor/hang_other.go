//go:build !windows

package supervisor

import "context"

func (m *Manager) watchUI(context.Context, int) func() { return func() {} }
