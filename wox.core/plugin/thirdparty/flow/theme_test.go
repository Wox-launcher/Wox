package flow

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"wox/plugin/thirdparty/migrate"
)

const flowCyanTheme = `
<!--
    Name: Cyan Dark
-->
<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="Background" Value="#0f1f26" />
        <Setter Property="BorderBrush" Value="#1e292f" />
        <Setter Property="CornerRadius" Value="6" />
    </Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox">
        <Setter Property="CaretBrush" Value="#336766" />
        <Setter Property="Foreground" Value="#e7e9eb" />
    </Style>
    <Style x:Key="ItemTitleStyle" TargetType="TextBlock">
        <Setter Property="Foreground" Value="#eff2f2" />
    </Style>
    <Style x:Key="ItemSubTitleStyle" TargetType="TextBlock">
        <Setter Property="Foreground" Value="#768084" />
    </Style>
    <Style x:Key="ItemBulletSelectedStyle" TargetType="Border">
        <Setter Property="Background" Value="#58c2b4" />
    </Style>
    <SolidColorBrush x:Key="ItemSelectedBackgroundColor">#1e292f</SolidColorBrush>
    <CornerRadius x:Key="ItemRadius">0</CornerRadius>
    <Thickness x:Key="ItemMargin">0</Thickness>
</ResourceDictionary>
`

func TestFlowThemeMapsColorsAndKeepsTheSameID(t *testing.T) {
	isolateFlowMigrationHome(t)
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Settings", "Settings.json"), `{"Theme":"Cyan Dark"}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Themes", "Cyan Dark.xaml"), flowCyanTheme)

	installation, err := (flowMigrationSource{}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	categories, err := installation.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var themeItem migrate.Item
	found := false
	for _, category := range categories {
		if category.ID != migrate.CategoryThemes {
			continue
		}
		if len(category.Items) != 1 {
			t.Fatalf("theme items %#v", category.Items)
		}
		themeItem = category.Items[0]
		found = true
	}
	if !found || themeItem.ID != flowThemeSettingID || themeItem.Detail != "Flow Cyan Dark" || !themeItem.Selectable {
		t.Fatalf("theme item %#v", themeItem)
	}

	first, err := installation.Import(context.Background(), []string{flowThemeSettingID})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Themes) != 1 || len(first.Plugins) != 0 {
		t.Fatalf("import %#v", first)
	}
	theme := first.Themes[0].Theme
	if theme.ThemeName != "Flow Cyan Dark" || theme.ThemeAuthor != "Flow Launcher" || !theme.HasAuthoredStyles() {
		t.Fatalf("theme identity %#v authored %v", theme.ThemeName, theme.HasAuthoredStyles())
	}
	if !strings.EqualFold(theme.AppBackgroundColor, "#0F1F26FF") || !strings.EqualFold(theme.QueryBoxFontColor, "#E7E9EBFF") {
		t.Fatalf("colors background %s text %s", theme.AppBackgroundColor, theme.QueryBoxFontColor)
	}
	if !strings.EqualFold(theme.QueryBoxBackgroundColor, "#00000000") || !strings.EqualFold(theme.ToolbarBackgroundColor, "#0F1F2604") || theme.ToolbarBorderWidth == nil || *theme.ToolbarBorderWidth != 0 {
		t.Fatalf("chrome background query %s toolbar %s border %#v", theme.QueryBoxBackgroundColor, theme.ToolbarBackgroundColor, theme.ToolbarBorderWidth)
	}
	if theme.QueryBoxPaddingLeft == nil || *theme.QueryBoxPaddingLeft != 16 {
		t.Fatalf("query inset %#v, want the inherited 16px box margin", theme.QueryBoxPaddingLeft)
	}
	if !strings.EqualFold(theme.ResultItemActiveBackgroundColor, "#1E292FFF") || !strings.EqualFold(theme.ResultItemActiveIndicatorColor, "#58C2B4FF") {
		t.Fatalf("selection background %s indicator %s", theme.ResultItemActiveBackgroundColor, theme.ResultItemActiveIndicatorColor)
	}
	if theme.ResultItemActiveIndicatorWidth == nil || *theme.ResultItemActiveIndicatorWidth != 4 {
		t.Fatalf("indicator width %#v", theme.ResultItemActiveIndicatorWidth)
	}
	if theme.ResultItemBorderRadius != 0 || theme.AppPaddingLeft != 0 || theme.AppPaddingRight != 0 || theme.ResultContainerPaddingTop != 0 {
		t.Fatalf("result geometry radius %d padding %d,%d gap %d", theme.ResultItemBorderRadius, theme.AppPaddingLeft, theme.AppPaddingRight, theme.ResultContainerPaddingTop)
	}
	if theme.ActionItemBorderRadius == nil || *theme.ActionItemBorderRadius != 0 {
		t.Fatalf("action radius %#v", theme.ActionItemBorderRadius)
	}
	saved, err := json.Marshal(theme)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"ResultItemBorderRadius":0`, `"AppPaddingLeft":0`, `"AppPaddingRight":0`, `"ActionItemBorderRadius":0`, `"ResultContainerPaddingTop":0`} {
		if !strings.Contains(string(saved), key) {
			t.Fatalf("saved theme dropped %s: %s", key, saved)
		}
	}
	second, err := installation.Import(context.Background(), []string{flowThemeSettingID})
	if err != nil {
		t.Fatal(err)
	}
	if second.Themes[0].Theme.ThemeId != theme.ThemeId {
		t.Fatalf("theme id changed from %s to %s", theme.ThemeId, second.Themes[0].Theme.ThemeId)
	}
}

func TestFlowBundledThemeWinsOverTheUserCopy(t *testing.T) {
	isolateFlowMigrationHome(t)
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Settings", "Settings.json"), `{"Theme":"Cyan Dark"}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Themes", "Cyan Dark.xaml"), strings.Replace(flowCyanTheme, "#0f1f26", "#111111", 1))
	writeMigrationFile(t, filepath.Join(root, "Themes", "Cyan Dark.xaml"), flowCyanTheme)

	installation, err := (flowMigrationSource{}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := installation.Import(context.Background(), []string{flowThemeSettingID})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Themes) != 1 || !strings.EqualFold(result.Themes[0].Theme.AppBackgroundColor, "#0F1F26FF") {
		t.Fatalf("bundled theme was not used: %#v", result.Themes)
	}
}

