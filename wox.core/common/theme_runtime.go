package common

// Theme is the schema-independent resolved view consumed by the application.
// Wire formats and their defaults belong to their versioned schema files.
type Theme struct {
	PreviewBorderRadius                   *int   `json:",omitempty"`
	ResultItemActiveIndicatorWidth        *int   `json:",omitempty"`
	ResultItemActiveIndicatorInsetLeft    *int   `json:",omitempty"`
	ResultItemActiveIndicatorInsetTop     *int   `json:",omitempty"`
	ResultItemActiveIndicatorInsetBottom  *int   `json:",omitempty"`
	ResultItemActiveIndicatorBorderRadius *int   `json:",omitempty"`
	QueryBoxBorderBottomWidth             *int   `json:",omitempty"`
	ResultItemActiveIndicatorColor        string `json:",omitempty"`
	QueryBoxBorderBottomColor             string `json:",omitempty"`
	PreviewTagBorderRadius                *int   `json:",omitempty"`
	AppBorderWidth                        *int   `json:",omitempty"`
	AppBorderRadius                       *int   `json:",omitempty"`

	ScrollbarWidth        *int `json:",omitempty"`
	ScrollbarHoverWidth   *int `json:",omitempty"`
	ScrollbarBorderRadius *int `json:",omitempty"`

	// source retains the originating schema document for sparse, lossless saves.
	source           any
	SchemaVersion    int
	MinWoxVersion    string `json:",omitempty"`
	ThemeId          string
	ThemeName        string
	ThemeAuthor      string
	ThemeUrl         string
	Version          string
	Description      string
	IsSystem         bool
	IsInstalled      bool
	IsAutoAppearance bool   // Whether to automatically switch theme based on system appearance
	DarkThemeId      string // ID of the dark theme variant
	LightThemeId     string // ID of the light theme variant

	AppBackgroundColor string
	AppPaddingLeft     int
	AppPaddingTop      int
	AppPaddingRight    int
	AppPaddingBottom   int

	QueryBoxFontColor                    string
	QueryBoxBackgroundColor              string
	QueryBoxBorderRadius                 int
	QueryBoxCursorColor                  string
	QueryBoxTextSelectionBackgroundColor string
	QueryBoxTextSelectionColor           string

	ResultContainerPaddingLeft   int
	ResultContainerPaddingTop    int
	ResultContainerPaddingRight  int
	ResultContainerPaddingBottom int

	ResultItemBorderRadius          int
	ResultItemPaddingLeft           int
	ResultItemPaddingTop            int
	ResultItemPaddingRight          int
	ResultItemPaddingBottom         int
	ResultItemTitleColor            string
	ResultItemSubTitleColor         string
	ResultItemTailTextColor         string
	ResultItemBorderLeftWidth       int
	ResultItemActiveBackgroundColor string
	ResultItemActiveTitleColor      string
	ResultItemActiveSubTitleColor   string
	ResultItemActiveBorderLeftWidth int
	ResultItemActiveBorderLeftColor string `json:",omitempty"`
	ResultItemActiveTailTextColor   string

	ActionContainerBackgroundColor string
	ActionContainerBorderColor     string `json:",omitempty"`
	ActionContainerBorderWidth     *int   `json:",omitempty"` // Nil preserves the legacy one-unit border; zero disables it.
	ActionContainerHeaderFontColor string
	ActionContainerBorderRadius    *int `json:",omitempty"`
	ActionContainerPaddingLeft     int
	ActionContainerPaddingTop      int
	ActionContainerPaddingRight    int
	ActionContainerPaddingBottom   int

	ActionItemBorderRadius          *int `json:",omitempty"`
	ActionItemActiveBackgroundColor string
	ActionItemActiveFontColor       string
	ActionItemFontColor             string

	ActionQueryBoxFontColor       string
	ActionQueryBoxBackgroundColor string
	ActionQueryBoxBorderRadius    int

	PreviewFontColor            string
	PreviewSplitLineColor       string
	PreviewPropertyTitleColor   string
	PreviewPropertyContentColor string
	PreviewTextSelectionColor   string

	ToolbarFontColor       string
	ToolbarBackgroundColor string
	ToolbarBorderColor     string `json:",omitempty"`
	ToolbarBorderWidth     *int   `json:",omitempty"`
	ToolbarPaddingLeft     int
	ToolbarPaddingRight    int

	Windows *ThemePlatformOverride `json:"windows,omitempty"`
	MacOS   *ThemePlatformOverride `json:"macos,omitempty"`
	Linux   *ThemePlatformOverride `json:"linux,omitempty"`
}
