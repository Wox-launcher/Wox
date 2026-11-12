package system

import (
	"context"
	"slices"
	"strings"
	"unicode"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
	"wox/util/clipboard"

	"github.com/Masterminds/semver/v3"
)

const localSendPluginID = "407fdbba-bdab-4bc1-aa09-b46e811155a6"
const localSendFilePathKey = "localsend.filePath"
const localSendTextKey = "localsend.text"

var (
	localSendHandoffVersion     = semver.MustParse("0.1.1")
	localSendTextHandoffVersion = semver.MustParse("0.1.2")
)

type localSendTarget struct {
	keyword      string
	icon         common.WoxImage
	supportsText bool
}

func (t localSendTarget) canSend(isText bool) bool {
	return t.keyword != "" && (!isText || t.supportsText)
}

func getLocalSendTarget(ctx context.Context) localSendTarget {
	return localSendTargetFromInfo(plugin.GetPluginManager().GetLoadedPluginInfo(ctx, localSendPluginID))
}

// localSendTargetFromInfo checks the handoff versions and prefers the default send trigger.
func localSendTargetFromInfo(info plugin.GetPluginInfoResult) localSendTarget {
	if !info.Installed || !info.Ready || !info.Enabled {
		return localSendTarget{}
	}
	version, err := semver.NewVersion(info.Version)
	if err != nil || version.LessThan(localSendHandoffVersion) {
		return localSendTarget{}
	}
	var target localSendTarget
	if slices.Contains(info.TriggerKeywords, "send") {
		target.keyword = "send"
	} else {
		for _, keyword := range info.TriggerKeywords {
			if keyword != "" && keyword != "*" && keyword != "receive" && !strings.ContainsFunc(keyword, unicode.IsSpace) {
				target.keyword = keyword
				break
			}
		}
	}
	if target.keyword == "" {
		return target
	}
	target.supportsText = !version.LessThan(localSendTextHandoffVersion)
	if info.Icon != nil {
		target.icon = *info.Icon
	} else {
		target.icon = icons.Get(icons.ControlSend)
	}
	return target
}

// appendLocalSendAction adds a handoff for the original clipboard payload.
func (c *ClipboardPlugin) appendLocalSendAction(result *plugin.QueryResult, record ClipboardRecord, lookup func() localSendTarget) {
	var key, value string
	switch clipboard.Type(record.Type) {
	case clipboard.ClipboardTypeText:
		key, value = localSendTextKey, record.Content
	case clipboard.ClipboardTypeImage:
		key, value = localSendFilePathKey, record.FilePath
	}
	if value == "" {
		return
	}
	isText := key == localSendTextKey
	target := lookup()
	if !target.canSend(isText) {
		return
	}
	result.Actions = append(result.Actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_clipboard_send_via_localsend",
		Icon:                   target.icon,
		PreventHideAfterAction: true,
		ContextData:            clipboardMRUContext(record.ID),
		Action: func(ctx context.Context, _ plugin.ActionContext) {
			current := localSendTargetFromInfo(c.api.GetPluginInfo(ctx, plugin.GetPluginInfoOption{PluginId: localSendPluginID}))
			if !current.canSend(isText) {
				return
			}
			if !isText && !util.IsFileExists(value) {
				c.api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: "i18n:plugin_clipboard_localsend_image_missing"})
				return
			}
			c.api.ChangeQuery(ctx, common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: current.keyword + " ",
				ContextData: common.ContextData{
					plugin.QueryContextDataRecipientKey: localSendPluginID,
					key:                                 value,
				},
			})
		},
	})
}
