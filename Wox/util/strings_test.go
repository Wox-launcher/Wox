package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPinyin(t *testing.T) {
	contain := StringContainsPinYin("你好", "nh")

	assert.True(t, contain)
}
