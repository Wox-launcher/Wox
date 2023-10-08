package util

import (
	"fmt"
	"os"
	"path"
	"sync"
)

var locationInstance *Location
var locationOnce sync.Once

type Location struct {
	woxDataDirectory              string
	userDataDirectory             string
	userDataDirectoryShortcutPath string // A file named .wox.location that contains the user data directory path
}

func GetLocation() *Location {
	locationOnce.Do(func() {
		locationInstance = &Location{}
	})
	return locationInstance
}

func (l *Location) Init() error {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	// check if wox data directory exists, if not, create it
	l.woxDataDirectory = path.Join(dirname, ".wox")
	if directoryErr := l.ensureDirectoryExist(l.woxDataDirectory); directoryErr != nil {
		return directoryErr
	}

	l.userDataDirectoryShortcutPath = path.Join(l.woxDataDirectory, ".userdata.location")
	if _, statErr := os.Stat(l.userDataDirectoryShortcutPath); os.IsNotExist(statErr) {
		// shortcut file does not exist, create and write default data directory path to it
		file, createErr := os.Create(l.userDataDirectoryShortcutPath)
		if createErr != nil {
			return fmt.Errorf("failed to create shortcut file: %w", createErr)
		}
		defer file.Close()

		// write data directory path to file
		_, writeErr := file.WriteString(path.Join(l.woxDataDirectory, "wox-user"))
		if writeErr != nil {
			return fmt.Errorf("failed to write user data directory path to shortcut file: %w", writeErr)
		}
	}

	// read data directory path from shortcut file
	file, openErr := os.Open(l.userDataDirectoryShortcutPath)
	if openErr != nil {
		return fmt.Errorf("failed to open shortcut file: %w", openErr)
	}
	defer file.Close()

	// read data directory path from file
	readFile, readFileErr := os.ReadFile(l.userDataDirectoryShortcutPath)
	if readFileErr != nil {
		return fmt.Errorf("failed to read shortcut file: %w", readFileErr)
	}
	l.userDataDirectory = string(readFile)

	if directoryErr := l.ensureDirectoryExist(l.userDataDirectory); directoryErr != nil {
		return directoryErr
	}
	if directoryErr := l.ensureDirectoryExist(l.GetLogDirectory()); directoryErr != nil {
		return directoryErr
	}
	if directoryErr := l.ensureDirectoryExist(l.GetLogHostsDirectory()); directoryErr != nil {
		return directoryErr
	}
	if directoryErr := l.ensureDirectoryExist(l.GetLogPluginDirectory()); directoryErr != nil {
		return directoryErr
	}
	if directoryErr := l.ensureDirectoryExist(l.GetPluginDirectory()); directoryErr != nil {
		return directoryErr
	}
	if directoryErr := l.ensureDirectoryExist(l.GetHostDirectory()); directoryErr != nil {
		return directoryErr
	}

	return nil
}

func (l *Location) ensureDirectoryExist(directory string) error {
	if _, statErr := os.Stat(directory); os.IsNotExist(statErr) {
		mkdirErr := os.MkdirAll(directory, os.ModePerm)
		if mkdirErr != nil {
			return fmt.Errorf("failed to create directory [%s]: %w", directory, mkdirErr)
		}
	}

	return nil
}

func (l *Location) GetLogDirectory() string {
	return path.Join(l.woxDataDirectory, "log")
}

func (l *Location) GetLogPluginDirectory() string {
	return path.Join(l.GetLogDirectory(), "plugins")
}

func (l *Location) GetLogHostsDirectory() string {
	return path.Join(l.GetLogDirectory(), "hosts")
}

func (l *Location) GetPluginDirectory() string {
	return path.Join(l.userDataDirectory, "plugins")
}

func (l *Location) GetHostDirectory() string {
	return path.Join(l.woxDataDirectory, "hosts")
}
