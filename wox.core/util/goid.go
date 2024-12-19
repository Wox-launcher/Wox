package util

import (
	"github.com/petermattis/goid"
)

func GetGID() int64 {
	return goid.Get()
}