func TestFlowThemeReadsOpacityAndWPFAlpha(t *testing.T) {
	document, err := parseFlowTheme([]byte(`
<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="Background">
            <Setter.Value>
                <SolidColorBrush Opacity="0.5" Color="Black" />
            </Setter.Value>
        </Setter>
    </Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox">
        <Setter Property="Foreground" Value="#FFFFFFFF" />
    </Style>
    <SolidColorBrush x:Key="ItemSelectedBackgroundColor">#19c9c9c9</SolidColorBrush>
    <Style x:Key="WindowStyle" TargetType="Window">
        <Setter Property="Background" Value="{StaticResource DarkBG}" />
    </Style>
    <Color x:Key="DarkBG">#B6000000</Color>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	background, ok := document.style("WindowBorderStyle", "Background")
	if !ok || background != "#00000080" {
		t.Fatalf("opacity background = %s ok=%v", background, ok)
	}
	selected, ok := document.lookup("ItemSelectedBackgroundColor")
	if !ok || selected != "#c9c9c919" {
		t.Fatalf("selected = %s ok=%v", selected, ok)
	}
	referenced, ok := document.style("WindowStyle", "Background")
	if !ok || referenced != "#000000b6" {
		t.Fatalf("resource background = %s ok=%v", referenced, ok)
	}
}

func TestFlowThemeKeepsAuthoredRowRadiusAndMargin(t *testing.T) {
	document, err := parseFlowTheme([]byte(`
<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="Background" Value="#111111" />
        <Setter Property="CornerRadius" Value="6" />
    </Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox">
        <Setter Property="Foreground" Value="#FFFFFF" />
    </Style>
    <CornerRadius x:Key="ItemRadius">6</CornerRadius>
    <Thickness x:Key="ItemMargin">4 0 10 0</Thickness>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	theme, err := document.woxTheme("Slim", "Slim", false)
	if err != nil {
		t.Fatal(err)
	}
	if theme.ResultItemBorderRadius != 6 || theme.AppPaddingLeft != 4 || theme.AppPaddingRight != 10 {
		t.Fatalf("geometry radius %d padding %d,%d", theme.ResultItemBorderRadius, theme.AppPaddingLeft, theme.AppPaddingRight)
	}
	if theme.ActionItemBorderRadius == nil || *theme.ActionItemBorderRadius != 6 {
		t.Fatalf("action radius %#v", theme.ActionItemBorderRadius)
	}

	inherited, err := parseFlowTheme([]byte(`
<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="Background" Value="#111111" />
    </Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox">
        <Setter Property="Foreground" Value="#FFFFFF" />
    </Style>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	baseTheme, err := inherited.woxTheme("Base", "Base", false)
	if err != nil {
		t.Fatal(err)
	}
	if baseTheme.ResultItemBorderRadius != 0 || baseTheme.AppPaddingLeft != 0 || baseTheme.AppPaddingRight != 0 {
		t.Fatalf("inherited geometry radius %d padding %d,%d", baseTheme.ResultItemBorderRadius, baseTheme.AppPaddingLeft, baseTheme.AppPaddingRight)
	}
}

const flowWin11Theme = `
<!--
    Name: Windows 11
-->
<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" xmlns:system="clr-namespace:System;assembly=mscorlib">
    <system:Boolean x:Key="ThemeBlurEnabled">True</system:Boolean>
    <system:String x:Key="SystemBG">Light</system:String>
    <Color x:Key="LightBG">#CEFAFAFA</Color>
    <Color x:Key="DarkBG">#D6202020</Color>
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="Background" Value="{DynamicResource Color01B}" />
    </Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox">
        <Setter Property="Foreground" Value="{DynamicResource Color05B}" />
    </Style>
    <Style x:Key="ItemBulletSelectedStyle" TargetType="Border">
        <Setter Property="Width" Value="4" />
        <Setter Property="Height" Value="38" />
        <Setter Property="CornerRadius" Value="2" />
        <Setter Property="Background" Value="{DynamicResource BasicSystemAccentColor}" />
    </Style>
    <SolidColorBrush x:Key="ItemSelectedBackgroundColor" Color="{m:DynamicColor ItemSelectedBackgroundColorBrush}" />
    <Style x:Key="SeparatorStyle" TargetType="Rectangle">
        <Setter Property="Fill" Value="{DynamicResource SeparatorForeground}" />
        <Setter Property="Height" Value="1" />
    </Style>
    <CornerRadius x:Key="ItemRadius">5</CornerRadius>
    <Thickness x:Key="ItemMargin">10 0 10 0</Thickness>
    <system:Double x:Key="ResultItemHeight">58</system:Double>
</ResourceDictionary>
`

func TestFlowSystemThemeStaysVisibleInTheCatalog(t *testing.T) {
	light, err := parseFlowTheme([]byte(flowWin11Theme))
	if err != nil {
		t.Fatal(err)
	}
	if light.strings["SystemBG"] != "Light" {
		t.Fatalf("system background %q", light.strings["SystemBG"])
	}
	theme, err := light.woxTheme("Win11Light", "Windows 11", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(theme.AppBackgroundColor, "#FAFAFACE") || !strings.EqualFold(theme.QueryBoxFontColor, "#000000E4") {
		t.Fatalf("light colors background %s text %s", theme.AppBackgroundColor, theme.QueryBoxFontColor)
	}
	if !strings.EqualFold(theme.QueryBoxBackgroundColor, "#00000000") || !strings.EqualFold(theme.ToolbarBackgroundColor, "#FAFAFA04") || !strings.EqualFold(theme.ActionQueryBoxBackgroundColor, "#00000000") || theme.ToolbarBorderWidth == nil || *theme.ToolbarBorderWidth != 0 {
		t.Fatalf("light chrome query %s toolbar %s action %s border %#v", theme.QueryBoxBackgroundColor, theme.ToolbarBackgroundColor, theme.ActionQueryBoxBackgroundColor, theme.ToolbarBorderWidth)
	}
	if theme.QuerySeparatorThickness == nil || *theme.QuerySeparatorThickness != 1 || !strings.EqualFold(theme.QuerySeparatorColor, "#00000020") || theme.QuerySeparatorPaddingLeft != nil || theme.QuerySeparatorPaddingRight != nil {
		t.Fatalf("light separator thickness %#v color %s padding %#v %#v", theme.QuerySeparatorThickness, theme.QuerySeparatorColor, theme.QuerySeparatorPaddingLeft, theme.QuerySeparatorPaddingRight)
	}
	if theme.QueryBoxPaddingLeft == nil || *theme.QueryBoxPaddingLeft != 6 {
		t.Fatalf("light query inset %#v, want 16px margin minus the 10px item gutter", theme.QueryBoxPaddingLeft)
	}
	if theme.ResultItemBorderRadius != 5 || theme.AppPaddingLeft != 10 {
		t.Fatalf("geometry radius %d padding %d", theme.ResultItemBorderRadius, theme.AppPaddingLeft)
	}
	if !strings.EqualFold(theme.ResultItemActiveBackgroundColor, "#FFFFFF7E") {
		t.Fatalf("light selection %s", theme.ResultItemActiveBackgroundColor)
	}
	if theme.ResultItemActiveIndicatorInsetTop == nil || *theme.ResultItemActiveIndicatorInsetTop != 10 || theme.ResultItemActiveIndicatorInsetBottom == nil || *theme.ResultItemActiveIndicatorInsetBottom != 10 {
		t.Fatalf("indicator insets %#v %#v", theme.ResultItemActiveIndicatorInsetTop, theme.ResultItemActiveIndicatorInsetBottom)
	}
	if theme.ResultItemActiveIndicatorBorderRadius == nil || *theme.ResultItemActiveIndicatorBorderRadius != 2 {
		t.Fatalf("indicator radius %#v", theme.ResultItemActiveIndicatorBorderRadius)
	}

	darkSource := strings.Replace(flowWin11Theme, ">Light<", ">Dark<", 1)
	dark, err := parseFlowTheme([]byte(darkSource))
	if err != nil {
		t.Fatal(err)
	}
	theme, err = dark.woxTheme("Win11Light", "Windows 11", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(theme.AppBackgroundColor, "#202020D6") || !strings.EqualFold(theme.QueryBoxFontColor, "#FFFFFFFF") {
		t.Fatalf("dark colors background %s text %s", theme.AppBackgroundColor, theme.QueryBoxFontColor)
	}
	if !strings.EqualFold(theme.ResultItemActiveBackgroundColor, "#8F8F8F19") {
		t.Fatalf("dark selection %s", theme.ResultItemActiveBackgroundColor)
	}
	if !strings.EqualFold(theme.ToolbarBackgroundColor, "#20202004") {
		t.Fatalf("dark toolbar %s", theme.ToolbarBackgroundColor)
	}
	if !strings.EqualFold(theme.QuerySeparatorColor, "#FFFFFF24") || theme.QuerySeparatorThickness == nil || *theme.QuerySeparatorThickness != 1 {
		t.Fatalf("dark separator thickness %#v color %s", theme.QuerySeparatorThickness, theme.QuerySeparatorColor)
	}
	dark.itemHeight = 42
	shorter, err := dark.woxTheme("Win11Light", "Windows 11", true)
	if err != nil {
		t.Fatal(err)
	}
	if shorter.ResultItemActiveIndicatorInsetTop == nil || *shorter.ResultItemActiveIndicatorInsetTop != 2 {
		t.Fatalf("setting row inset %#v", shorter.ResultItemActiveIndicatorInsetTop)
	}

	isolateFlowMigrationHome(t)
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Settings", "Settings.json"), `{"Theme":"Win11Light","ColorScheme":"Light"}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Themes", "Win11Light.xaml"), flowWin11Theme)
	installation, err := (flowMigrationSource{}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	categories, err := installation.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	themes := categoryByID(t, categories, migrate.CategoryThemes)
	if len(themes.Items) != 1 || themes.Items[0].ID != flowThemeSettingID || themes.Items[0].Detail != "Flow Windows 11" {
		t.Fatalf("theme item %#v", themes.Items)
	}
}

func TestFlowSeparatorMarginInsetsTheLine(t *testing.T) {
	document, err := parseFlowTheme([]byte(`<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border"><Setter Property="Background" Value="#111111" /></Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox"><Setter Property="Margin" Value="4 0 0 0" /><Setter Property="Foreground" Value="#ffffff" /></Style>
    <Style x:Key="SeparatorStyle" TargetType="Rectangle"><Setter Property="Fill" Value="#444444" /><Setter Property="Height" Value="1" /><Setter Property="Margin" Value="12 0 12 8" /></Style>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	theme, err := document.woxTheme("BlurBlack", "Blur Black", true)
	if err != nil {
		t.Fatal(err)
	}
	if theme.QuerySeparatorPaddingLeft == nil || *theme.QuerySeparatorPaddingLeft != 12 || theme.QuerySeparatorPaddingRight == nil || *theme.QuerySeparatorPaddingRight != 12 {
		t.Fatalf("separator padding %#v %#v", theme.QuerySeparatorPaddingLeft, theme.QuerySeparatorPaddingRight)
	}
	if theme.ResultContainerPaddingTop != 8 {
		t.Fatalf("result gap %d, want the separator's 8px bottom margin", theme.ResultContainerPaddingTop)
	}
	if theme.QueryBoxPaddingLeft == nil || *theme.QueryBoxPaddingLeft != 4 {
		t.Fatalf("authored query margin %#v", theme.QueryBoxPaddingLeft)
	}

	listed, err := parseFlowTheme([]byte(`<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border"><Setter Property="Background" Value="#2e2e34" /></Style>
    <Thickness x:Key="ResultMargin">0 5 0 6</Thickness>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	listTheme, err := listed.woxTheme("Discord Dark", "Discord Dark", true)
	if err != nil {
		t.Fatal(err)
	}
	if listTheme.ResultContainerPaddingTop != 5 {
		t.Fatalf("result gap %d, want ResultMargin top", listTheme.ResultContainerPaddingTop)
	}
}

func TestFlowWindowBorderMapsColorWidthAndRadius(t *testing.T) {
	document, err := parseFlowTheme([]byte(`<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="BorderThickness" Value="2" />
        <Setter Property="BorderBrush" Value="#6c7279" />
        <Setter Property="CornerRadius" Value="6" />
        <Setter Property="Background" Value="#303840" />
    </Style>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	theme, err := document.woxTheme("Sublime", "Flow Sublime", true)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := json.Marshal(theme)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"AppBorderColor":"#6c7279ff"`, `"AppBorderWidth":2`, `"AppBorderRadius":6`} {
		if !strings.Contains(string(saved), key) {
			t.Fatalf("saved theme missing %s: %s", key, saved)
		}
	}
	if !theme.UsesCustomWindowChrome() {
		t.Fatal("window border did not select custom chrome")
	}
}

func TestFlowBlurThemeKeepsSystemMaterial(t *testing.T) {
	document, err := parseFlowTheme([]byte(`<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" xmlns:system="clr-namespace:System;assembly=mscorlib">
    <system:Boolean x:Key="ThemeBlurEnabled">True</system:Boolean>
    <Color x:Key="DarkBG">#D6202020</Color>
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="BorderThickness" Value="1" />
        <Setter Property="BorderBrush" Value="{DynamicResource SystemThemeBorder}" />
        <Setter Property="CornerRadius" Value="8" />
        <Setter Property="Background" Value="{DynamicResource SystemControlBackgroundChromeMediumLowBrush}" />
    </Style>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	if !document.blur {
		t.Fatal("ThemeBlurEnabled was not read")
	}
	theme, err := document.woxTheme("Circle System", "Flow Circle", true)
	if err != nil {
		t.Fatal(err)
	}
	if !flowBackdropSupported() {
		if theme.AppBorderRadius == nil || *theme.AppBorderRadius != 8 {
			t.Fatal("blur theme without a system backdrop should keep the painted corner")
		}
		return
	}
	if theme.UsesCustomWindowChrome() || theme.AppBorderRadius != nil || theme.AppBorderWidth != nil {
		t.Fatalf("blur theme enabled custom chrome: radius=%v width=%v", theme.AppBorderRadius, theme.AppBorderWidth)
	}
}

func TestFlowBlurThemeTintsWithSchemeBackground(t *testing.T) {
	document, err := parseFlowTheme([]byte(`<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" xmlns:system="clr-namespace:System;assembly=mscorlib">
    <system:Boolean x:Key="ThemeBlurEnabled">True</system:Boolean>
    <system:String x:Key="SystemBG">Light</system:String>
    <Color x:Key="LightBG">#BFFAFAFA</Color>
    <Style x:Key="WindowBorderStyle" TargetType="Border">
        <Setter Property="Background">
            <Setter.Value>
                <SolidColorBrush Opacity="0.9" Color="White" />
            </Setter.Value>
        </Setter>
        <Setter Property="BorderBrush" Value="#aaaaaa" />
        <Setter Property="BorderThickness" Value="1" />
        <Setter Property="CornerRadius" Value="0" />
    </Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox">
        <Setter Property="Foreground" Value="#FF000000" />
    </Style>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	theme, err := document.woxTheme("BlurWhite", "Flow Blur White", false)
	if err != nil {
		t.Fatal(err)
	}
	if flowBackdropSupported() {
		if !strings.EqualFold(theme.AppBackgroundColor, "#FAFAFABF") {
			t.Fatalf("acrylic tint %s, want LightBG", theme.AppBackgroundColor)
		}
		if theme.UsesCustomWindowChrome() {
			t.Fatal("blur border turned off system material")
		}
		return
	}
	if !strings.EqualFold(theme.AppBackgroundColor, "#FFFFFFE6") {
		t.Fatalf("opaque fill %s, want the border brush", theme.AppBackgroundColor)
	}
}

func TestFlowClockColorBecomesTheGlanceColor(t *testing.T) {
	document, err := parseFlowTheme([]byte(`<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
    <Style x:Key="WindowBorderStyle" TargetType="Border"><Setter Property="Background" Value="#303840" /></Style>
    <Style x:Key="QueryBoxStyle" TargetType="TextBox"><Setter Property="Foreground" Value="#d2d8e5" /></Style>
    <Style x:Key="ClockBox" TargetType="TextBlock"><Setter Property="Foreground" Value="#5bafb0" /></Style>
</ResourceDictionary>`))
	if err != nil {
		t.Fatal(err)
	}
	theme, err := document.woxTheme("Sublime", "Flow Sublime", true)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := json.Marshal(theme)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"GlanceFontColor":"#5bafb0ff"`, `"GlanceIconColor":"#5bafb0ff"`} {
		if !strings.Contains(string(saved), key) {
			t.Fatalf("saved theme missing %s: %s", key, saved)
		}
	}
}
