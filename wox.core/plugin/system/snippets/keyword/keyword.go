// Package keyword defines the literal trigger characters supported by Snippets.
package keyword

// Valid accepts an optional keyword containing printable ASCII, including spaces.
func Valid(value string) bool {
	for _, c := range value {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}
