//go:build (!windows && !linux && !darwin) || (windows && !amd64) || (linux && !amd64) || (darwin && !amd64 && !arm64)

package resource

import "embed"

var DictationNativeFS embed.FS

const dictationNativeResourcePath = ""
