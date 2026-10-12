package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"wox/common"
	"wox/plugin/thirdparty/migrate"
	"wox/util"
)

const flowThemeSettingID = "setting:theme"

// themeItems offers the current Flow theme when its colors can be mapped.
func (f flowInstallation) themeItems(ctx context.Context) []migrate.Item {
	theme, ok := f.mappedTheme(ctx)
	if !ok {
		return nil
	}
	return []migrate.Item{flowSettingItem(flowThemeSettingID, "theme", "", theme.ThemeName)}
}

// themeImport maps the selected Flow theme into one new Wox theme.
func (f flowInstallation) themeImport(ctx context.Context, ids []string) ([]migrate.ThemeWrite, []migrate.PluginResult) {
	selected := false
	for _, id := range ids {
		if id == flowThemeSettingID {
			selected = true
			break
		}
	}
	if !selected {
		return nil, nil
	}
	theme, ok := f.mappedTheme(ctx)
	if !ok {
		return nil, []migrate.PluginResult{{ID: flowThemeSettingID, Error: "theme cannot be imported"}}
	}
	return []migrate.ThemeWrite{{ItemIDs: []string{flowThemeSettingID}, Theme: theme}}, nil
}

// mappedTheme reads the active Flow theme file and builds a Wox theme from its solid colors.
func (f flowInstallation) mappedTheme(ctx context.Context) (common.Theme, bool) {
	path, ok := f.themePath()
	if !ok {
		return common.Theme{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow theme %s: %s", path, err.Error()))
		return common.Theme{}, false
	}
	document, err := parseFlowTheme(raw)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow theme %s: %s", path, err.Error()))
		return common.Theme{}, false
	}
	// The list row is the user's item height, not the theme's ResultItemHeight resource.
	document.itemHeight = f.settings.ItemHeightSize
	theme, err := document.woxTheme(strings.TrimSpace(f.settings.Theme), themeDisplayName(path, document.name), f.flowDarkAppearance(document))
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow theme %s: %s", path, err.Error()))
		return common.Theme{}, false
	}
	return theme, true
}

// themePath follows Flow's lookup: bundled themes beside the app, then themes saved in user data.
func (f flowInstallation) themePath() (string, bool) {
	name := strings.TrimSpace(f.settings.Theme)
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", false
	}
	file := name + ".xaml"
	var dirs []string
	if parent := filepath.Dir(f.root); parent != "" && parent != f.root {
		dirs = append(dirs, filepath.Join(parent, "Themes"))
	}
	dirs = append(dirs, flowBundledThemeDirs()...)
	dirs = append(dirs, filepath.Join(f.root, "Themes"))
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		path := filepath.Join(dir, file)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// flowBundledThemeDirs lists Themes folders shipped next to Flow installs, newest app first.
func flowBundledThemeDirs() []string {
	type candidate struct {
		version string
		dir     string
	}
	var found []candidate
	consider := func(appDir string) {
		themes := filepath.Join(appDir, "Themes")
		info, err := os.Stat(themes)
		if err != nil || !info.IsDir() {
			return
		}
		found = append(found, candidate{version: flowAppVersion(appDir), dir: themes})
	}
	var parents []string
	if home := strings.TrimSpace(os.Getenv("USERPROFILE")); home != "" {
		parents = append(parents, filepath.Join(home, "scoop", "apps", "flow-launcher", "current"))
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		parents = append(parents, filepath.Join(local, "FlowLauncher"))
	}
	for _, parent := range parents {
		consider(parent)
		entries, err := os.ReadDir(parent)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), "app-") {
				consider(filepath.Join(parent, entry.Name()))
			}
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		return flowVersionLess(found[j].version, found[i].version)
	})
	dirs := make([]string, len(found))
	for index, item := range found {
		dirs[index] = item.dir
	}
	return dirs
}

const flowShadowExtraMargin = 32

// visibleWindowWidth is the painted launcher width. Flow stores WindowSize as the
// transparent window. With the query-window shadow on, Theme.AddDropShadowEffectToCurrentTheme
// insets WindowBorderStyle by 32 on the left and right so the drop shadow can paint
// inside that window instead of being clipped. Wox AppWidth is the visible window,
// so the gutter is removed. A blur theme on Windows 11 skips the effect and keeps
// the full width, matching Flow's AutoDropShadow.
func (f flowInstallation) visibleWindowWidth() int {
	width := f.settings.WindowSize
	if width <= 0 || !f.dropShadowApplies() {
		return width
	}
	visible := width - flowShadowExtraMargin*2
	if visible < 1 {
		return width
	}
	return visible
}

