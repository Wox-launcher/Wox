//go:build windows

package util

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// EnsureDeepLinkProtocolHandler registers the current executable for wox URLs.
func EnsureDeepLinkProtocolHandler(ctx context.Context) bool {
	executable, err := os.Executable()
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to resolve executable for protocol handler: %s", err.Error()))
		return false
	}

	protocolKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\wox`, registry.SET_VALUE)
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to create protocol key: %s", err.Error()))
		return false
	}
	defer protocolKey.Close()
	if err := protocolKey.SetStringValue("", "URL:wox Protocol"); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to name protocol key: %s", err.Error()))
		return false
	}
	if err := protocolKey.SetStringValue("URL Protocol", ""); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to mark URL protocol key: %s", err.Error()))
		return false
	}

	commandKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\wox\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to create protocol command key: %s", err.Error()))
		return false
	}
	defer commandKey.Close()
	if err := commandKey.SetStringValue("", fmt.Sprintf(`"%s" "%%1"`, executable)); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to register protocol command: %s", err.Error()))
		return false
	}
	return true
}
