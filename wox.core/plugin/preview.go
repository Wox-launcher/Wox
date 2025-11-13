package plugin

import (
	"wox/common"
)

type WoxPreviewType = string
type WoxPreviewScrollPosition = string

const (
	WoxPreviewTypeMarkdown = "markdown"
	WoxPreviewTypeText     = "text"
	WoxPreviewTypeImage    = "image" // when type is image, data should be WoxImage.String()
	WoxPreviewTypeUrl      = "url"
	WoxPreviewTypeFile     = "file"   // when type is file(can be *.md, *.jpg, *.pdf and so on), data should be url/filepath
	WoxPreviewTypeRemote   = "remote" // when type is remote, data should be url to load WoxPreview

	// internal use
	WoxPreviewTypePluginDetail = "plugin_detail" // when type is plugin_detail, data should be JSON string of plugin metadata
	WoxPreviewTypeChat         = "chat"          // when type is chat, data should be Json string of common.AIChatData. this type is only for ai chat plugin
)

const (
	WoxPreviewScrollPositionBottom = "bottom" // scroll to bottom after preview first show
)

type WoxPreview struct {
	PreviewType       WoxPreviewType
	PreviewData       string
	PreviewProperties map[string]string // key support i18n
	ScrollPosition    WoxPreviewScrollPosition
}

func (p *WoxPreview) IsEmpty() bool {
	return p.PreviewData == ""
}

type WoxPreviewChatData struct {
	Conversations []common.Conversation
	Model         common.Model
}