// dropShadowApplies reports whether Flow will inset the window for its drop shadow.
func (f flowInstallation) dropShadowApplies() bool {
	return flowDropShadowApplies(f.settings.UseDropShadowEffect, f.themeBlurEnabled(), flowBackdropSupported())
}

// flowDropShadowApplies is Flow's AutoDropShadow rule: the setting turns the
// shadow on, and a blur theme on a backdrop-capable Windows 11 skips it.
func flowDropShadowApplies(useDropShadow, themeBlur, backdrop bool) bool {
	if themeBlur && backdrop {
		return false
	}
	return useDropShadow
}

// themeBlurEnabled reads ThemeBlurEnabled. Flow uses it to skip the drop shadow.
func (f flowInstallation) themeBlurEnabled() bool {
	path, ok := f.themePath()
	if !ok {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return flowThemeBlurEnabled(raw)
}

func flowThemeBlurEnabled(raw []byte) bool {
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var recording bool
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch element := token.(type) {
		case xml.StartElement:
			if element.Name.Local == "Boolean" && xmlLocalAttr(element, "Key") == "ThemeBlurEnabled" {
				recording = true
				text.Reset()
			}
		case xml.CharData:
			if recording {
				text.Write(element)
			}
		case xml.EndElement:
			if recording && element.Name.Local == "Boolean" {
				return strings.EqualFold(strings.TrimSpace(text.String()), "true")
			}
		}
	}
}

func themeDisplayName(path, commentName string) string {
	name := strings.TrimSpace(commentName)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	// The imported theme sits next to Wox themes, so the Flow origin stays in the name.
	if strings.HasPrefix(strings.ToLower(name), "flow ") {
		return name
	}
	return "Flow " + name
}

type flowThemeDocument struct {
	name    string
	styles  map[string]map[string]string
	brushes map[string]string
	colors  map[string]string
	strings map[string]string
	// metrics holds ItemRadius and ItemMargin. Missing keys keep Flow's base values of zero.
	metrics map[string]flowMetric
	// margins holds style Margin values, such as the separator inset and the query box offset.
	margins map[string]flowMetric
	// doubles holds ResultItemHeight. Flow still paints the row at ItemHeightSize when that setting is set.
	doubles map[string]float64
	// itemHeight is Settings.ItemHeightSize, the height Flow actually gives each result row.
	itemHeight int
	// dark selects LightBG or DarkBG for themes whose chrome is a DynamicResource.
	dark bool
	// blur is ThemeBlurEnabled. On Windows 11 Flow paints that tint over Desktop Acrylic.
	blur bool
}

// flowMetric is one CornerRadius or Thickness. CornerRadius order is top-left, top-right, bottom-right, bottom-left.
type flowMetric struct {
	topLeft, topRight, bottomRight, bottomLeft float64
	left, top, right, bottom                   float64
}

