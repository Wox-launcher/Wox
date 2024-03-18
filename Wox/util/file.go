package util

import "os"

// IsFileExecAny returns true if the file mode indicates that the file is executable by any user.
func IsFileExecAny(mode os.FileMode) bool {
	return mode&0111 != 0
}

func IsFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDirExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

func ListDir(path string) ([]string, error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, file := range dir {
		files = append(files, file.Name())
	}

	return files, nil
}
