package platform

type PreviewKind string

const (
	PreviewKindNone        PreviewKind = ""
	PreviewKindText        PreviewKind = "text"
	PreviewKindMarkdown    PreviewKind = "markdown"
	PreviewKindUnsupported PreviewKind = "unsupported"
)

type PreviewProperty struct {
	Title   string
	Content string
}

type PreviewContent struct {
	Kind       PreviewKind
	Content    string
	Properties []PreviewProperty
}

type PreviewState struct {
	Visible bool
	Frame   Rect
	Title   string
	Body    PreviewContent
}
