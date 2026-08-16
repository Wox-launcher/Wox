package shell

import (
	"errors"
	"testing"
)

func TestIsElevationCancelled(t *testing.T) {
	if !IsElevationCancelled(ErrElevationCancelled) {
		t.Fatal("expected elevation cancelled error")
	}
	if IsElevationCancelled(errors.New("other")) {
		t.Fatal("did not expect a generic error to be treated as cancelled elevation")
	}
}
