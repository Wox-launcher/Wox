package locale

import (
	"fmt"
	"time"
	"unsafe"
	"wox/util"

	"golang.org/x/sys/windows"
	"golang.org/x/text/language"
)

var getUserDefaultLocaleName = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")

// detectLocale avoids blocking startup on PowerShell or its user profile scripts.
func detectLocale() (lang, region string) {
	ctx := util.NewTraceContext()
	logger := util.GetLogger()
	started := time.Now()
	logger.Info(ctx, "detecting Windows locale via GetUserDefaultLocaleName")
	lang, region = "en", "US"
	defer func() {
		logger.Info(ctx, fmt.Sprintf("Windows locale detection completed: language=%s region=%s durationMs=%d", lang, region, time.Since(started).Milliseconds()))
	}()

	var name [85]uint16 // LOCALE_NAME_MAX_LENGTH includes the terminating NUL.
	n, _, err := getUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if n == 0 {
		logger.Warn(ctx, fmt.Sprintf("GetUserDefaultLocaleName failed; using en-US: %v", err))
		return
	}
	localeName := windows.UTF16ToString(name[:])
	lang, region, parseErr := parseWindowsLocale(localeName)
	if parseErr != nil {
		logger.Warn(ctx, fmt.Sprintf("invalid Windows locale %q; using en-US: %v", localeName, parseErr))
		return "en", "US"
	}
	return lang, region
}

// parseWindowsLocale keeps script subtags (such as Hans) out of the region field.
func parseWindowsLocale(name string) (string, string, error) {
	tag, err := language.Parse(name)
	// Windows alternate sort suffixes are unknown BCP 47 values, but Parse
	// preserves the language and region in the tag returned with ValueError.
	if _, unknownValue := err.(language.ValueError); err != nil && !unknownValue {
		return "", "", err
	}
	base, _, region := tag.Raw()
	if base.String() == "und" || region.String() == "ZZ" {
		return "", "", fmt.Errorf("locale must contain a language and region")
	}
	return base.String(), region.String(), nil
}
