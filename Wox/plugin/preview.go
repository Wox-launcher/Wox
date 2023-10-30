package plugin

type WoxPreviewType = string

const (
	WoxPreviewTypeMarkdown = "markdown"
	WoxPreviewTypeText     = "text"
	WoxPreviewTypeImage    = "image" // when type is image, data should be WoxImage serialized with json
	WoxPreviewTypeUrl      = "url"
)

type WoxPreview struct {
	PreviewType       WoxPreviewType
	PreviewData       string
	PreviewProperties map[string]string // key support i18n
}
