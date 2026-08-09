package webview

import (
	"context"
	"fmt"

	"wox/util"
)

// LogEscapeDiagnostic emits one consistently formatted step from the native WebView Escape handoff.
func LogEscapeDiagnostic(platform string, owner uintptr, detail string) {
	util.GetLogger().Info(context.Background(), fmt.Sprintf("webview escape diagnostic: platform=%s owner=%#x detail=%s", platform, owner, detail))
}
