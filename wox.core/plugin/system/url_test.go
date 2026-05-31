package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUrlPlugin_Query(t *testing.T) {
	urlPlugin := &UrlPlugin{}
	reg := urlPlugin.getReg()

	assert.Greater(t, len(reg.FindStringIndex("https://www.google.com")), 0)
	assert.Greater(t, len(reg.FindStringIndex("bilibili.com")), 0)

	// IP address URLs
	assert.Greater(t, len(reg.FindStringIndex("192.168.1.10")), 0)
	assert.Greater(t, len(reg.FindStringIndex("http://192.168.1.10")), 0)
	assert.Greater(t, len(reg.FindStringIndex("https://192.168.1.10")), 0)
	assert.Greater(t, len(reg.FindStringIndex("http://192.168.1.10:8080")), 0)
	assert.Greater(t, len(reg.FindStringIndex("http://192.168.1.10:8080/path")), 0)
	assert.Greater(t, len(reg.FindStringIndex("10.0.0.1")), 0)
	assert.Greater(t, len(reg.FindStringIndex("255.255.255.255")), 0)

	// some invalid urls
	assert.Equal(t, 0, len(reg.FindStringIndex("http://google")))
	assert.Equal(t, 0, len(reg.FindStringIndex("http://.google.com")))
}
