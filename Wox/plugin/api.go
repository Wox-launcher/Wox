package plugin

type API interface {
	ChangeQuery(query string)
	HideApp()
	ShowApp()
	ShowMsg(title string, description string, icon string)
	Log(msg string)
	GetTranslation(key string) string
}

type APIImpl struct {
	metadata Metadata
}

func (a *APIImpl) ChangeQuery(query string) {

}

func (a *APIImpl) HideApp() {

}

func (a *APIImpl) ShowApp() {

}

func (a *APIImpl) ShowMsg(title string, description string, icon string) {

}

func (a *APIImpl) Log(msg string) {

}

func (a *APIImpl) GetTranslation(key string) string {
	return ""
}

func NewAPI(metadata Metadata) API {
	return &APIImpl{metadata: metadata}
}
