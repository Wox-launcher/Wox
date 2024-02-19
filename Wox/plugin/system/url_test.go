package system

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUrlPlugin_Query(t *testing.T) {
	urlPlugin := &UrlPlugin{}
	reg := urlPlugin.getReg()

	assert.Greater(t, len(reg.FindStringIndex("https://www.google.com")), 0)
}
