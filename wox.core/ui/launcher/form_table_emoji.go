package launcher

import (
	"context"
	"strings"

	emojiplugin "wox/plugin/system/emoji"
	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/emojisearch"
)

// formTableEmojiGroup mirrors Flutter's WoxImageSelector.emojiGroups catalog so
// the tray query and plugin table icon fields share one picker.
type formTableEmojiGroup struct {
	LabelKey string
	Marker   string
	Emojis   []string
}

var formTableEmojiGroups = []formTableEmojiGroup{
	{LabelKey: "ui_select_emoji_group_recommended", Marker: "👍", Emojis: []string{
		"🤖",
		"💡",
		"🔍",
		"📊",
		"📈",
		"📝",
		"🛠",
		"⚙️",
		"🧠",
		"✅",
		"🚀",
		"🎯",
		"🔥",
		"⭐",
		"🌟",
		"💻",
		"📱",
		"🌐",
		"🔒",
		"🧩",
		"📌",
		"📎",
		"🗂️",
		"📦",
		"📁",
		"📂",
		"⌨️",
		"🖥️",
		"⚡",
		"🏷️",
		"💬",
		"ℹ️",
		"❤️",
		"😂",
		"😍",
		"🎉",
		"👍",
		"👏",
		"🙏",
		"💯",
		"🕘",
		"📅",
		"🗓️",
		"⏰",
		"⏱️",
		"🧭",
		"🗺️",
		"🏠",
		"🏢",
		"🏁",
		"🚧",
		"🧪",
		"🔬",
		"📚",
		"📖",
		"✏️",
		"🖊️",
		"📋",
		"📍",
		"🔔",
		"🔕",
		"🔑",
		"🛡️",
		"💎",
		"🏆",
		"🥇",
		"🎁",
		"🛒",
		"💰",
		"💳",
		"📣",
		"🎨",
	}},
	{LabelKey: "ui_select_emoji_group_faces", Marker: "😊", Emojis: []string{
		"😀",
		"😃",
		"😄",
		"😁",
		"😆",
		"😅",
		"😂",
		"🤣",
		"😊",
		"🙂",
		"😉",
		"😍",
		"😘",
		"😜",
		"🤩",
		"🤔",
		"🤨",
		"😐",
		"😑",
		"🙄",
		"😏",
		"😶",
		"😮",
		"😲",
		"😴",
		"😌",
		"🤤",
		"😷",
		"🤒",
		"🤕",
		"🥳",
		"😎",
		"🤯",
		"😇",
		"🥲",
		"😭",
		"😡",
		"🤬",
		"🥶",
		"🥵",
	}},
	{LabelKey: "ui_select_emoji_group_people", Marker: "👤", Emojis: []string{
		"👤",
		"👥",
		"🧑",
		"👨",
		"👩",
		"🧔",
		"👱",
		"👶",
		"🧒",
		"👦",
		"👧",
		"🧑‍💻",
		"👨‍💻",
		"👩‍💻",
		"🧑‍🔬",
		"🧑‍🏫",
		"🧑‍🎨",
		"🧑‍🚀",
		"🧑‍🚒",
		"🧑‍⚕️",
		"🕵️",
		"💂",
		"👷",
		"🧙",
		"🧛",
		"🧟",
		"🙋",
		"🙌",
		"👏",
		"👍",
		"👎",
		"👊",
		"✌️",
		"🤝",
		"🙏",
		"💪",
		"👀",
		"🫶",
		"👋",
		"🤟",
	}},
	{LabelKey: "ui_select_emoji_group_animals", Marker: "🐾", Emojis: []string{
		"🐶",
		"🐱",
		"🐭",
		"🐹",
		"🐰",
		"🦊",
		"🐻",
		"🐼",
		"🐨",
		"🐯",
		"🦁",
		"🐮",
		"🐷",
		"🐸",
		"🐵",
		"🦄",
		"🐔",
		"🐧",
		"🐦",
		"🐤",
		"🦆",
		"🦅",
		"🦉",
		"🦇",
		"🐺",
		"🐗",
		"🐴",
		"🦋",
		"🐝",
		"🐞",
		"🐢",
		"🐍",
		"🦖",
		"🦕",
		"🐙",
		"🦑",
		"🐬",
		"🐳",
		"🦈",
		"🌿",
	}},
	{LabelKey: "ui_select_emoji_group_nature", Marker: "🌿", Emojis: []string{
		"🌱",
		"🌿",
		"☘️",
		"🍀",
		"🎍",
		"🪴",
		"🎋",
		"🍃",
		"🍂",
		"🍁",
		"🍄",
		"🐚",
		"🪨",
		"🌾",
		"💐",
		"🌷",
		"🌹",
		"🥀",
		"🌺",
		"🌸",
		"🌼",
		"🌻",
		"🌞",
		"🌝",
		"🌛",
		"🌜",
		"🌚",
		"🌕",
		"🌖",
		"🌗",
		"🌘",
		"🌑",
		"🌒",
		"🌓",
		"🌔",
		"⭐",
		"🌟",
		"✨",
		"⚡",
		"🔥",
		"🌈",
		"☀️",
		"🌤️",
		"⛅",
		"☁️",
		"🌧️",
		"⛈️",
		"🌩️",
		"🌨️",
		"❄️",
		"☃️",
		"🌬️",
		"💨",
		"💧",
		"💦",
		"☔",
		"🌊",
		"🌍",
		"🌎",
		"🌏",
	}},
	{LabelKey: "ui_select_emoji_group_food", Marker: "🍽️", Emojis: []string{
		"🍎",
		"🍊",
		"🍋",
		"🍌",
		"🍉",
		"🍇",
		"🍓",
		"🫐",
		"🍒",
		"🍍",
		"🥭",
		"🥝",
		"🍅",
		"🥑",
		"🥦",
		"🥕",
		"🌽",
		"🍞",
		"🥐",
		"🧀",
		"🍗",
		"🍖",
		"🍔",
		"🍟",
		"🍕",
		"🌭",
		"🌮",
		"🌯",
		"🍜",
		"🍝",
		"🍣",
		"🍤",
		"🍰",
		"🧁",
		"🍩",
		"🍪",
		"🍫",
		"☕",
		"🍵",
		"🥤",
	}},
	{LabelKey: "ui_select_emoji_group_activities", Marker: "🎉", Emojis: []string{
		"⚽",
		"🏀",
		"🏈",
		"⚾",
		"🎾",
		"🏐",
		"🏉",
		"🥏",
		"🎱",
		"🏓",
		"🏸",
		"🏒",
		"🏑",
		"🥍",
		"🏏",
		"⛳",
		"🏹",
		"🎣",
		"🥊",
		"🥋",
		"🎮",
		"🕹️",
		"🎲",
		"🧩",
		"♟️",
		"🎯",
		"🎳",
		"🎼",
		"🎵",
		"🎶",
		"🎸",
		"🎹",
		"🥁",
		"🎻",
		"🎺",
		"🎨",
		"🎭",
		"🎬",
		"🎪",
		"🏆",
	}},
	{LabelKey: "ui_select_emoji_group_travel", Marker: "🚗", Emojis: []string{
		"🚗",
		"🚕",
		"🚙",
		"🚌",
		"🚎",
		"🏎️",
		"🚓",
		"🚑",
		"🚒",
		"🚚",
		"🚜",
		"🚲",
		"🛴",
		"🏍️",
		"🚨",
		"🚦",
		"🚥",
		"🛣️",
		"🛤️",
		"⛽",
		"🚉",
		"✈️",
		"🛫",
		"🛬",
		"🚁",
		"🚀",
		"🛰️",
		"⛵",
		"🚤",
		"🛳️",
		"🗺️",
		"🧭",
		"🏔️",
		"🏕️",
		"🏖️",
		"🏝️",
		"🏜️",
		"🌋",
		"🌅",
		"🌃",
	}},
	{LabelKey: "ui_select_emoji_group_objects", Marker: "📦", Emojis: []string{
		"📱",
		"☎️",
		"💻",
		"⌨️",
		"🖥️",
		"🖨️",
		"🖱️",
		"💾",
		"💿",
		"📷",
		"📹",
		"🎥",
		"📞",
		"📟",
		"📠",
		"📺",
		"📻",
		"🧭",
		"⏰",
		"⏱️",
		"💡",
		"🔦",
		"🕯️",
		"🧯",
		"🧰",
		"🔧",
		"🔨",
		"⚒️",
		"🪛",
		"🔩",
		"⚙️",
		"🧲",
		"🧪",
		"🧫",
		"🧬",
		"💊",
		"📦",
		"📚",
		"🗂️",
		"🗃️",
	}},
	{LabelKey: "ui_select_emoji_group_flags", Marker: "🏳️", Emojis: []string{
		"🏁",
		"🚩",
		"🎌",
		"🏴",
		"🏳️",
		"🏳️‍🌈",
		"🏳️‍⚧️",
		"🏴‍☠️",
		"🇨🇳",
		"🇺🇸",
		"🇬🇧",
		"🇯🇵",
		"🇰🇷",
		"🇩🇪",
		"🇫🇷",
		"🇮🇹",
		"🇪🇸",
		"🇨🇦",
		"🇦🇺",
		"🇳🇿",
		"🇮🇳",
		"🇧🇷",
		"🇲🇽",
		"🇦🇷",
		"🇨🇱",
		"🇵🇪",
		"🇨🇴",
		"🇵🇹",
		"🇳🇱",
		"🇧🇪",
		"🇨🇭",
		"🇦🇹",
		"🇸🇪",
		"🇳🇴",
		"🇩🇰",
		"🇫🇮",
		"🇵🇱",
		"🇨🇿",
		"🇬🇷",
		"🇹🇷",
		"🇺🇦",
		"🇸🇬",
		"🇲🇾",
		"🇹🇭",
		"🇻🇳",
		"🇮🇩",
		"🇵🇭",
		"🇿🇦",
		"🇪🇬",
		"🇦🇪",
	}},
	{LabelKey: "ui_select_emoji_group_symbols", Marker: "#️⃣", Emojis: []string{
		"❤️",
		"🧡",
		"💛",
		"💚",
		"💙",
		"💜",
		"🖤",
		"🤍",
		"🤎",
		"💔",
		"❣️",
		"💕",
		"💞",
		"💯",
		"✅",
		"☑️",
		"✔️",
		"❌",
		"⚠️",
		"❗",
		"❓",
		"💤",
		"♻️",
		"⚜️",
		"🔱",
		"♾️",
		"™️",
		"©️",
		"®️",
		"➕",
		"➖",
		"✖️",
		"➗",
		"🟢",
		"🔴",
		"🟡",
		"🔵",
		"🟣",
		"⚪",
		"⚫",
	}},
}