// woxTheme builds a sparse schema v2 theme. Unmapped chrome keeps Wox defaults derived from the three base colors.
func (d flowThemeDocument) woxTheme(fileName, displayName string, dark bool) (common.Theme, error) {
	d.dark = dark
	background, ok := firstUsableColor(
		colorChoice(d.style("WindowBorderStyle", "Background")),
		colorChoice(d.style("WindowStyle", "Background")),
	)
	// Acrylic clears the border fill and tints Desktop Acrylic with LightBG or
	// DarkBG. The border brush is only the opaque color Flow uses when blur is off.
	if d.blur && flowBackdropSupported() {
		if tint, found := d.schemeBackground(); found {
			background, ok = tint, true
		}
	}
	if !ok {
		// Win11 system themes paint the border with DynamicResource and keep the
		// real fill in LightBG or DarkBG. Flow's ColorizeWindow picks one from SystemBG.
		background, ok = d.schemeBackground()
	}
	if !ok {
		return common.Theme{}, fmt.Errorf("theme has no window background")
	}
	text, ok := firstUsableColor(colorChoice(d.style("QueryBoxStyle", "Foreground")), colorChoice(d.style("ItemTitleStyle", "Foreground")))
	if !ok {
		// Those themes also leave the text as DynamicResource Color05B. A solid
		// text color is required for a Wox theme, so use the matching Win11 ink.
		if d.dark {
			text = "#ffffffff"
		} else {
			text = "#000000e4"
		}
	}
	accent, ok := firstUsableColor(
		colorChoice(d.style("ItemBulletSelectedStyle", "Background")),
		colorChoice(d.style("PreviewGlyph", "Foreground")),
		colorChoice(d.style("ClockBox", "Foreground")),
	)
	if !ok {
		if color, found := flowSystemAccent(); found {
			accent = color
		} else {
			// Some Flow themes never paint a selection bar. The text color keeps the generated theme valid.
			accent = text
		}
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = displayName
	}
	fields := map[string]any{
		"SchemaVersion":       2,
		"ThemeId":             flowMigratedThemeID(fileName),
		"ThemeName":           displayName,
		"ThemeAuthor":         "Flow Launcher",
		"Version":             "1.0.0",
		"BaseBackgroundColor": background,
		"BaseTextColor":       text,
		"BaseAccentColor":     accent,
	}
	d.assignStyle(fields, "QueryBoxFontColor", "QueryBoxStyle", "Foreground")
	d.assignStyle(fields, "QueryBoxCursorColor", "QueryBoxStyle", "CaretBrush")
	d.assignStyle(fields, "QueryBoxBackgroundColor", "QueryBoxStyle", "Background")
	// Flow's query field stays transparent unless the theme paints one. Wox
	// otherwise fills it with BaseBackgroundColor, and painting that translucent
	// window color again darkens the band.
	if _, ok := fields["QueryBoxBackgroundColor"]; !ok {
		fields["QueryBoxBackgroundColor"] = "#00000000"
	}
	// Flow has no toolbar. Glass uses a 4/255 tint of the window color so the
	// footer still blurs results that scroll underneath, without a solid band.
	// The default 1px toolbar border is the line above that footer.
	fields["ToolbarBackgroundColor"] = flowGlassToolbarColor(background)
	fields["ToolbarBorderWidth"] = 0
	fields["ActionQueryBoxBackgroundColor"] = "#00000000"
	d.assignWindowBorder(fields)
	d.assignQuerySeparator(fields)
	d.assignStyle(fields, "ResultItemTitleColor", "ItemTitleStyle", "Foreground")
	d.assignStyle(fields, "ResultItemSubTitleColor", "ItemSubTitleStyle", "Foreground")
	d.assignStyle(fields, "ResultItemTailTextColor", "ItemHotkeyStyle", "Foreground")
	d.assignStyle(fields, "ResultItemActiveTitleColor", "ItemTitleSelectedStyle", "Foreground")
	d.assignStyle(fields, "ResultItemActiveSubTitleColor", "ItemSubTitleSelectedStyle", "Foreground")
	d.assignStyle(fields, "ResultItemActiveTailTextColor", "ItemHotkeySelectedStyle", "Foreground")
	d.assignLookup(fields, "ResultItemActiveBackgroundColor", "ItemSelectedBackgroundColor")
	d.assignBullet(fields)
	// Flow's base theme is a square, full-bleed result row. Leaving these unset
	// keeps Wox's 8px row radius and 10px window gutter, which is the rounded
	// inset selection on an imported theme such as Cyan Dark. The query box
	// shares this horizontal gutter and still has its own 8px text inset.
	radius := d.itemRadius()
	fields["ResultItemBorderRadius"] = radius
	fields["ActionItemBorderRadius"] = radius
	marginLeft, marginRight := d.itemMargin()
	fields["AppPaddingLeft"] = marginLeft
	fields["AppPaddingRight"] = marginRight
	// Flow leaves the first row against the separator unless a theme adds space
	// there. Omitting this keeps Wox's 8px result inset and opens a band Flow does not have.
	fields["ResultContainerPaddingTop"] = d.resultGapTop()
	d.assignQueryBoxPadding(fields, marginLeft)
	// Flow's clock is the query accessory. Wox otherwise paints Glance with secondary text.
	d.assignGlance(fields)
	d.assignStyle(fields, "PreviewFontColor", "PreviewItemTitleStyle", "Foreground")
	d.assignStyle(fields, "PreviewPropertyContentColor", "PreviewItemTitleStyle", "Foreground")
	d.assignStyle(fields, "PreviewPropertyTitleColor", "PreviewItemSubTitleStyle", "Foreground")
	d.assignStyle(fields, "PreviewBorderColor", "PreviewBorderStyle", "BorderBrush")
	d.assignStyle(fields, "PreviewBackgroundColor", "PreviewBorderStyle", "Background")
	raw, err := json.Marshal(fields)
	if err != nil {
		return common.Theme{}, err
	}
	var theme common.Theme
	if err := json.Unmarshal(raw, &theme); err != nil {
		return common.Theme{}, err
	}
	return theme, nil
}

