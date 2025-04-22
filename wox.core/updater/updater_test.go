package updater

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestCheckUpdate(t *testing.T) {
	version1, v1Err := semver.NewVersion("2.0.0-beta.2")
	version2, v2Err := semver.NewVersion("2.0.0")

	assert.Nil(t, v1Err)
	assert.Nil(t, v2Err)
	assert.True(t, version1.LessThan(version2), true)
}