// openFormTableEmojiPicker opens the shared emoji dialog for one woxImage row field.
func (a *App) openFormTableEmojiPicker(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "woxImage" {
		return
	}
	initialEmoji := strings.TrimSpace(state.rowForm.values[state.rowForm.definitions[index].Value.Key])
	if initialEmoji != "" && !strings.HasPrefix(initialEmoji, "{") {
		a.rememberFormTableEmoji(initialEmoji)
	}
	a.ensureFormTableEmojiSearchEntries()
	state.emojiPicker = &formTableEmojiPickerState{
		fieldIndex:   index,
		initialEmoji: initialEmoji,
	}
	clearFormTableRowValidationLocked(state)
	a.updateFormTableTextInput(true)
	a.invalidateFormTableWindow()
}

// ensureFormTableEmojiSearchEntries prepares the plugin catalog once outside the render path.
func (a *App) ensureFormTableEmojiSearchEntries() {
	a.formTableEmojiSearchOnce.Do(func() {
		catalog, err := emojiplugin.LoadCatalog()
		if err != nil {
			ctx := a.lifecycleCtx
			if ctx == nil {
				ctx = context.Background()
			}
			util.GetLogger().Error(ctx, "load emoji picker search catalog: "+err.Error())
			return
		}
		a.formTableEmojiSearchEntries = make([]emojisearch.Entry, len(catalog))
		for index, entry := range catalog {
			a.formTableEmojiSearchEntries[index] = emojisearch.Entry{Emoji: entry.Emoji, SearchTerms: entry.SearchTerms}
		}
	})
}

