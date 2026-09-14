//go:build !darwin

package clipboard

import "image"

// readImage keeps existing callers on the same decoder without changing their size policy.
func readImage() (image.Image, error) {
	snapshot, err := readImageSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.Decode(0)
}