// flowGlassToolbarColor keeps the window color and Glass's 4/255 toolbar alpha.
func flowGlassToolbarColor(background string) string {
	parsed, ok := common.ParseThemeColor(background)
	if !ok {
		return "#00000004"
	}
	return fmt.Sprintf("#%02X%02X%02X04", parsed.R, parsed.G, parsed.B)
}

// assignWindowBorder maps Flow's window outline. Thickness is kept only when the
// brush resolves, so a system border resource does not turn on an invisible outline.
// A blur theme on Windows 11 keeps the system material: authored chrome disables
// Desktop Acrylic and the same alpha becomes a sharp wallpaper wash.
func (d flowThemeDocument) assignWindowBorder(fields map[string]any) {
	if d.blur && flowBackdropSupported() {
		return
	}
	d.assignStyle(fields, "AppBorderColor", "WindowBorderStyle", "BorderBrush")
	if _, ok := fields["AppBorderColor"]; ok {
		if width, ok := d.styleNumber("WindowBorderStyle", "BorderThickness"); ok {
			fields["AppBorderWidth"] = flowMetricInt(width)
		}
	}
	if radius, ok := d.styleNumber("WindowBorderStyle", "CornerRadius"); ok {
		fields["AppBorderRadius"] = flowMetricInt(radius)
	}
}

// assignQuerySeparator maps the hairline Flow draws under the query box.
func (d flowThemeDocument) assignQuerySeparator(fields map[string]any) {
	color, ok := d.style("SeparatorStyle", "Fill")
	if color, ok = usableFlowColor(color, ok); !ok {
		return
	}
	width := 1.0
	if value, ok := d.styleNumber("SeparatorStyle", "Height"); ok && value > 0 {
		width = value
	}
	fields["QuerySeparatorColor"] = color
	fields["QuerySeparatorThickness"] = flowMetricInt(width)
	// SeparatorStyle.Margin is the line's inset. Zero keeps it across the window.
	if margin, ok := d.margins["SeparatorStyle"]; ok {
		if left := flowMetricInt(margin.left); left > 0 {
			fields["QuerySeparatorPaddingLeft"] = left
		}
		if right := flowMetricInt(margin.right); right > 0 {
			fields["QuerySeparatorPaddingRight"] = right
		}
	}
}

// assignGlance maps Flow's clock color onto the query accessory text and icon.
func (d flowThemeDocument) assignGlance(fields map[string]any) {
	color, ok := d.style("ClockBox", "Foreground")
	if color, ok = usableFlowColor(color, ok); !ok {
		color, ok = d.style("DateBox", "Foreground")
		color, ok = usableFlowColor(color, ok)
	}
	if !ok {
		return
	}
	fields["GlanceFontColor"] = color
	fields["GlanceIconColor"] = color
}

// assignQueryBoxPadding keeps Flow's query text inset. BaseQueryBoxStyle places the
// box 16px from the left unless the theme sets its own margin. Wox's built-in inset is 8px.
func (d flowThemeDocument) assignQueryBoxPadding(fields map[string]any, appPaddingLeft int) {
	inner := flowMetricInt(d.queryBoxMargin().left) - appPaddingLeft
	if inner < 0 {
		inner = 0
	}
	if inner == 8 {
		return
	}
	fields["QueryBoxPaddingLeft"] = inner
}

// queryBoxMargin reads QueryBoxStyle.Margin. An omitted margin inherits BaseQueryBoxStyle.
func (d flowThemeDocument) queryBoxMargin() flowMetric {
	if margin, ok := d.margins["QueryBoxStyle"]; ok {
		return margin
	}
	return flowMetric{left: 16, top: 7, right: 0, bottom: 7}
}

// assignBullet maps the selected result marker. Flow centers a fixed-height bullet in the
// row, so a full-height indicator would paint the leftover space as part of the bar.
func (d flowThemeDocument) assignBullet(fields map[string]any) {
	if _, ok := d.styles["ItemBulletSelectedStyle"]["Background"]; !ok {
		return
	}
	if bullet, ok := d.style("ItemBulletSelectedStyle", "Background"); ok {
		fields["ResultItemActiveIndicatorColor"] = bullet
	}
	width := 4.0
	if value, ok := d.styleNumber("ItemBulletSelectedStyle", "Width"); ok && value > 0 {
		width = value
	}
	fields["ResultItemActiveIndicatorWidth"] = flowMetricInt(width)
	if radius, ok := d.styleNumber("ItemBulletSelectedStyle", "CornerRadius"); ok {
		fields["ResultItemActiveIndicatorBorderRadius"] = flowMetricInt(radius)
	}
	height, ok := d.styleNumber("ItemBulletSelectedStyle", "Height")
	if !ok || height <= 0 {
		return
	}
	row := d.resultRowHeight()
	if row <= height {
		return
	}
	inset := flowMetricInt((row - height) / 2)
	if inset <= 0 {
		return
	}
	fields["ResultItemActiveIndicatorInsetTop"] = inset
	fields["ResultItemActiveIndicatorInsetBottom"] = inset
}

