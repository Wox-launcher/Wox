package dto

type ThemeDto struct {
	ThemeId          string
	ThemeName        string
	ThemeAuthor      string
	ThemeUrl         string
	Version          string
	Description      string
	IsSystem         bool
	IsInstalled      bool
	IsUpgradable     bool
	IsAutoAppearance bool
	DarkThemeId      string
	LightThemeId     string

	AppBackgroundColor                   string
	AppPaddingLeft                       int
	AppPaddingTop                        int
	AppPaddingRight                      int
	AppPaddingBottom                     int
	ResultContainerPaddingLeft           int
	ResultContainerPaddingTop            int
	ResultContainerPaddingRight          int
	ResultContainerPaddingBottom         int
	ResultItemBorderRadius               int
	ResultItemPaddingLeft                int
	ResultItemPaddingTop                 int
	ResultItemPaddingRight               int
	ResultItemPaddingBottom              int
	ResultItemTitleColor                 string
	ResultItemSubTitleColor              string
	ResultItemTailTextColor              string
	ResultItemBorderLeftWidth            int
	ResultItemActiveBackgroundColor      string
	ResultItemActiveTitleColor           string
	ResultItemActiveSubTitleColor        string
	ResultItemActiveBorderLeftWidth      int
	ResultItemActiveBorderLeftColor      string `json:",omitempty"`
	ResultItemActiveTailTextColor        string
	QueryBoxFontColor                    string
	QueryBoxBackgroundColor              string
	QueryBoxBorderRadius                 int
	QueryBoxCursorColor                  string
	QueryBoxTextSelectionBackgroundColor string
	QueryBoxTextSelectionColor           string
	ActionContainerBackgroundColor       string
	ActionContainerBorderColor           string `json:",omitempty"`
	ActionContainerBorderWidth           *int   `json:",omitempty"`
	ActionContainerHeaderFontColor       string
	ActionContainerBorderRadius          *int `json:",omitempty"`
	ActionItemBorderRadius               *int `json:",omitempty"`
	ActionContainerPaddingLeft           int
	ActionContainerPaddingTop            int
	ActionContainerPaddingRight          int
	ActionContainerPaddingBottom         int
	ActionItemActiveBackgroundColor      string
	ActionItemActiveFontColor            string
	ActionItemFontColor                  string
	ActionQueryBoxFontColor              string
	ActionQueryBoxBackgroundColor        string
	ActionQueryBoxBorderRadius           int
	PreviewFontColor                     string
	PreviewSplitLineColor                string
	PreviewPropertyTitleColor            string
	PreviewPropertyContentColor          string
	PreviewTextSelectionColor            string
	ToolbarFontColor                     string
	ToolbarBackgroundColor               string
	ToolbarBorderColor                   string `json:",omitempty"`
	ToolbarBorderWidth                   *int   `json:",omitempty"`
	ToolbarPaddingLeft                   int
	ToolbarPaddingRight                  int
}
