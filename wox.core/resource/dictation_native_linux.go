//go:build linux && amd64

package resource

import "embed"

//go:embed dictation/linux/amd64
var DictationNativeFS embed.FS

const dictationNativeResourcePath = "dictation/linux/amd64"