// resultRowHeight is the list row Flow paints. The setting wins over the theme resource.
func (d flowThemeDocument) resultRowHeight() float64 {
	if d.itemHeight > 0 {
		return float64(d.itemHeight)
	}
	if height, ok := d.doubles["ResultItemHeight"]; ok && height > 0 {
		return height
	}
	return 0
}

func (d flowThemeDocument) styleNumber(style, property string) (float64, bool) {
	return parseFlowLength(d.styles[style][property])
}

// itemRadius reads Flow's ItemRadius. A missing key uses the base theme's square row.
func (d flowThemeDocument) itemRadius() int {
	metric, ok := d.metrics["ItemRadius"]
	if !ok {
		return 0
	}
	return flowMetricInt(math.Max(metric.topLeft, metric.bottomLeft))
}

// resultGapTop is the space under Flow's query separator before the first row.
// That is the separator's bottom margin, the list's top margin, and a row's top margin.
func (d flowThemeDocument) resultGapTop() int {
	gap := 0
	if margin, ok := d.margins["SeparatorStyle"]; ok {
		gap += flowMetricInt(margin.bottom)
	}
	if margin, ok := d.metrics["ResultMargin"]; ok {
		gap += flowMetricInt(margin.top)
	}
	if margin, ok := d.metrics["ItemMargin"]; ok {
		gap += flowMetricInt(margin.top)
	}
	return gap
}

// itemMargin reads the horizontal ItemMargin. A missing key uses the base theme's flush row.
func (d flowThemeDocument) itemMargin() (left, right int) {
	metric, ok := d.metrics["ItemMargin"]
	if !ok {
		return 0, 0
	}
	return flowMetricInt(metric.left), flowMetricInt(metric.right)
}

func flowMetricInt(value float64) int {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int(math.Round(value))
}

// parseFlowMetric reads a WPF CornerRadius or Thickness. One number applies to every side.
func parseFlowMetric(value, kind string) (flowMetric, bool) {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	numbers := make([]float64, 0, 4)
	for _, part := range parts {
		number, err := strconv.ParseFloat(part, 64)
		if err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
			return flowMetric{}, false
		}
		numbers = append(numbers, number)
	}
	switch len(numbers) {
	case 1:
		numbers = []float64{numbers[0], numbers[0], numbers[0], numbers[0]}
	case 2:
		numbers = []float64{numbers[0], numbers[1], numbers[0], numbers[1]}
	case 4:
	default:
		return flowMetric{}, false
	}
	if kind == "CornerRadius" {
		return flowMetric{topLeft: numbers[0], topRight: numbers[1], bottomRight: numbers[2], bottomLeft: numbers[3]}, true
	}
	return flowMetric{left: numbers[0], top: numbers[1], right: numbers[2], bottom: numbers[3]}, true
}

// flowDarkAppearance follows Flow's ColorizeWindow: SystemBG wins, then ColorScheme, then the Windows app theme.
func (f flowInstallation) flowDarkAppearance(document flowThemeDocument) bool {
	switch strings.ToLower(strings.TrimSpace(document.strings["SystemBG"])) {
	case "dark":
		return true
	case "light":
		return false
	}
	switch strings.ToLower(strings.TrimSpace(f.settings.ColorScheme)) {
	case "dark":
		return true
	case "light":
		return false
	}
	return flowSystemDark()
}

// schemeBackground is LightBG or DarkBG. Flow paints that color as the window fill for a system theme.
func (d flowThemeDocument) schemeBackground() (string, bool) {
	if d.dark {
		if color, ok := d.lookup("DarkBG"); ok {
			return color, true
		}
		return d.lookup("LightBG")
	}
	if color, ok := d.lookup("LightBG"); ok {
		return color, true
	}
	return d.lookup("DarkBG")
}

func (d flowThemeDocument) assignStyle(fields map[string]any, key, style, property string) {
	color, ok := d.style(style, property)
	d.assignColor(fields, key, color, ok)
}

