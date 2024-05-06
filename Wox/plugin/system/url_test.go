package system

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUrlPlugin_Query(t *testing.T) {
	urlPlugin := &UrlPlugin{}
	reg := urlPlugin.getReg()

	assert.Greater(t, len(reg.FindStringIndex("https://www.google.com")), 0)
	assert.Greater(t, len(reg.FindStringIndex("bilibili.com")), 0)

	// some invalid urls
	assert.Equal(t, 0, len(reg.FindStringIndex("http://google")))
	assert.Equal(t, 0, len(reg.FindStringIndex("http://.google.com")))
}
