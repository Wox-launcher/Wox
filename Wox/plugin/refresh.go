package plugin

type RefreshableResult struct {
	Title           string
	SubTitle        string
	Icon            WoxImage
	Preview         WoxPreview
	ContextData     string
	RefreshInterval int // set to 0 if you don't want to refresh this result anymore
}
