package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.github.wox</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.AppPath}}</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
	<key>LSUIElement</key>
	<true/>
	<key>CFBundleIconFile</key>
	<string>app.icns</string>
</dict>
</plist>`

func setAutostart(enable bool) error {
	appPath := filepath.Dir(filepath.Dir(filepath.Dir(os.Args[0])))
	if !strings.HasSuffix(appPath, ".app") {
		return fmt.Errorf("not running from an .app bundle")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.github.wox.plist")

	if enable {
		// Create plist file
		tmpl, err := template.New("plist").Parse(plistTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse plist template: %w", err)
		}

		file, err := os.Create(plistPath)
		if err != nil {
			return fmt.Errorf("failed to create plist file: %w", err)
		}
		defer file.Close()

		err = tmpl.Execute(file, struct{ AppPath string }{AppPath: appPath})
		if err != nil {
			return fmt.Errorf("failed to write plist file: %w", err)
		}
	} else {
		// Remove plist file
		err := os.Remove(plistPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove plist file: %w", err)
		}
	}

	return nil
}

func isAutostart() (bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("failed to get user home directory: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.github.wox.plist")
	_, err = os.Stat(plistPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check plist file: %w", err)
}
