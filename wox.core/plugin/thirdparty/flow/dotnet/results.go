package dotnet

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
)

// dotnetResult is one row from the plugin process.
type dotnetResult struct {
	Title       string         `json:"title"`
	SubTitle    string         `json:"subTitle"`
	IconPath    string         `json:"iconPath"`
	Score       int64          `json:"score"`
	CopyText    string         `json:"copyText"`
	Rounded     bool           `json:"rounded"`
	PreviewFile string         `json:"previewFile"`
	PreviewText string         `json:"previewText"`
	Actions     []dotnetAction `json:"actions"`
}

type dotnetAction struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// dotnetMessage is a reply or an event from the plugin process.
type dotnetMessage struct {
	ID        string         `json:"id"`
	OK        bool           `json:"ok"`
	Error     string         `json:"error"`
	Event     string         `json:"event"`
	Query     string         `json:"query"`
	Title     string         `json:"title"`
	Subtitle  string         `json:"subtitle"`
	Text      string         `json:"text"`
	Path      string         `json:"path"`
	Directory string         `json:"directory"`
	File      string         `json:"file"`
	Command   string         `json:"command"`
	Program   string         `json:"program"`
	Results   []dotnetResult `json:"results"`
	// Epoch is the launcher-visibility generation a hide event belongs to.
	Epoch int64 `json:"epoch"`
	// Preview is set when the event line is read while its query call is still waiting.
	Preview bool `json:"-"`
	// Generation is the query call that was in flight when the event was read.
	Generation int64 `json:"-"`
}

// dotnetQueryResults turns plugin rows into launcher results.
// Action delegates report whether to hide only after they run, so the row
// stays open and the process sends hide when the delegate returns true.
func dotnetQueryResults(directory string, fallbackIcon common.WoxImage, results []dotnetResult, session *dotnetSession) []plugin.QueryResult {
	rows := make([]plugin.QueryResult, 0, len(results))
	for _, result := range results {
		icon := dotnetImage(directory, result.IconPath)
		if icon.IsEmpty() {
			icon = fallbackIcon
		}
		row := plugin.QueryResult{
			Title:             result.Title,
			SubTitle:          result.SubTitle,
			Icon:              icon,
			IconShowContainer: result.Rounded,
			Score:             result.Score,
		}
		if result.PreviewFile != "" {
			row.Preview = plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeFile,
				PreviewData: dotnetAssetPath(directory, result.PreviewFile),
			}
		} else if result.PreviewText != "" {
			row.Preview = plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeText,
				PreviewData: result.PreviewText,
			}
		}
		for index, action := range result.Actions {
			actionID := action.ID
			name := strings.TrimSpace(action.Name)
			if name == "" {
				name = "Run"
			}
			row.Actions = append(row.Actions, plugin.QueryResultAction{
				Name:                   name,
				Type:                   plugin.QueryResultActionTypeExecute,
				Icon:                   icons.Get(icons.ActionExecute),
				IsDefault:              index == 0,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, _ plugin.ActionContext) {
					if session == nil {
						return
					}
					if err := session.Action(ctx, actionID); err != nil {
						util.GetLogger().Error(ctx, "[flow-dotnet:"+session.launch.Name+"] action: "+err.Error())
					}
				},
			})
		}
		if text := strings.TrimSpace(result.CopyText); text != "" {
			copied := text
			row.Actions = append(row.Actions, plugin.QueryResultAction{
				Name: "Copy",
				Type: plugin.QueryResultActionTypeExecute,
				Icon: icons.Get(icons.ActionCopy),
				Action: func(ctx context.Context, _ plugin.ActionContext) {
					if session == nil || session.launch.Bridge == nil {
						return
					}
					session.launch.Bridge.CopyText(ctx, copied)
				},
			})
		}
		rows = append(rows, row)
	}
	return rows
}

// shownDotNetRow is one row already returned for the current query.
// Later result lists reuse its id so UpdateResult can change that visible row.
type shownDotNetRow struct {
	ID       string
	Title    string
	SubTitle string
}

// assignResultIDs gives each returned row a stable id before the query runner publishes it.
// PolishResult keeps an id that is already set, and a later list updates that same row.
func assignResultIDs(rows []plugin.QueryResult) []shownDotNetRow {
	shown := make([]shownDotNetRow, len(rows))
	for index := range rows {
		if rows[index].Id == "" {
			rows[index].Id = uuid.NewString()
		}
		shown[index] = shownDotNetRow{ID: rows[index].Id, Title: rows[index].Title, SubTitle: rows[index].SubTitle}
	}
	return shown
}

// splitResultUpdate pairs a new list with rows already returned for this query.
// The same title and subtitle keep their id. Remaining rows pair in order, so a
// title change still updates the visible row. Rows past the previous list are new.
// Rows that disappeared stay in next: UpdateResult cannot remove a visible row.
func splitResultUpdate(shown []shownDotNetRow, rows []plugin.QueryResult) (updates []plugin.UpdatableResult, extra []plugin.QueryResult, next []shownDotNetRow) {
	used := make([]bool, len(shown))
	matched := make([]int, len(rows))
	for index := range matched {
		matched[index] = -1
	}
	for index, row := range rows {
		for previous := range shown {
			if used[previous] || shown[previous].Title != row.Title || shown[previous].SubTitle != row.SubTitle {
				continue
			}
			used[previous] = true
			matched[index] = previous
			break
		}
	}
	var unmatchedRows []int
	var unmatchedShown []int
	for index, previous := range matched {
		if previous < 0 {
			unmatchedRows = append(unmatchedRows, index)
		}
	}
	for index := range shown {
		if !used[index] {
			unmatchedShown = append(unmatchedShown, index)
		}
	}
	pairCount := len(unmatchedRows)
	if len(unmatchedShown) < pairCount {
		pairCount = len(unmatchedShown)
	}
	for index := 0; index < pairCount; index++ {
		matched[unmatchedRows[index]] = unmatchedShown[index]
		used[unmatchedShown[index]] = true
	}

	next = make([]shownDotNetRow, 0, len(rows)+len(shown))
	for index, row := range rows {
		if previous := matched[index]; previous >= 0 {
			id := shown[previous].ID
			updates = append(updates, updatableFromRow(id, row))
			next = append(next, shownDotNetRow{ID: id, Title: row.Title, SubTitle: row.SubTitle})
			continue
		}
		if row.Id == "" {
			row.Id = uuid.NewString()
		}
		extra = append(extra, row)
		next = append(next, shownDotNetRow{ID: row.Id, Title: row.Title, SubTitle: row.SubTitle})
	}
	for index := range shown {
		if !used[index] {
			next = append(next, shown[index])
		}
	}
	return updates, extra, next
}

// updatableFromRow copies the fields UpdateResult knows how to change.
func updatableFromRow(id string, row plugin.QueryResult) plugin.UpdatableResult {
	title := row.Title
	subtitle := row.SubTitle
	icon := row.Icon
	preview := row.Preview
	actions := make([]plugin.QueryResultAction, len(row.Actions))
	copy(actions, row.Actions)
	return plugin.UpdatableResult{
		Id:       id,
		Title:    &title,
		SubTitle: &subtitle,
		Icon:     &icon,
		Preview:  &preview,
		Actions:  &actions,
	}
}

func dotnetAssetPath(directory, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(directory, filepath.FromSlash(value))
}

func dotnetImage(directory, value string) common.WoxImage {
	resolved := dotnetAssetPath(directory, value)
	if resolved == "" {
		return common.WoxImage{}
	}
	if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
		return common.NewWoxImageUrl(resolved)
	}
	return common.NewWoxImageAbsolutePath(resolved)
}
