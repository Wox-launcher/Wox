//go:build !linux

package omarchytheme

import (
	"fmt"

	"wox/common"
)

// Available is always false outside Linux. Omarchy Auto is not shown there.
func Available() bool { return false }

// StateDir is empty outside Linux.
func StateDir() string { return "" }

// ColorsPath is empty outside Linux.
func ColorsPath() string { return "" }

// CurrentSlug is empty outside Linux.
func CurrentSlug() string { return "" }

// Load fails outside Linux because there is no Omarchy palette to read.
func Load(string) (common.Theme, error) {
	return common.Theme{}, fmt.Errorf("omarchy palette is only available on Omarchy")
}
