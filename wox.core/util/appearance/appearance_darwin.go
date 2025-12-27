package appearance

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

bool isDark() {
    NSString *style = [[NSUserDefaults standardUserDefaults] stringForKey:@"AppleInterfaceStyle"];
    return [style isEqualToString:@"Dark"];
}
*/
import "C"

import (
	"time"
)

var (
	callback      func(isDark bool)
	stopChan      chan struct{}
	lastIsDark    bool
	checkInterval = time.Second
)

func isDark() bool {
	return bool(C.isDark())
}

func watchSystemAppearance(cb func(isDark bool)) {
	callback = cb
	lastIsDark = isDark()
	stopChan = make(chan struct{})

	// Start a goroutine to periodically check for appearance changes
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentIsDark := isDark()
				if currentIsDark != lastIsDark {
					lastIsDark = currentIsDark
					if callback != nil {
						callback(currentIsDark)
					}
				}
			case <-stopChan:
				return
			}
		}
	}()
}

func stopWatching() {
	close(stopChan)
	callback = nil
}
