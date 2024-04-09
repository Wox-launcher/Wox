package dto

type ThemeDto struct {
	ThemeId      string
	ThemeName    string
	ThemeAuthor  string
	ThemeUrl     string
	Version      string
	IsInstalled  bool
	IsSystem     bool
	IsUpgradable bool

	AppBackgroundColor              string
	AppPaddingLeft                  int
	AppPaddingTop                   int
	AppPaddingRight                 int
	AppPaddingBottom                int
	ResultContainerPaddingLeft      int
	ResultContainerPaddingTop       int
	ResultContainerPaddingRight     int
	ResultContainerPaddingBottom    int
	ResultItemBorderRadius          int
	ResultItemPaddingLeft           int
	ResultItemPaddingTop            int
	ResultItemPaddingRight          int
	ResultItemPaddingBottom         int
	ResultItemTitleColor            string
	ResultItemSubTitleColor         string
	ResultItemBorderLeft            string
	ResultItemActiveBackgroundColor string
	ResultItemActiveTitleColor      string
	ResultItemActiveSubTitleColor   string
	ResultItemActiveBorderLeft      string
	QueryBoxFontColor               string
	QueryBoxBackgroundColor         string
	QueryBoxBorderRadius            int
	QueryBoxCursorColor             string
	QueryBoxTextSelectionColor      string
	ActionContainerBackgroundColor  string
	ActionContainerHeaderFontColor  string
	ActionContainerPaddingLeft      int
	ActionContainerPaddingTop       int
	ActionContainerPaddingRight     int
	ActionContainerPaddingBottom    int
	ActionItemActiveBackgroundColor string
	ActionItemActiveFontColor       string
	ActionItemFontColor             string
	ActionQueryBoxFontColor         string
	ActionQueryBoxBackgroundColor   string
	ActionQueryBoxBorderRadius      int
	PreviewFontColor                string
	PreviewSplitLineColor           string
	PreviewPropertyTitleColor       string
	PreviewPropertyContentColor     string
	PreviewTextSelectionColor       string
}

type SettingTheme struct {
	ThemeId        string
	ThemeName      string
	ThemeAuthor    string
	ThemeUrl       string
	Version        string
	Description    string
	IsInstalled    bool
	IsSystem       bool
	IsUpgradable   bool
	ScreenshotUrls []string
}
