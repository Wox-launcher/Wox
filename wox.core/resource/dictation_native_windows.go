//go:build windows && amd64

package resource

import "embed"

//go:embed dictation/windows/amd64
var DictationNativeFS embed.FS

const dictationNativeResourcePath = "dictation/windows/amd64"
