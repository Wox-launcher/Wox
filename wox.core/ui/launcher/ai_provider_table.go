package launcher

import (
	"encoding/json"
	"fmt"
	"time"

	"wox/common/icons"
	"wox/setting"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	"wox/util"
)

// aiProviderRowTest is UI-thread-owned; the configuration identifies the exact tested row.
type aiProviderRowTest struct {
	provider setting.AIProvider
	id       uint64
	loading  bool
	message  string
}

type aiProviderTestHover struct {
	index    int
	provider setting.AIProvider
	anchor   woxui.Rect
}

// addAIProviderTestActions attaches the probe to saved rows, independent of display sorting.
func (c *aiSettingsController) addAIProviderTestActions(field *launcherview.FormTableFieldProps, value string, tests map[int]aiProviderRowTest, imageScale float32) {
	var providers []setting.AIProvider
	if json.Unmarshal([]byte(value), &providers) != nil {
		return
	}
	color := field.Theme.TextSecondary
	icon := c.imageForTint(fromCoreImage(icons.Get(icons.ControlBolt)), &color, physicalImageSize(16, imageScale))
	for i := range field.Rows {
		index := field.Rows[i].Index
		if index < 0 || index >= len(providers) {
			continue
		}
		provider := providers[index]
		label := c.providerTestTooltip(provider, tests[index])
		field.Rows[i].TrailingActions = append(field.Rows[i].TrailingActions, launcherview.FormTableRowAction{
			ID: "test-connection", Label: label, Icon: icon,
			OnTap:   func() { c.testSavedAIProvider(index, provider) },
			OnHover: func(inside bool, anchor woxui.Rect) { c.hoverAIProviderTest(index, provider, inside, anchor) },
		})
	}
}

// providerTestTooltip includes a result only for the configuration that was tested.
func (c *aiSettingsController) providerTestTooltip(provider setting.AIProvider, test aiProviderRowTest) string {
	text := c.language.translate("i18n:ui_ai_provider_test") + " — " + c.language.translate("i18n:ui_ai_provider_test_description")
	if test.provider == provider && test.message != "" {
		text += "\n" + test.message
	}
	return text
}

// hoverAIProviderTest tracks the anchor so an in-flight result can update the open tooltip.
func (c *aiSettingsController) hoverAIProviderTest(index int, provider setting.AIProvider, inside bool, anchor woxui.Rect) {
	if inside {
		c.providerTestHover = &aiProviderTestHover{index: index, provider: provider, anchor: anchor}
		c.settings.setSettingChoiceTooltip(true, c.providerTestTooltip(provider, c.providerTests[index]), anchor)
	} else if hover := c.providerTestHover; hover != nil && hover.index == index && hover.provider == provider {
		if !util.IsLinux() && nativeHoverTooltipOSCursorOnAnchor(c.settings.settingsNativeWindow, anchor) {
			return
		}
		c.providerTestHover = nil
		c.settings.setSettingChoiceTooltip(false, c.providerTestTooltip(provider, c.providerTests[index]), anchor)
	}
}

// refreshAIProviderTestTooltip changes only the currently hovered row's tooltip.
func (c *aiSettingsController) refreshAIProviderTestTooltip(index int, provider setting.AIProvider) {
	hover := c.providerTestHover
	if hover == nil || hover.index != index || hover.provider != provider || !c.settings.open || c.settings.tab != "ai" {
		return
	}
	c.settings.refreshSettingChoiceTooltip(c.providerTestTooltip(provider, c.providerTests[index]), hover.anchor)
}

// testSavedAIProvider ignores stale completion after a settings reload, edit or newer test.
func (c *aiSettingsController) testSavedAIProvider(index int, provider setting.AIProvider) {
	if !c.savedAIProviderMatches(index, provider) {
		return
	}
	if current, ok := c.providerTests[index]; ok && current.provider == provider && current.loading {
		return
	}
	if c.providerTests == nil {
		c.providerTests = make(map[int]aiProviderRowTest)
	}
	c.providerTestSequence++
	id := c.providerTestSequence
	c.providerTests[index] = aiProviderRowTest{provider: provider, id: id, loading: true, message: c.language.translate("i18n:ui_ai_provider_test_running")}
	form := c.form
	c.refreshAIProviderTestTooltip(index, provider)
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "test saved AI provider", func() {
		key := probeAIProvider(c.lifecycleCtx, provider)
		_ = c.runOnUI("apply saved AI provider test", func() {
			current, ok := c.providerTests[index]
			if c.form != form || !ok || current.id != id || !current.loading || current.provider != provider || !c.savedAIProviderMatches(index, provider) {
				return
			}
			current.loading = false
			current.message = fmt.Sprintf(c.language.translate(key), time.Now().Format("15:04:05"))
			c.providerTests[index] = current
			c.refreshAIProviderTestTooltip(index, provider)
			c.settings.invalidateSettingsWindow()
		})
	})
}

// savedAIProviderMatches prevents an old request from updating an edited or deleted row.
func (c *aiSettingsController) savedAIProviderMatches(index int, provider setting.AIProvider) bool {
	if c.form == nil {
		return false
	}
	var providers []setting.AIProvider
	return json.Unmarshal([]byte(c.form.values["AIProviders"]), &providers) == nil && index >= 0 && index < len(providers) && providers[index] == provider
}
