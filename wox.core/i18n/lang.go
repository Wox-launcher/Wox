package i18n

type LangCode string

type Lang struct {
	Code LangCode
	Name string
}

const (
	LangCodeEnUs LangCode = "en_US"
	LangCodeZhCn LangCode = "zh_CN"
	LangCodeRuRu LangCode = "ru_RU"
)

func GetSupportedLanguages() []Lang {
	return []Lang{
		{
			Code: LangCodeEnUs,
			Name: "English",
		},
		{
			Code: LangCodeZhCn,
			Name: "简体中文",
		},
		{
			Code: LangCodeRuRu,
			Name: "Русский",
		},
	}
}

func IsSupportedLangCode(langCode string) bool {
	for _, lang := range GetSupportedLanguages() {
		if string(lang.Code) == langCode {
			return true
		}
	}
	return false
}
