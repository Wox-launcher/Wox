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
	LangCodePtBr LangCode = "pt_BR"
	LangCodeKoKr LangCode = "ko_KR"
	LangCodeKoKr LangCode = "ja_JP"
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
		{
			Code: LangCodePtBr,
			Name: "Português",
		},
		{
			Code: LangCodeKoKr,
			Name: "한국어",
		},
		{
			Code: LangCodeKoKr,
			Name: "日本語",
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
