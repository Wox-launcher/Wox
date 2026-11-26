//go:build !linux && !darwin && !windows

package supervisor

import "context"

func (m *Manager) watchUI(context.Context, int) func() { return func() {} }

func (*Manager) StartUIHeartbeat(context.Context, func(func()) error) {}
