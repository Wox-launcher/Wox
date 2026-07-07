//go:build darwin && amd64

package resource

import "embed"

//go:embed dictation/darwin/amd64
var DictationNativeFS embed.FS

const dictationNativeResourcePath = "dictation/darwin/amd64"
