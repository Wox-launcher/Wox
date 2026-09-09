//go:build windows

package util

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// EnsureDeepLinkProtocolHandler registers the current executable for wox URLs
// and the .wox plugin package file association.
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

	if err := registerPluginPackageAssociation(ctx, executable); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to register .wox file association: %s", err.Error()))
		return true
	}
	notifyFileAssociationChanged()
	return true
}

const windowsPluginPackageProgID = "Wox.PluginPackage"

// registerPluginPackageAssociation makes Explorer open *.wox files with Wox.
func registerPluginPackageAssociation(ctx context.Context, executable string) error {
	extensionKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+PluginPackageExtension, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create extension key: %w", err)
	}
	defer extensionKey.Close()
	if err := extensionKey.SetStringValue("", windowsPluginPackageProgID); err != nil {
		return fmt.Errorf("set extension progid: %w", err)
	}
	if err := extensionKey.SetStringValue("Content Type", PluginPackageMIMEType); err != nil {
		return fmt.Errorf("set extension content type: %w", err)
	}

	progIDKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+windowsPluginPackageProgID, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create progid key: %w", err)
	}
	defer progIDKey.Close()
	if err := progIDKey.SetStringValue("", "Wox Plugin Package"); err != nil {
		return fmt.Errorf("name progid key: %w", err)
	}

	iconKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+windowsPluginPackageProgID+`\DefaultIcon`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create default icon key: %w", err)
	}
	defer iconKey.Close()
	if err := iconKey.SetStringValue("", fmt.Sprintf(`"%s",0`, executable)); err != nil {
		return fmt.Errorf("set default icon: %w", err)
	}

	openCommand := fmt.Sprintf(`"%s" "%%1"`, executable)
	openKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+windowsPluginPackageProgID+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create open command key: %w", err)
	}
	defer openKey.Close()
	if err := openKey.SetStringValue("", openCommand); err != nil {
		return fmt.Errorf("set open command: %w", err)
	}

	appKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\Applications\`+filepath.Base(executable)+`\SupportedTypes`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create application supported types: %w", err)
	}
	defer appKey.Close()
	if err := appKey.SetStringValue(PluginPackageExtension, ""); err != nil {
		return fmt.Errorf("set application supported type: %w", err)
	}

	appOpenKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\Applications\`+filepath.Base(executable)+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create application open command: %w", err)
	}
	defer appOpenKey.Close()
	if err := appOpenKey.SetStringValue("", openCommand); err != nil {
		return fmt.Errorf("set application open command: %w", err)
	}

	GetLogger().Info(ctx, "registered .wox file association")
	return nil
}

func notifyFileAssociationChanged() {
	const (
		SHCNE_ASSOCCHANGED = 0x08000000
		SHCNF_IDLIST       = 0x0000
	)
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	procSHChangeNotify := shell32.NewProc("SHChangeNotify")
	_, _, _ = procSHChangeNotify.Call(SHCNE_ASSOCCHANGED, SHCNF_IDLIST, 0, 0)
}
