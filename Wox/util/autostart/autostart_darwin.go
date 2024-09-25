package autostart

import (
	"fmt"
	"os"
	"os/exec"
)

func setAutostart(enable bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	if enable {
		cmd := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to make login item at end with properties {path:"%s", hidden:false}`, exePath))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("添加登录项失败: %w", err)
		}
	} else {
		cmd := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to delete login item "%s"`, exePath))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("删除登录项失败: %w", err)
		}
	}

	return nil
}

// 删除 createLaunchAgent 函数，因为我们不再使用它
