package audio

// VolumeDucker lowers the system output volume during dictation and restores
// it when dictation ends. This prevents other audio (music, videos) from
// interfering with the user's speech during dictation.
//
// Platform implementations:
//   - macOS: osascript to get/set output volume
//   - Windows: IAudioEndpointVolume COM API
//   - Linux: pactl set-sink-volume

// VolumeDucker manages the duck/restore lifecycle.
type VolumeDucker struct {
	originalVolume int // 0-100, the volume before ducking
	ducked         bool
}

// NewVolumeDucker creates a new VolumeDucker.
func NewVolumeDucker() *VolumeDucker {
	return &VolumeDucker{}
}

// Duck lowers the system output volume, remembering the original volume for
// later restoration. The volume is lowered to a fraction of the original
// (e.g. 0.3 means 30% of the original volume), with a minimum of 5. If already
// ducked it is a no-op.
func (v *VolumeDucker) Duck(ratio float64) error {
	if v.ducked {
		return nil
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	orig, err := getSystemVolume()
	if err != nil {
		return err
	}
	v.originalVolume = orig

	// Calculate target as a fraction of the original volume, with a floor
	// of 5 so the user can still faintly hear if the original is very low.
	target := int(float64(orig) * ratio)
	if target < 5 {
		target = 5
	}

	// Only duck if the target is lower than the current volume.
	if orig > target {
		if err := setSystemVolume(target); err != nil {
			return err
		}
	}
	v.ducked = true
	return nil
}

// Restore returns the system output volume to its original value. If not
// ducked it is a no-op.
func (v *VolumeDucker) Restore() error {
	if !v.ducked {
		return nil
	}
	v.ducked = false
	return setSystemVolume(v.originalVolume)
}