func (a *App) closeFormTableEmojiPicker() {
	state := a.activeFormTableEditor()
	textInput := false
	if state != nil && state.emojiPicker != nil {
		state.emojiPicker = nil
		clearFormTableRowValidationLocked(state)
		textInput = state.rowForm != nil && state.rowForm.editor != nil
	}
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

// chooseFormTableEmoji commits one emoji to the focused woxImage row field.
func (a *App) chooseFormTableEmoji(emoji string) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.emojiPicker == nil || emoji == "" {
		return
	}
	fieldIndex := state.emojiPicker.fieldIndex
	if fieldIndex < 0 || fieldIndex >= len(state.rowForm.definitions) {
		return
	}
	state.rowForm.values[state.rowForm.definitions[fieldIndex].Value.Key] = emoji
	a.rememberFormTableEmoji(emoji)
	state.emojiPicker = nil
	clearFormTableRowValidationLocked(state)
	setFormFieldsFocusLocked(state.rowForm, fieldIndex)
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// rememberFormTableEmoji keeps a small process-local MRU list for the picker.
func (a *App) rememberFormTableEmoji(emoji string) {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return
	}
	recent := make([]string, 0, min(24, len(a.recentFormTableEmojis)+1))
	recent = append(recent, emoji)
	for _, existing := range a.recentFormTableEmojis {
		if existing != emoji && len(recent) < 24 {
			recent = append(recent, existing)
		}
	}
	a.recentFormTableEmojis = recent
}

