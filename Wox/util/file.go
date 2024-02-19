package util

import "os"

// IsFileExecAny returns true if the file mode indicates that the file is executable by any user.
func IsFileExecAny(mode os.FileMode) bool {
	return mode&0111 != 0
}
