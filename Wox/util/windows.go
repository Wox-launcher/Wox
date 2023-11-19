package util

import (
	"fmt"
	"github.com/go-vgo/robotgo"
)

func GetActiveWindowHash() string {
	activePid := robotgo.GetPid()
	activeTitle := robotgo.GetTitle()
	return Md5([]byte(fmt.Sprintf("%s%d", activeTitle, activePid)))
}
