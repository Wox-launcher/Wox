//go:build !linux

package sys

import "context"

func (r *SysPlugin) appendOmarchyCommands(ctx context.Context) {}
