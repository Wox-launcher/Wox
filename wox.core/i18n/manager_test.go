package i18n

import (
	"context"
	"testing"
)

func TestTranslateWoxUsesCurrentLanguageThenEnglishFallback(t *testing.T) {
	manager := &Manager{
		currentLangCode: LangCodeZhCn,
		enUsLang:        map[string]string{"only_en": "English only", "shared": "Hello"},
		currentLang:     map[string]string{"shared": "你好", "empty": ""},
	}

	if got := manager.TranslateWox(context.Background(), "i18n:shared"); got != "你好" {
		t.Fatalf("current language = %q, want 你好", got)
	}
	if got := manager.TranslateWox(context.Background(), "only_en"); got != "English only" {
		t.Fatalf("en_US fallback = %q, want English only", got)
	}
	if got := manager.TranslateWox(context.Background(), "empty"); got != "" {
		t.Fatalf("present empty value = %q, want empty string not fallback", got)
	}
	if got := manager.TranslateWox(context.Background(), "i18n:missing"); got != "i18n:missing" {
		t.Fatalf("missing key = %q, want the original key", got)
	}
	if got := manager.TranslateWoxEnUs(context.Background(), "i18n:shared"); got != "Hello" {
		t.Fatalf("TranslateWoxEnUs = %q, want Hello", got)
	}
}

func TestUpdateLangSharesEnglishTableAndReloadsOthers(t *testing.T) {
	manager := GetI18nManager()
	if err := manager.UpdateLang(context.Background(), LangCodeEnUs); err != nil {
		t.Fatalf("UpdateLang(en_US): %v", err)
	}

	english := manager.TranslateWox(context.Background(), "plugin_wox_memory_gpu")
	if english == "" || english == "plugin_wox_memory_gpu" {
		t.Fatalf("en_US translation for plugin_wox_memory_gpu = %q", english)
	}

	if err := manager.UpdateLang(context.Background(), LangCodeZhCn); err != nil {
		t.Fatalf("UpdateLang(zh_CN): %v", err)
	}
	chinese := manager.TranslateWox(context.Background(), "plugin_wox_memory_gpu")
	if chinese == "" || chinese == english {
		t.Fatalf("zh_CN translation = %q, want a distinct value from %q", chinese, english)
	}
	if got := manager.TranslateWoxEnUs(context.Background(), "plugin_wox_memory_gpu"); got != english {
		t.Fatalf("TranslateWoxEnUs after lang change = %q, want %q", got, english)
	}

	if err := manager.UpdateLang(context.Background(), LangCodeEnUs); err != nil {
		t.Fatalf("UpdateLang back to en_US: %v", err)
	}
}
