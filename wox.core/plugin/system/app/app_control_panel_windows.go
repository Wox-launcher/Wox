package app

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"wox/common"
	"wox/util"
	"wox/util/fileicon"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const (
	controlPanelAllItemsNamespace = controlPanelRootParsingName + `\0`
	controlPanelAllTasksNamespace = `shell:::` + controlPanelAllTasksGUID
)

// getControlPanelApps indexes classic Control Panel applets and their tasks.
// Modern Settings pages stay in the explicit ms-settings table because this
// namespace does not expose them.
func (a *WindowsRetriever) getControlPanelApps(ctx context.Context) []appInfo {
	entries, err := listControlPanelEntries()
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("control panel enumeration failed: %v", err))
		return nil
	}

	apps := make([]appInfo, 0, len(entries))
	for _, entry := range entries {
		apps = append(apps, appInfo{
			Name:          entry.Name,
			Path:          entry.Path,
			Icon:          appIcon,
			Type:          AppTypeWindowsSetting,
			IsDefaultIcon: true,
		})
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("Loaded %d Control Panel items", len(apps)))
	return apps
}

// attachControlPanelIcons resolves the Shell icon for each indexed Control Panel item.
// Tasks are read in one folder enumeration because their parsing names are not directly resolvable.
func (a *WindowsRetriever) attachControlPanelIcons(ctx context.Context, apps []appInfo) {
	needsTasks := false
	for _, app := range apps {
		if isControlPanelTaskPath(app.Path) {
			needsTasks = true
			break
		}
	}
	taskIcons := map[string]string{}
	if needsTasks {
		icons, err := fileicon.GetShellFolderIcons(ctx, controlPanelAllTasksNamespace)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("control panel task icons failed: %v", err))
		}
		if icons != nil {
			taskIcons = icons
		}
	}

	resolved := 0
	failed := 0
	for i := range apps {
		if !isControlPanelAppPath(apps[i].Path) {
			continue
		}
		var iconPath string
		if isControlPanelTaskPath(apps[i].Path) {
			iconPath = taskIcons[strings.ToLower(apps[i].Path)]
		} else {
			var iconErr error
			iconPath, iconErr = fileicon.GetShellNamespaceIcon(ctx, apps[i].Path)
			if iconErr != nil {
				iconPath = ""
				util.GetLogger().Debug(ctx, fmt.Sprintf("control panel icon failed: name=%s err=%v", apps[i].Name, iconErr))
			}
		}
		if iconPath == "" {
			failed++
			continue
		}
		apps[i].Icon = common.NewWoxImageAbsolutePath(iconPath)
		apps[i].IsDefaultIcon = false
		resolved++
	}
	if resolved+failed > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf("Control Panel icons: resolved=%d failed=%d", resolved, failed))
	}
}

// listControlPanelEntries reads All Control Panel Items and the All Tasks namespace.
func listControlPanelEntries() ([]controlPanelEntry, error) {
	applets, err := listShellNamespaceItems(controlPanelAllItemsNamespace)
	if err != nil {
		return nil, fmt.Errorf("list control panel applets: %w", err)
	}
	tasks, err := listShellNamespaceItems(controlPanelAllTasksNamespace)
	if err != nil {
		return nil, fmt.Errorf("list control panel tasks: %w", err)
	}
	return mergeControlPanelEntries(applets, tasks), nil
}

// listShellNamespaceItems enumerates a Shell folder through in-process COM.
func listShellNamespaceItems(namespace string) ([]controlPanelEntry, error) {
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

	folderVariant, err := oleutil.CallMethod(shellDispatch, "NameSpace", namespace)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", namespace, err)
	}
	defer folderVariant.Clear()

	folder := folderVariant.ToIDispatch()
	if folder == nil {
		return nil, fmt.Errorf("%s IDispatch is nil", namespace)
	}

	itemsVariant, err := oleutil.CallMethod(folder, "Items")
	if err != nil {
		return nil, fmt.Errorf("read %s items: %w", namespace, err)
	}
	defer itemsVariant.Clear()

	items := itemsVariant.ToIDispatch()
	if items == nil {
		return nil, fmt.Errorf("%s items IDispatch is nil", namespace)
	}

	countVariant, err := oleutil.GetProperty(items, "Count")
	if err != nil {
		return nil, fmt.Errorf("read %s item count: %w", namespace, err)
	}
	defer countVariant.Clear()

	count := int(countVariant.Val)
	entries := make([]controlPanelEntry, 0, count)
	for i := 0; i < count; i++ {
		itemVariant, itemErr := oleutil.CallMethod(items, "Item", i)
		if itemErr != nil {
			return nil, fmt.Errorf("read %s item %d: %w", namespace, i, itemErr)
		}
		item := itemVariant.ToIDispatch()
		if item == nil {
			itemVariant.Clear()
			return nil, fmt.Errorf("%s item %d IDispatch is nil", namespace, i)
		}
		name, nameErr := olePropertyString(item, "Name")
		path, pathErr := olePropertyString(item, "Path")
		itemVariant.Clear()
		if err := errors.Join(nameErr, pathErr); err != nil {
			return nil, fmt.Errorf("read %s item %d metadata: %w", namespace, i, err)
		}
		entries = append(entries, controlPanelEntry{Name: name, Path: path})
	}
	return entries, nil
}
