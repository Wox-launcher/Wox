//go:build !wox_automation

package automation

import "context"

// Start is intentionally inert in production builds.
func Start(context.Context, Controller) (Info, error) {
	return Info{}, nil
}
