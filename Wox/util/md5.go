package util

import (
	"crypto/md5"
	"fmt"
)

func Md5(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}
