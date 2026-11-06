package keyboard

// StandaloneHoldSubscription owns a consuming key listener and can replay a tap.
type StandaloneHoldSubscription interface {
	RawKeySubscription
	ReplayTap(uint32) error
}

var standaloneHoldListenerPlatform func(Key, RawKeyHandler) (StandaloneHoldSubscription, bool, error)

// TryAddStandaloneHoldListener installs a platform-specific consuming listener when available.
func TryAddStandaloneHoldListener(key Key, handler RawKeyHandler) (StandaloneHoldSubscription, bool, error) {
	if standaloneHoldListenerPlatform == nil {
		return nil, false, nil
	}
	return standaloneHoldListenerPlatform(key, handler)
}