// buildFormTableEmojiPicker maps controller state onto the pure emoji dialog.
func (a *App) buildFormTableEmojiPicker(snapshot *formTableEmojiPickerSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	groups := make([]launcherview.FormTableEmojiGroup, 0, len(formTableEmojiGroups)+1)
	if len(a.recentFormTableEmojis) > 0 {
		groups = append(groups, launcherview.FormTableEmojiGroup{
			Label: a.translate("i18n:ui_select_emoji_group_recent"), Marker: "🕘", Emojis: append([]string(nil), a.recentFormTableEmojis...),
		})
	}
	for _, group := range formTableEmojiGroups {
		groups = append(groups, launcherview.FormTableEmojiGroup{Label: a.translate("i18n:" + group.LabelKey), Marker: group.Marker, Emojis: append([]string(nil), group.Emojis...)})
	}
	theme := palette.componentTheme()
	iconTint := palette.resultSubtitle
	return launcherview.FormTableEmojiPicker(launcherview.FormTableEmojiPickerProps{
		OverlayWidth: width, OverlayHeight: height, Window: a.formTableNativeWindow(), Theme: theme,
		Title: a.translate("i18n:ui_select_emoji"), SearchLabel: a.translate("i18n:ui_select_emoji_search"),
		SearchResultsLabel: a.translate("i18n:ui_select_emoji_search_results"), NoResultsLabel: a.translate("i18n:ui_select_emoji_no_results"),
		CloseLabel: a.translate("i18n:ui_close"), SearchIcon: a.imageForTint(settingControlIconSource("search"), &iconTint, physicalImageSize(18, imageScale)),
		Groups: groups, SearchEntries: a.formTableEmojiSearchEntries, InitialEmoji: snapshot.initialEmoji,
		OnChoose: a.chooseFormTableEmoji, OnCancel: a.closeFormTableEmojiPicker,
	})
}