func (d flowThemeDocument) assignLookup(fields map[string]any, key, name string) {
	color, ok := d.lookup(name)
	d.assignColor(fields, key, color, ok)
}

func (d flowThemeDocument) assignColor(fields map[string]any, key, color string, ok bool) {
	if color, ok = usableFlowColor(color, ok); ok {
		fields[key] = color
	}
}

type flowColorChoice struct {
	color string
	ok    bool
}

func colorChoice(color string, ok bool) flowColorChoice {
	color, ok = usableFlowColor(color, ok)
	return flowColorChoice{color: color, ok: ok}
}

func firstUsableColor(choices ...flowColorChoice) (string, bool) {
	for _, choice := range choices {
		if choice.ok {
			return choice.color, true
		}
	}
	return "", false
}

func usableFlowColor(color string, ok bool) (string, bool) {
	if !ok || color == "" || color == "#00000000" {
		return "", false
	}
	return color, true
}

func (d flowThemeDocument) style(style, property string) (string, bool) {
	value := d.styles[style][property]
	return d.resolve(value)
}

func (d flowThemeDocument) lookup(key string) (string, bool) {
	if value, ok := d.brushes[key]; ok {
		return d.resolve(value)
	}
	return d.resolve(d.colors[key])
}

func (d flowThemeDocument) resolve(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	name, referenced := flowResourceName(value)
	if !referenced {
		return value, true
	}
	if color, ok := d.brushes[name]; ok && color != value {
		return d.resolve(color)
	}
	if color, ok := d.colors[name]; ok && color != value {
		return d.resolve(color)
	}
	if color, ok := flowDynamicColor(name, d.dark); ok {
		return color, true
	}
	return "", false
}

// flowDynamicColor maps the iNKORE keys a Win11 Flow theme references instead of a solid color.
func flowDynamicColor(name string, dark bool) (string, bool) {
	switch name {
	case "BasicSystemAccentColor":
		// Flow's dark dictionary binds this brush to SystemAccentColorLight1.
		// The light dictionary uses the base accent.
		if dark {
			if color, ok := flowSystemAccentLight1(); ok {
				return color, true
			}
		}
		return flowSystemAccent()
	case "SystemAccentColorLight1", "SystemAccentColorLight1Brush":
		if color, ok := flowSystemAccentLight1(); ok {
			return color, true
		}
		return flowSystemAccent()
	case "SystemAccentColor":
		return flowSystemAccent()
	case "ItemSelectedBackgroundColorBrush":
		// Win11 does not author a solid selection fill. The dark dictionary is
		// #198F8F8F and the light dictionary is #7EFFFFFF. Leaving this unset
		// makes Wox paint the accent at 18%, so the whole row turns blue.
		if dark {
			return "#8f8f8f19", true
		}
		return "#ffffff7e", true
	case "Color05B", "Color04B":
		if dark {
			return "#ffffffff", true
		}
		return "#000000e4", true
	case "SeparatorForeground":
		// Win11's query separator. The dark dictionary is #24FFFFFF and the light
		// dictionary is #20000000.
		if dark {
			return "#ffffff24", true
		}
		return "#00000020", true
	default:
		return "", false
	}
}

// flowMigratedThemeID is stable for one Flow theme name so a later import updates the same Wox theme.
func flowMigratedThemeID(name string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("wox.flow.theme:"+strings.ToLower(strings.TrimSpace(name)))).String()
}

