package system

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"
)

const colorHistorySettingKey = "colorHistory"

var colorPluginIcon = common.PluginColorIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ColorPlugin{})
}

type ColorPlugin struct {
	api       plugin.API
	historyMu sync.Mutex
}

type ColorHistoryItem struct {
	Hex        string `json:"hex"`
	Name       string `json:"name,omitempty"`
	Favorite   bool   `json:"favorite,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	LastSeenAt int64  `json:"lastSeenAt"`
}

type parsedColor struct {
	Hex string
	R   int
	G   int
	B   int
	H   float64
	S   float64
	L   float64
}

func (c *ColorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "5e6e7d7a-6af7-4bf0-8f64-2c4b76a2fb36",
		Name:          "i18n:plugin_color_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_color_plugin_description",
		Icon:          colorPluginIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
			"color",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Macos",
			"Windows",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
	}
}

func (c *ColorPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
}

func (c *ColorPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	search := strings.TrimSpace(query.Search)
	if query.IsGlobalQuery() {
		search = strings.TrimSpace(query.RawQuery)
	}

	parsed, ok := parseHexColor(search)
	if ok {
		// New feature: a complete HEX query is treated as an observation. Upserting
		// keeps query-time history useful without duplicating the same color while
		// the user repeats or refines the query.
		item := c.upsertColorHistory(ctx, parsed.Hex)
		return plugin.NewQueryResponse([]plugin.QueryResult{c.buildColorResult(ctx, parsed, item)})
	}

	if query.IsGlobalQuery() {
		return plugin.QueryResponse{}
	}

	return plugin.NewQueryResponse(c.buildHistoryResults(ctx, search))
}

func (c *ColorPlugin) buildHistoryResults(ctx context.Context, search string) []plugin.QueryResult {
	results := make([]plugin.QueryResult, 0)
	for _, item := range c.filterHistory(ctx, search) {
		parsedHistory, parseOK := parseHexColor(item.Hex)
		if !parseOK {
			continue
		}
		results = append(results, c.buildColorResult(ctx, parsedHistory, item))
	}

	return results
}

func parseHexColor(input string) (parsedColor, bool) {
	raw := strings.TrimSpace(input)
	raw = strings.TrimPrefix(raw, "#")
	if len(raw) != 6 {
		return parsedColor{}, false
	}

	value, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return parsedColor{}, false
	}

	// The first version intentionally accepts only 6-digit opaque HEX. Keeping
	// alpha and short forms out avoids unclear copy/history semantics.
	normalized := fmt.Sprintf("#%06X", value)
	r := int((value >> 16) & 0xFF)
	g := int((value >> 8) & 0xFF)
	b := int(value & 0xFF)
	h, s, l := rgbToHSL(r, g, b)

	return parsedColor{
		Hex: normalized,
		R:   r,
		G:   g,
		B:   b,
		H:   h,
		S:   s,
		L:   l,
	}, true
}

func (c *ColorPlugin) buildColorResult(ctx context.Context, color parsedColor, item ColorHistoryItem) plugin.QueryResult {
	title := color.Hex
	if strings.TrimSpace(item.Name) != "" {
		title = item.Name
	}

	group, groupScore := c.getResultGroup(item)
	complement := deriveHexColor(color, 180)
	analogousLeft := deriveHexColor(color, -30)
	analogousRight := deriveHexColor(color, 30)
	rgb := color.rgbText()
	hsl := color.hslText()

	return plugin.QueryResult{
		Id:         "color-" + strings.TrimPrefix(color.Hex, "#"),
		Title:      title,
		SubTitle:   fmt.Sprintf("%s  %s  %s", color.Hex, rgb, hsl),
		Icon:       color.swatchIcon(),
		Score:      item.LastSeenAt,
		Group:      group,
		GroupScore: groupScore,
		Tails:      c.buildColorTails(ctx, complement, analogousLeft, analogousRight),
		Actions:    c.buildColorActions(ctx, color, item, complement, analogousLeft, analogousRight, rgb, hsl),
	}
}

func (c *ColorPlugin) buildColorTails(ctx context.Context, complement string, analogousLeft string, analogousRight string) []plugin.QueryResultTail {
	return []plugin.QueryResultTail{
		buildColorTailSwatch("complement", complement, fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_color_tail_complement"), complement)),
		buildColorTailSwatch("analogous-left", analogousLeft, fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_color_tail_analogous_single"), analogousLeft)),
		buildColorTailSwatch("analogous-right", analogousRight, fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_color_tail_analogous_single"), analogousRight)),
	}
}

func buildColorTailSwatch(kind string, hex string, tooltip string) plugin.QueryResultTail {
	width := 28.0
	height := 18.0
	// Color relationship tails are visual metadata. SVG swatches are easier to scan
	// than long HEX text chips; tooltips keep each relationship discoverable.
	return plugin.QueryResultTail{
		Type:        plugin.QueryResultTailTypeImage,
		Image:       colorSwatchImage(hex, 7),
		ImageWidth:  &width,
		ImageHeight: &height,
		Tooltip:     tooltip,
		ContextData: common.ContextData{
			"kind": kind,
			"hex":  hex,
		},
	}
}

func (c *ColorPlugin) buildColorActions(ctx context.Context, color parsedColor, item ColorHistoryItem, complement string, analogousLeft string, analogousRight string, rgb string, hsl string) []plugin.QueryResultAction {
	favoriteActionName := "i18n:plugin_color_mark_favorite"
	favoriteActionIcon := common.PinIcon
	nextFavoriteValue := true
	if item.Favorite {
		favoriteActionName = "i18n:plugin_color_cancel_favorite"
		favoriteActionIcon = common.UnpinIcon
		nextFavoriteValue = false
	}

	return []plugin.QueryResultAction{
		c.buildCopyAction("i18n:plugin_color_copy_hex", color.Hex, true),
		c.buildCopyAction("i18n:plugin_color_copy_rgb", rgb, false),
		c.buildCopyAction("i18n:plugin_color_copy_hsl", hsl, false),
		c.buildCopyAction("i18n:plugin_color_copy_complement", complement, false),
		c.buildCopyAction("i18n:plugin_color_copy_analogous", analogousLeft+" "+analogousRight, false),
		{
			Name:                   favoriteActionName,
			Icon:                   favoriteActionIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.updateFavorite(ctx, color.Hex, nextFavoriteValue)
				c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			},
		},
		{
			Name:                   "i18n:plugin_color_name_color",
			Icon:                   common.EditIcon,
			Type:                   plugin.QueryResultActionTypeForm,
			PreventHideAfterAction: true,
			Form: definition.PluginSettingDefinitions{
				{
					Type: definition.PluginSettingDefinitionTypeTextBox,
					Value: &definition.PluginSettingValueTextBox{
						Key:          "name",
						Label:        "i18n:plugin_color_name_color_label",
						DefaultValue: item.Name,
						Tooltip:      "i18n:plugin_color_name_color_hint",
					},
				},
			},
			OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
				c.updateName(ctx, color.Hex, strings.TrimSpace(actionContext.Values["name"]))
				c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			},
		},
		{
			Name:                   "i18n:plugin_color_delete",
			Icon:                   common.TrashIcon,
			PreventHideAfterAction: true,
			Hotkey:                 util.PrimaryHotkey("d"),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.deleteColor(ctx, color.Hex)
				c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: false})
			},
		},
	}
}

func (c *ColorPlugin) buildCopyAction(name string, text string, isDefault bool) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:      name,
		Icon:      common.CopyIcon,
		IsDefault: isDefault,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			if err := clipboard.WriteText(text); err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy color text: %s", err.Error()))
			}
		},
	}
}

func (c *ColorPlugin) filterHistory(ctx context.Context, search string) []ColorHistoryItem {
	history := c.loadHistory(ctx)
	normalizedSearch := strings.ToLower(strings.TrimSpace(search))
	normalizedSearch = strings.TrimPrefix(normalizedSearch, "#")

	filtered := make([]ColorHistoryItem, 0, len(history))
	for _, item := range history {
		if normalizedSearch == "" ||
			strings.Contains(strings.ToLower(strings.TrimPrefix(item.Hex, "#")), normalizedSearch) ||
			strings.Contains(strings.ToLower(item.Name), normalizedSearch) {
			filtered = append(filtered, item)
		}
	}

	sortColorHistory(filtered)
	return filtered
}

func (c *ColorPlugin) upsertColorHistory(ctx context.Context, hex string) ColorHistoryItem {
	// Query execution can overlap while the user types. Lock write paths so the
	// setting-backed JSON history is updated as one read-modify-write operation.
	c.historyMu.Lock()
	defer c.historyMu.Unlock()

	history := c.loadHistory(ctx)
	now := util.GetSystemTimestamp()

	for i := range history {
		if history[i].Hex != hex {
			continue
		}
		history[i].LastSeenAt = now
		if history[i].CreatedAt == 0 {
			history[i].CreatedAt = now
		}
		c.saveHistory(ctx, history)
		return history[i]
	}

	item := ColorHistoryItem{
		Hex:        hex,
		CreatedAt:  now,
		LastSeenAt: now,
	}
	history = append(history, item)
	c.saveHistory(ctx, history)
	return item
}

func (c *ColorPlugin) updateFavorite(ctx context.Context, hex string, favorite bool) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()

	history := c.loadHistory(ctx)
	for i := range history {
		if history[i].Hex == hex {
			history[i].Favorite = favorite
			history[i].LastSeenAt = util.GetSystemTimestamp()
			c.saveHistory(ctx, history)
			return
		}
	}
}

func (c *ColorPlugin) updateName(ctx context.Context, hex string, name string) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()

	history := c.loadHistory(ctx)
	for i := range history {
		if history[i].Hex == hex {
			history[i].Name = name
			history[i].LastSeenAt = util.GetSystemTimestamp()
			c.saveHistory(ctx, history)
			return
		}
	}
}

func (c *ColorPlugin) deleteColor(ctx context.Context, hex string) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()

	history := c.loadHistory(ctx)
	filtered := history[:0]
	for _, item := range history {
		if item.Hex != hex {
			filtered = append(filtered, item)
		}
	}
	c.saveHistory(ctx, filtered)
}

func (c *ColorPlugin) loadHistory(ctx context.Context) []ColorHistoryItem {
	raw := strings.TrimSpace(c.api.GetSetting(ctx, colorHistorySettingKey))
	if raw == "" {
		return []ColorHistoryItem{}
	}

	var history []ColorHistoryItem
	if err := json.Unmarshal([]byte(raw), &history); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal color history: %s", err.Error()))
		return []ColorHistoryItem{}
	}

	return history
}

func (c *ColorPlugin) saveHistory(ctx context.Context, history []ColorHistoryItem) {
	sortColorHistory(history)
	payload, err := json.Marshal(history)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to marshal color history: %s", err.Error()))
		return
	}
	c.api.SaveSetting(ctx, colorHistorySettingKey, string(payload), false)
}

func sortColorHistory(history []ColorHistoryItem) {
	sort.SliceStable(history, func(i, j int) bool {
		if history[i].Favorite != history[j].Favorite {
			return history[i].Favorite
		}
		return history[i].LastSeenAt > history[j].LastSeenAt
	})
}

func (c *ColorPlugin) getResultGroup(item ColorHistoryItem) (string, int64) {
	if item.Favorite {
		return "i18n:plugin_color_group_favorites", 100
	}

	elapsed := util.GetSystemTimestamp() - item.LastSeenAt
	if elapsed < 1000*60*60*24 {
		return "i18n:plugin_color_group_today", 90
	}
	if elapsed < 1000*60*60*24*2 {
		return "i18n:plugin_color_group_yesterday", 80
	}

	return "i18n:plugin_color_group_history", 10
}

func (c parsedColor) swatchIcon() common.WoxImage {
	return colorSwatchImage(c.Hex, 10)
}

func colorSwatchImage(hex string, radius int) common.WoxImage {
	foreground := "#FFFFFF"
	if parsed, ok := parseHexColor(hex); ok && parsed.L > 0.62 {
		foreground = "#111111"
	}

	return common.NewWoxImageSvg(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><rect width="48" height="48" rx="%d" fill="%s"/><path fill="%s" opacity=".95" d="M12 34h24v4H12z"/></svg>`, radius, hex, foreground))
}

