package platform

const defaultResultListItemHeight = 56

type ResultListItem struct {
	QueryID  string
	ResultID string
	ActionID string
	Title    string
	Subtitle string
	IsGroup  bool
}

type ResultListState struct {
	Visible       bool
	Frame         Rect
	Items         []ResultListItem
	SelectedIndex int
}

func (s ResultListState) ItemHeight() int {
	return defaultResultListItemHeight
}