// parseFlowTheme reads the solid colors a Flow theme uses for the launcher window.
// Control templates, images, and gradients stay behind because Wox themes have no place for them.
func parseFlowTheme(raw []byte) (flowThemeDocument, error) {
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	document := flowThemeDocument{
		styles:  map[string]map[string]string{},
		brushes: map[string]string{},
		colors:  map[string]string{},
		strings: map[string]string{},
		metrics: map[string]flowMetric{},
		margins: map[string]flowMetric{},
		doubles: map[string]float64{},
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var styles []string
	var setterProperty string
	var setterDepth int
	var templateDepth int
	var brushKey string
	var brushColor string
	var brushOpacity string
	var brushText strings.Builder
	var inBrush bool
	var colorKey string
	var colorText strings.Builder
	var inColor bool
	var metricKey string
	var metricKind string
	var metricText strings.Builder
	var inMetric bool
	var stringKey string
	var stringText strings.Builder
	var inString bool
	var doubleKey string
	var doubleText strings.Builder
	var inDouble bool
	var boolKey string
	var boolText strings.Builder
	var inBool bool
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return flowThemeDocument{}, err
		}
		switch element := token.(type) {
		case xml.Comment:
			if document.name == "" {
				document.name = flowThemeCommentName(string(element))
			}
		case xml.StartElement:
			switch element.Name.Local {
			case "Style":
				styles = append(styles, xmlLocalAttr(element, "Key"))
			case "Setter":
				setterDepth++
				if templateDepth == 0 && setterDepth == 1 {
					setterProperty = xmlLocalAttr(element, "Property")
					value := xmlLocalAttr(element, "Value")
					if color, ok := flowSolidColor(value, ""); ok && flowColorProperty(setterProperty) {
						document.putStyle(currentStyle(styles), setterProperty, color)
					} else if _, referenced := flowResourceName(value); referenced && flowColorProperty(setterProperty) {
						document.putStyle(currentStyle(styles), setterProperty, value)
					} else if setterProperty == "Margin" {
						if metric, ok := parseFlowMetric(value, "Thickness"); ok {
							document.margins[currentStyle(styles)] = metric
						}
					} else if setterProperty == "BorderThickness" {
						if metric, ok := parseFlowMetric(value, "Thickness"); ok {
							width := math.Max(math.Max(metric.left, metric.right), math.Max(metric.top, metric.bottom))
							document.putStyle(currentStyle(styles), setterProperty, strconv.FormatFloat(width, 'f', -1, 64))
						}
					} else if flowLengthProperty(setterProperty) {
						if _, ok := parseFlowLength(value); ok {
							document.putStyle(currentStyle(styles), setterProperty, value)
						}
					}
				}
			case "ControlTemplate":
				templateDepth++
			case "SolidColorBrush":
				if templateDepth > 0 {
					break
				}
				inBrush = true
				brushKey = xmlLocalAttr(element, "Key")
				brushColor = xmlLocalAttr(element, "Color")
				brushOpacity = xmlLocalAttr(element, "Opacity")
				brushText.Reset()
			case "Color":
				if templateDepth > 0 {
					break
				}
				inColor = true
				colorKey = xmlLocalAttr(element, "Key")
				colorText.Reset()
			case "CornerRadius", "Thickness":
				if templateDepth > 0 {
					break
				}
				inMetric = true
				metricKind = element.Name.Local
				metricKey = xmlLocalAttr(element, "Key")
				metricText.Reset()
			case "String":
				if templateDepth > 0 {
					break
				}
				inString = true
				stringKey = xmlLocalAttr(element, "Key")
				stringText.Reset()
			case "Double":
				if templateDepth > 0 {
					break
				}
				inDouble = true
				doubleKey = xmlLocalAttr(element, "Key")
				doubleText.Reset()
			case "Boolean":
				if templateDepth > 0 {
					break
				}
				inBool = true
				boolKey = xmlLocalAttr(element, "Key")
				boolText.Reset()
			}
		case xml.CharData:
			if inBrush {
				brushText.Write(element)
			}
			if inColor {
				colorText.Write(element)
			}
			if inMetric {
				metricText.Write(element)
			}
			if inString {
				stringText.Write(element)
			}
			if inDouble {
				doubleText.Write(element)
			}
			if inBool {
				boolText.Write(element)
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "Style":
				if len(styles) > 0 {
					styles = styles[:len(styles)-1]
				}
			case "Setter":
				if setterDepth > 0 {
					setterDepth--
				}
				if setterDepth == 0 {
					setterProperty = ""
				}
			case "ControlTemplate":
				if templateDepth > 0 {
					templateDepth--
				}
			case "SolidColorBrush":
				if !inBrush {
					break
				}
				colorSource := brushColor
				if strings.TrimSpace(colorSource) == "" {
					colorSource = brushText.String()
				}
				if color, ok := flowSolidColor(colorSource, brushOpacity); ok {
					if brushKey != "" {
						document.brushes[brushKey] = color
					} else if setterProperty != "" && flowColorProperty(setterProperty) {
						document.putStyle(currentStyle(styles), setterProperty, color)
					}
				} else if name, referenced := flowResourceName(colorSource); referenced {
					if brushKey != "" {
						document.brushes[brushKey] = "{StaticResource " + name + "}"
					} else if setterProperty != "" && flowColorProperty(setterProperty) {
						document.putStyle(currentStyle(styles), setterProperty, "{StaticResource "+name+"}")
					}
				}
				inBrush = false
				brushKey, brushColor, brushOpacity = "", "", ""
			case "Color":
				if inColor {
					if color, ok := flowSolidColor(colorText.String(), ""); ok && colorKey != "" {
						document.colors[colorKey] = color
					}
				}
				inColor = false
				colorKey = ""
			case "CornerRadius", "Thickness":
				if inMetric && (metricKey == "ItemRadius" || metricKey == "ItemMargin" || metricKey == "ResultMargin") && metricKind == element.Name.Local {
					if metric, ok := parseFlowMetric(metricText.String(), metricKind); ok {
						document.metrics[metricKey] = metric
					}
				}
				inMetric = false
				metricKey, metricKind = "", ""
			case "String":
				if inString && stringKey != "" {
					document.strings[stringKey] = strings.TrimSpace(stringText.String())
				}
				inString = false
				stringKey = ""
			case "Double":
				if inDouble && doubleKey == "ResultItemHeight" {
					if height, ok := parseFlowLength(doubleText.String()); ok {
						document.doubles[doubleKey] = height
					}
				}
				inDouble = false
				doubleKey = ""
			case "Boolean":
				if inBool && boolKey == "ThemeBlurEnabled" {
					document.blur = strings.EqualFold(strings.TrimSpace(boolText.String()), "true")
				}
				inBool = false
				boolKey = ""
			}
		}
	}
	return document, nil
}

