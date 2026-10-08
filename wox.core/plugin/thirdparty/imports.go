// Package thirdparty blank-imports compatibility layers so their init runs.
// Adding a layer means one blank import in this file. init registers its hosts and store.
package thirdparty

import (
	_ "wox/plugin/thirdparty/flow"
)
