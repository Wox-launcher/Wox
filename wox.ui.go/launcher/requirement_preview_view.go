package launcher

import (
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildRequirementPreview renders the compact plugin configuration flow inside the query preview.
func (a *App) buildRequirementPreview(result queryResult, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
	form, err := a.ensureRequirementForm(result, preview)
	if err != nil {
		return woxwidget.Container{
			Width: width, Height: height, Padding: woxwidget.UniformInsets(18),
			Child: woxwidget.TextBlock{Value: err.Error(), Width: max(float32(0), width-36), Height: max(float32(0), height-36), Style: woxui.TextStyle{Size: 13}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255}},
		}
	}
	innerWidth := max(float32(0), width-36)
	innerHeight := max(float32(0), height-28)
	titleHeight := float32(28)
	messageHeight := float32(42)
	footerHeight := float32(48)
	errorMessage := form.error
	if errorMessage == "" && form.modelsError != "" && hasFormDefinitionType(form.definitions, "selectAIModel") {
		errorMessage = "Unable to load AI models: " + form.modelsError
	}
	errorHeight := float32(0)
	if strings.TrimSpace(errorMessage) != "" {
		errorHeight = 30
	}
	bodyHeight := max(float32(48), innerHeight-titleHeight-messageHeight-footerHeight-errorHeight)
	a.setRequirementFormViewport(form.key, bodyHeight)
	contentHeight := max(bodyHeight, formDefinitionsContentHeight(form.definitions))
	callbacks := formFieldCallbacks{
		idPrefix:  "requirement-form",
		focus:     a.focusRequirementFormField,
		change:    a.changeRequirementFormChoice,
		setCaret:  a.setRequirementFormCaret,
		openTable: a.openRequirementFormTable,
	}
	rows := make([]woxwidget.Widget, 0, len(form.definitions))
	for index, definition := range form.definitions {
		rowHeight := formDefinitionHeight(definition)
		rows = append(rows, a.buildFormField(form.formFieldsSnapshot, callbacks, palette, index, definition, innerWidth, rowHeight))
	}
	body := woxwidget.Gesture{
		ID: "requirement-form-scroll",
		OnScroll: func(delta woxui.Point) {
			a.scrollRequirementForm(form.key, -delta.Y)
		},
		Child: woxwidget.ScrollView{
			Width: innerWidth, Height: bodyHeight, ContentHeight: contentHeight, Offset: form.scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		},
	}
	saveLabel := a.translate("i18n:ui_save")
	buttonColor := palette.actionSelected
	if form.saving {
		saveLabel += "…"
		buttonColor = palette.selectedBackground
	}
	button := woxwidget.Gesture{ID: "requirement-form-save", OnTap: a.submitRequirementForm, Child: woxwidget.Container{
		Width: 104, Height: 36, Radius: 8, Color: buttonColor, Padding: woxwidget.Insets{Left: 22, Top: 10, Right: 18},
		Child: woxwidget.Text{Value: saveLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionSelectedText},
	}}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-104), Height: footerHeight},
		button,
	}}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: titleHeight, Child: woxwidget.Text{Value: form.title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}},
		woxwidget.Container{Width: innerWidth, Height: messageHeight, Child: woxwidget.TextBlock{Value: form.message, Width: innerWidth, Height: messageHeight, Style: woxui.TextStyle{Size: 12}, LineHeight: 17, Color: palette.resultSubtitle}},
		body,
	}
	if errorHeight > 0 {
		children = append(children, woxwidget.Container{Width: innerWidth, Height: errorHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
			Value: errorMessage, Style: woxui.TextStyle{Size: 11}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}})
	}
	children = append(children, footer)
	if len(form.definitions) == 0 {
		children[2] = woxwidget.Container{Width: innerWidth, Height: bodyHeight, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Text{
			Value: fmt.Sprintf("No editable settings were provided for %s.", form.pluginName), Style: woxui.TextStyle{Size: 12}, Color: palette.resultSubtitle,
		}}
	}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 14},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}
}