func (d flowThemeDocument) putStyle(style, property, color string) {
	if style == "" || property == "" || color == "" {
		return
	}
	props := d.styles[style]
	if props == nil {
		props = map[string]string{}
		d.styles[style] = props
	}
	props[property] = color
}

func currentStyle(styles []string) string {
	if len(styles) == 0 {
		return ""
	}
	return styles[len(styles)-1]
}

func flowLengthProperty(property string) bool {
	switch property {
	case "Width", "Height", "CornerRadius":
		return true
	default:
		return false
	}
}

func parseFlowLength(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "{") {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func flowColorProperty(property string) bool {
	switch property {
	case "Background", "Foreground", "BorderBrush", "CaretBrush", "Fill", "Stroke":
		return true
	default:
		return false
	}
}

func flowThemeCommentName(comment string) string {
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "name:") {
			return strings.TrimSpace(line[len("name:"):])
		}
	}
	return ""
}

func flowResourceName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"{StaticResource", "{DynamicResource", "{m:DynamicColor"} {
		if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, "}") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, prefix), "}"))
			return name, name != ""
		}
	}
	return "", false
}

func xmlLocalAttr(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

// flowSolidColor converts a WPF color into Wox #RRGGBBAA. WPF writes alpha first.
func flowSolidColor(value, opacity string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "{") {
		return "", false
	}
	color, ok := flowNamedColor(value)
	if !ok {
		color, ok = flowHexColor(value)
	}
	if !ok {
		return "", false
	}
	opacity = strings.TrimSpace(opacity)
	if opacity == "" {
		return color, true
	}
	factor, err := strconv.ParseFloat(opacity, 64)
	if err != nil || factor < 0 || factor > 1 {
		return color, true
	}
	alpha, err := strconv.ParseUint(color[7:], 16, 8)
	if err != nil {
		return color, true
	}
	scaled := uint8(math.Round(float64(alpha) * factor))
	return fmt.Sprintf("%s%02x", color[:7], scaled), true
}

func flowHexColor(value string) (string, bool) {
	if !strings.HasPrefix(value, "#") {
		return "", false
	}
	hex := value[1:]
	switch len(hex) {
	case 3: // RGB
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2], 'f', 'f'})
	case 4: // ARGB
		hex = string([]byte{hex[1], hex[1], hex[2], hex[2], hex[3], hex[3], hex[0], hex[0]})
	case 6: // RRGGBB
		hex += "ff"
	case 8: // AARRGGBB
		hex = hex[2:] + hex[:2]
	default:
		return "", false
	}
	if _, err := strconv.ParseUint(hex, 16, 32); err != nil {
		return "", false
	}
	return "#" + strings.ToLower(hex), true
}

func flowNamedColor(value string) (string, bool) {
	switch strings.ToLower(value) {
	case "transparent":
		return "#00000000", true
	case "black":
		return "#000000ff", true
	case "white":
		return "#ffffffff", true
	case "gray", "grey":
		return "#808080ff", true
	case "lightgray", "lightgrey":
		return "#d3d3d3ff", true
	case "darkgray", "darkgrey":
		return "#a9a9a9ff", true
	case "dimgray", "dimgrey":
		return "#696969ff", true
	case "silver":
		return "#c0c0c0ff", true
	case "gainsboro":
		return "#dcdcdcff", true
	case "whitesmoke":
		return "#f5f5f5ff", true
	default:
		return "", false
	}
}
