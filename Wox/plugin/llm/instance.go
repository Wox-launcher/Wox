package llm

var provider Provider
var model Model

func GetInstance() (Provider, Model) {
	return provider, model
}

func SetInstance(p Provider, m Model) {
	provider = p
	model = m
}

func IsInstanceReady() bool {
	return provider != nil && model.Name != ""
}
