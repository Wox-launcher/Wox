package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"wox/common"
	"wox/util/fileicon"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// resolveInboxAppsFolderPath maps a Known Folder AppsFolder ID to a real file.
func resolveInboxAppsFolderPath(appID string) string {
	lower := strings.ToLower(strings.TrimSpace(appID))
	_, suffix, ok := strings.Cut(appID, `\`)
	if !ok || strings.TrimSpace(suffix) == "" {
		return ""
	}

	systemRoot := getWindowsSystemRoot()
	switch {
	case strings.HasPrefix(lower, appsFolderSystem32GUID):
		return filepath.Join(systemRoot, "System32", suffix)
	case strings.HasPrefix(lower, appsFolderSysWOW64GUID):
		return filepath.Join(systemRoot, "SysWOW64", suffix)
	case strings.HasPrefix(lower, appsFolderWindowsGUID):
		return filepath.Join(systemRoot, suffix)
	default:
		return ""
	}
}

// listWindowsAppsFolderEntries enumerates shell:AppsFolder through in-process COM.
// This matches Start Search and PowerToys Run more closely than Get-StartApps,
// which only returns apps currently listed in the Start menu.
func listWindowsAppsFolderEntries() ([]appsFolderEntry, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initialized := false
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); ok {
			switch oleErr.Code() {
			case ole.S_OK, oleSFalse:
				initialized = true
			case rpcEChangedMode:
			default:
				return nil, fmt.Errorf("CoInitializeEx failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("CoInitializeEx failed: %w", err)
		}
	} else {
		initialized = true
	}
	if initialized {
		defer ole.CoUninitialize()
	}

	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return nil, fmt.Errorf("create Shell.Application COM object: %w", err)
	}
	defer unknown.Release()

	shellDispatch, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("query IDispatch from Shell.Application: %w", err)
	}
	defer shellDispatch.Release()

	folderVariant, err := oleutil.CallMethod(shellDispatch, "NameSpace", "shell:AppsFolder")
	if err != nil {
		return nil, fmt.Errorf("open shell:AppsFolder: %w", err)
	}
	defer folderVariant.Clear()

	folder := folderVariant.ToIDispatch()
	if folder == nil {
		return nil, fmt.Errorf("shell:AppsFolder IDispatch is nil")
	}

	itemsVariant, err := oleutil.CallMethod(folder, "Items")
	if err != nil {
		return nil, fmt.Errorf("read AppsFolder items: %w", err)
	}
	defer itemsVariant.Clear()

	items := itemsVariant.ToIDispatch()
	if items == nil {
		return nil, fmt.Errorf("AppsFolder items IDispatch is nil")
	}

	countVariant, err := oleutil.GetProperty(items, "Count")
	if err != nil {
		return nil, fmt.Errorf("read AppsFolder item count: %w", err)
	}
	defer countVariant.Clear()

	count := int(countVariant.Val)
	if count <= 0 {
		return nil, nil
	}

	entries := make([]appsFolderEntry, 0, count)
	for i := 0; i < count; i++ {
		itemVariant, itemErr := oleutil.CallMethod(items, "Item", i)
		if itemErr != nil {
			// A partial snapshot must not make reconciliation remove installed apps.
			return nil, fmt.Errorf("read AppsFolder item %d: %w", i, itemErr)
		}
		item := itemVariant.ToIDispatch()
		if item == nil {
			itemVariant.Clear()
			return nil, fmt.Errorf("AppsFolder item %d IDispatch is nil", i)
		}

		name, nameErr := olePropertyString(item, "Name")
		appID, pathErr := olePropertyString(item, "Path")
		if pathErr == nil && appID == "" {
			appID, pathErr = oleMethodString(item, "ExtendedProperty", "System.AppUserModel.ID")
		}
		packageFamily, familyErr := oleMethodString(item, "ExtendedProperty", "System.AppUserModel.PackageFamilyName")
		itemVariant.Clear()

		if err := errors.Join(nameErr, pathErr, familyErr); err != nil {
			return nil, fmt.Errorf("read AppsFolder item %d metadata: %w", i, err)
		}
		if name == "" || appID == "" {
			return nil, fmt.Errorf("AppsFolder item %d has an empty name or app ID", i)
		}
		entries = append(entries, appsFolderEntry{Name: name, AppID: appID, Packaged: packageFamily != "" && !isAppsFolderWebAppID(appID)})
	}

	return entries, nil
}

// ListExtraApps returns the current AppsFolder entries while retaining cached icons.
func (a *WindowsRetriever) ListExtraApps(ctx context.Context, existingByPath map[string]appInfo) ([]appInfo, error) {
	entries, err := listWindowsAppsFolderEntries()
	if err != nil {
		return nil, err
	}
	apps := extraAppsFromFolderEntries(entries, existingByPath, func(path string) string {
		return strings.ToLower(path)
	})
	for i := range apps {
		if !apps[i].Icon.IsEmpty() && apps[i].Icon.ImageData != appIcon.ImageData {
			continue
		}
		appID := strings.TrimPrefix(apps[i].Path, "shell:AppsFolder\\")
		resolved := resolveInboxAppsFolderPath(appID)
		if resolved == "" {
			continue
		}
		iconPath, iconErr := fileicon.GetFileIconByPath(ctx, resolved)
		if iconErr != nil {
			continue
		}
		apps[i].Icon = common.NewWoxImageAbsolutePath(iconPath)
		apps[i].IsDefaultIcon = false
		apps[i].IconSourcePath = filepath.Clean(resolved)
	}
	return apps, nil
}

// olePropertyString preserves read failures so callers can reject incomplete snapshots.
func olePropertyString(dispatch *ole.IDispatch, name string) (string, error) {
	value, err := oleutil.GetProperty(dispatch, name)
	if err != nil {
		return "", err
	}
	defer value.Clear()
	return strings.TrimSpace(value.ToString()), nil
}

// oleMethodString distinguishes unavailable metadata from a failed COM call.
func oleMethodString(dispatch *ole.IDispatch, name string, arg interface{}) (string, error) {
	value, err := oleutil.CallMethod(dispatch, name, arg)
	if err != nil {
		return "", err
	}
	defer value.Clear()
	return strings.TrimSpace(value.ToString()), nil
}
