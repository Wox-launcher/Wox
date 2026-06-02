package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"wox/util"
)

func TestUrlPlugin_Query(t *testing.T) {
	assert.True(t, util.IsUrl("https://www.google.com"))
	assert.True(t, util.IsUrl("bilibili.com"))

	// IP address URLs
	assert.True(t, util.IsUrl("192.168.1.10"))
	assert.True(t, util.IsUrl("http://192.168.1.10"))
	assert.True(t, util.IsUrl("https://192.168.1.10"))
	assert.True(t, util.IsUrl("http://192.168.1.10:8080"))
	assert.True(t, util.IsUrl("http://192.168.1.10:8080/path"))
	assert.True(t, util.IsUrl("10.0.0.1"))
	assert.True(t, util.IsUrl("255.255.255.255"))

	// some invalid urls
	assert.False(t, util.IsUrl("http://google"))
	assert.False(t, util.IsUrl("http://.google.com"))
}
