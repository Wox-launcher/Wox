package font

import (
	"reflect"
	"testing"
)

func TestNormalizeFontFamilies(t *testing.T) {
	fontFamilies := []string{
		"",
		"  Arial  ",
		"arial",
		"'PingFang SC'",
		"\"PingFang SC\"",
		"Segoe UI",
	}

	result := normalizeFontFamilies(fontFamilies)
	expected := []string{"Arial", "PingFang SC", "Segoe UI"}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("normalizeFontFamilies() = %v, want %v", result, expected)
	}
}

func TestParseFcListOutput(t *testing.T) {
	output := `/usr/share/fonts/dejavu/DejaVuSans.ttf: DejaVu Sans,DejaVu Sans Condensed:style=Book
/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc: Noto Sans CJK SC:style=Regular`

	result := parseFcListOutput(output)
	expected := []string{"DejaVu Sans", "DejaVu Sans Condensed", "Noto Sans CJK SC"}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("parseFcListOutput() = %v, want %v", result, expected)
	}
}

func TestParseWindowsRegFontsOutput(t *testing.T) {
	output := `
HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts
    Segoe UI (TrueType)    REG_SZ    segoeui.ttf
    @Malgun Gothic (TrueType)    REG_SZ    malgun.ttf
    Arial (TrueType)    REG_SZ    arial.ttf
`

	result := parseWindowsRegFontsOutput(output)
	expected := []string{"Segoe UI", "Malgun Gothic", "Arial"}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("parseWindowsRegFontsOutput() = %v, want %v", result, expected)
	}
}

func TestParseSystemProfilerFontsOutput(t *testing.T) {
	output := []byte(`{
  "SPFontsDataType": [
    {
      "_name": "Kaiti.ttc",
      "typefaces": [
        {
          "family": "KaiTi",
          "fullname": "KaiTi Regular"
        }
      ]
    },
    {
      "_name": "SFCompactItalic.ttf",
      "typefaces": [
        {
          "family": ".SF Compact",
          "fullname": ".SF Compact"
        }
      ]
    },
    {
      "_name": "CustomFont-Regular.ttf",
      "typefaces": []
    }
  ]
}`)

	result := parseSystemProfilerFontsOutput(output)
	expected := []string{"KaiTi", "CustomFont-Regular"}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("parseSystemProfilerFontsOutput() = %v, want %v", result, expected)
	}
}

func TestNormalizeConfiguredFontFamily(t *testing.T) {
	available := []string{"KaiTi", "Noto Sans", "SF Pro Text"}

	if got := NormalizeConfiguredFontFamily("Kaiti.ttc", available); got != "KaiTi" {
		t.Fatalf("NormalizeConfiguredFontFamily(Kaiti.ttc) = %v, want %v", got, "KaiTi")
	}

	if got := NormalizeConfiguredFontFamily("Noto Sans", available); got != "Noto Sans" {
		t.Fatalf("NormalizeConfiguredFontFamily(Noto Sans) = %v, want %v", got, "Noto Sans")
	}

	if got := NormalizeConfiguredFontFamily("Unknown.ttf", available); got != "" {
		t.Fatalf("NormalizeConfiguredFontFamily(Unknown.ttf) = %v, want empty", got)
	}
}
