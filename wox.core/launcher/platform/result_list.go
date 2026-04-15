package platform

const defaultResultListItemHeight = 56

type ResultListItem struct {
	QueryID  string
	ResultID string
	ActionID string
	Title    string
	Subtitle string
	IsGroup  bool
	Preview  PreviewContent
}

type ResultListState struct {
	Visible       bool
	Frame         Rect
	Items         []ResultListItem
	SelectedIndex int
	RowHeight     int
}

func (s ResultListState) ItemHeight() int {
	if s.RowHeight > 0 {
		return s.RowHeight
	}
	return defaultResultListItemHeight
}
