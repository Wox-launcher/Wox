// Package dateformat defines the date fields shared by snippet expansion and import previews.
package dateformat

// Supported reports whether every ASCII letter in a format is a supported date field.
func Supported(format string) bool {
	for i := 0; i < len(format); {
		c := format[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			i++
			continue
		}
		end := i + 1
		for end < len(format) && format[end] == c {
			end++
		}
		switch format[i:end] {
		case "yyyy", "MM", "dd", "HH", "mm", "ss":
		default:
			return false
		}
		i = end
	}
	return true
}