func (c parsedColor) rgbText() string {
	return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
}

func (c parsedColor) hslText() string {
	return fmt.Sprintf("hsl(%d, %d%%, %d%%)", int(math.Round(c.H)), int(math.Round(c.S*100)), int(math.Round(c.L*100)))
}

func deriveHexColor(color parsedColor, hueOffset float64) string {
	// HSL hue rotation is the most predictable way to derive complementary and
	// analogous colors while preserving the source color's saturation/lightness.
	r, g, b := hslToRGB(normalizeHue(color.H+hueOffset), color.S, color.L)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func rgbToHSL(r int, g int, b int) (float64, float64, float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	maxValue := math.Max(rf, math.Max(gf, bf))
	minValue := math.Min(rf, math.Min(gf, bf))
	lightness := (maxValue + minValue) / 2

	if maxValue == minValue {
		return 0, 0, lightness
	}

	delta := maxValue - minValue
	saturation := delta / (1 - math.Abs(2*lightness-1))
	hue := 0.0
	switch maxValue {
	case rf:
		hue = 60 * math.Mod((gf-bf)/delta, 6)
	case gf:
		hue = 60 * ((bf-rf)/delta + 2)
	case bf:
		hue = 60 * ((rf-gf)/delta + 4)
	}

	return normalizeHue(hue), saturation, lightness
}

func hslToRGB(h float64, s float64, l float64) (int, int, int) {
	chroma := (1 - math.Abs(2*l-1)) * s
	x := chroma * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - chroma/2

	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = chroma, x, 0
	case h < 120:
		rf, gf, bf = x, chroma, 0
	case h < 180:
		rf, gf, bf = 0, chroma, x
	case h < 240:
		rf, gf, bf = 0, x, chroma
	case h < 300:
		rf, gf, bf = x, 0, chroma
	default:
		rf, gf, bf = chroma, 0, x
	}

	return clampColorChannel((rf + m) * 255), clampColorChannel((gf + m) * 255), clampColorChannel((bf + m) * 255)
}

func normalizeHue(h float64) float64 {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	return h
}

func clampColorChannel(value float64) int {
	rounded := int(math.Round(value))
	if rounded < 0 {
		return 0
	}
	if rounded > 255 {
		return 255
	}
	return rounded
}
