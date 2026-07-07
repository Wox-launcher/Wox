//go:build darwin && arm64

package resource

import "embed"

//go:embed dictation/darwin/arm64
var DictationNativeFS embed.FS

const dictationNativeResourcePath = "dictation/darwin/arm64"
