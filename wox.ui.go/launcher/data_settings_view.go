package launcher

import (
	"fmt"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildDataSettingsPage keeps backup, storage, and log operations on core while rendering one portable management surface.
func (a *App) buildDataSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-72)
	note := snapshot.note
	noteColor := snapshot.palette.resultSubtitle
	if snapshot.dataLoading {
		note = "Loading storage and backups…"
	} else if snapshot.dataError != "" {
		note = snapshot.dataError
		noteColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	} else if note == "" {
		note = "Backup creation and restore run in Wox core; this page remains responsive while they finish."
	}
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: 36, Top: 26, Right: 36, Bottom: 18},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: contentWidth, Height: 58, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Data & backup", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
				woxwidget.Text{Value: "Manage local storage, backups, and diagnostic logs", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
			}}},
			a.buildDataStorageCard(snapshot, contentWidth),
			a.buildDataBackupCard(snapshot, contentWidth),
			a.buildDataLogCard(snapshot, contentWidth),
			woxwidget.Container{Width: contentWidth, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.TextBlock{
				Value: note, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 10}, Color: noteColor,
			}},
		}},
	}
}

func (a *App) buildDataStorageCard(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	path := snapshot.dataLocation
	if snapshot.dataPendingLocation != "" {
		path = snapshot.dataPendingLocation
	}
	buttonChildren := []woxwidget.Widget{}
	if snapshot.dataPendingLocation == "" {
		buttonChildren = append(buttonChildren,
			a.buildFormTableButton("data-location-open", "Open", 76, snapshot.dataBusy == "", false, func() { a.openDataPath(snapshot.dataLocation) }, snapshot.palette),
			a.buildFormTableButton("data-location-change", "Change", 86, snapshot.dataBusy == "", true, a.chooseDataLocation, snapshot.palette),
		)
	} else {
		buttonChildren = append(buttonChildren,
			a.buildFormTableButton("data-location-cancel", "Cancel", 76, snapshot.dataBusy == "", false, a.cancelDataLocationChange, snapshot.palette),
			a.buildFormTableButton("data-location-confirm", "Move data", 96, snapshot.dataBusy == "", true, a.confirmDataLocationChange, snapshot.palette),
		)
	}
	labelWidth := max(float32(180), width-250)
	return woxwidget.Container{Width: width, Height: 82, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 14, Bottom: 10}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 60, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Storage location", Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.TextBlock{Value: path, Width: labelWidth, Height: 38, MaxLines: 2, Style: woxui.TextStyle{Size: 10}, LineHeight: 16, Color: snapshot.palette.resultSubtitle},
			}}},
			woxwidget.Container{Width: max(float32(0), width-labelWidth-36), Height: 48, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttonChildren}},
		},
	}}
}

func (a *App) buildDataBackupCard(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	autoLabel := "Auto backup: Off"
	if snapshot.data.EnableAutoBackup {
		autoLabel = "Auto backup: On"
	}
	busy := snapshot.dataBusy != ""
	header := woxwidget.Container{Width: width - 28, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(120), width-28-104-116-104-32), Height: 38, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{
			Value: "Backups", Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle,
		}},
		a.buildFormTableButton("data-auto-backup", autoLabel, 116, !snapshot.saving && !busy, false, a.toggleDataAutoBackup, snapshot.palette),
		a.buildFormTableButton("data-backup-folder", "Open folder", 104, !busy, false, a.openDataBackupFolder, snapshot.palette),
		a.buildFormTableButton("data-backup-now", dataBackupNowLabel(snapshot.dataBusy), 104, !busy, true, a.createDataBackup, snapshot.palette),
	}}}
	return woxwidget.Container{Width: width, Height: 272, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{header, a.buildDataBackupList(snapshot, width-28, 194)},
	}}
}

func dataBackupNowLabel(busy string) string {
	if busy == "backup" {
		return "Backing up…"
	}
	return "Back up now"
}

func (a *App) buildDataBackupList(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	a.setDataBackupViewport(height)
	if len(snapshot.dataBackups) == 0 {
		message := "No backups yet."
		if snapshot.dataLoading {
			message = "Loading backups…"
		}
		return woxwidget.Container{Width: width, Height: height, Radius: 8, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 14, Top: 18}, Child: woxwidget.Text{
			Value: message, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.dataBackups))
	for index, backup := range snapshot.dataBackups {
		index := index
		backup := backup
		restoreLabel := "Restore"
		if snapshot.dataRestoreArmed == backup.ID {
			restoreLabel = "Confirm restore"
		}
		rowColor := snapshot.palette.toolbarBackground
		if index%2 == 1 {
			rowColor = snapshot.palette.background
		}
		pathWidth := max(float32(120), width-170-72-114-68-32)
		rows = append(rows, woxwidget.Container{Width: width, Height: dataBackupRowHeight, Color: rowColor, Padding: woxwidget.Insets{Left: 12, Top: 7, Right: 8}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Container{Width: 162, Height: 32, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Text{Value: time.UnixMilli(backup.Timestamp).Format("2006-01-02 15:04:05"), Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultTitle}},
				woxwidget.Container{Width: 64, Height: 32, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Text{Value: strings.ToUpper(backup.Type), Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle}},
				woxwidget.Container{Width: pathWidth, Height: 32, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{Value: backup.Path, Width: pathWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 9}, Color: snapshot.palette.resultSubtitle}},
				a.buildFormTableButton(fmt.Sprintf("data-backup-restore-%d", index), restoreLabel, 106, snapshot.dataBusy == "", false, func() { a.restoreDataBackup(backup.ID) }, snapshot.palette),
				a.buildFormTableButton(fmt.Sprintf("data-backup-open-%d", index), "Open", 60, snapshot.dataBusy == "", false, func() { a.openDataPath(backup.Path) }, snapshot.palette),
			},
		}})
	}
	return woxwidget.Gesture{ID: "data-backup-scroll", OnScroll: func(delta woxui.Point) { a.scrollDataBackups(-delta.Y) }, Child: woxwidget.ScrollView{
		Width: width, Height: height, ContentHeight: max(height, float32(len(rows))*dataBackupRowHeight), Offset: snapshot.dataListScroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
}

func (a *App) buildDataLogCard(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	clearLabel := "Clear logs"
	if snapshot.dataClearLogsArmed {
		clearLabel = "Confirm clear"
	}
	level := strings.ToUpper(snapshot.data.LogLevel)
	if level != "DEBUG" {
		level = "INFO"
	}
	busy := snapshot.dataBusy != ""
	return woxwidget.Container{Width: width, Height: 72, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 14}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(160), width-112-98-98-48), Height: 46, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Diagnostic logs", Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.Text{Value: "Log level and local history", Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
			}}},
			a.buildFormTableButton("data-log-level", "Level: "+level, 104, !snapshot.saving && !busy, false, a.cycleDataLogLevel, snapshot.palette),
			a.buildFormTableButton("data-log-open", "Open log", 90, !busy, false, a.openDataLog, snapshot.palette),
			a.buildFormTableButton("data-log-clear", clearLabel, 90, !busy, false, a.clearDataLogs, snapshot.palette),
		},
	}}
}
