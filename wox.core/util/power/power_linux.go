package power

import (
	"context"
	"time"
	"wox/util"

	"github.com/godbus/dbus/v5"
)

const clockGapInterval = 5 * time.Second

// startPlatformWatcher keeps a clock-gap fallback for systems without logind.
func startPlatformWatcher(ctx context.Context, notify func()) {
	util.Go(ctx, "watch for system wake", func() {
		watchLogind(ctx, notify)
		if ctx.Err() != nil {
			return
		}
		ticker := time.NewTicker(clockGapInterval)
		defer ticker.Stop()
		previous := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				if crossedSleepGap(previous, now) {
					notify()
				}
				previous = now
			}
		}
	})
}

func crossedSleepGap(previous, now time.Time) bool {
	return now.Round(0).Sub(previous.Round(0))-now.Sub(previous) > 3*time.Second
}

func watchLogind(ctx context.Context, notify func()) {
	conn, err := dbus.ConnectSystemBus(dbus.WithContext(ctx))
	if err != nil {
		util.GetLogger().Debug(ctx, "logind sleep watcher unavailable: "+err.Error())
		return
	}
	defer conn.Close()
	const service = "org.freedesktop.login1"
	const iface = "org.freedesktop.login1.Manager"
	const member = "PrepareForSleep"
	const ownerChanged = "NameOwnerChanged"
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchSender(service), dbus.WithMatchObjectPath("/org/freedesktop/login1"), dbus.WithMatchInterface(iface), dbus.WithMatchMember(member)); err != nil {
		util.GetLogger().Debug(ctx, "cannot subscribe to logind sleep signals: "+err.Error())
		return
	}
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember(ownerChanged), dbus.WithMatchArg(0, service)); err != nil {
		util.GetLogger().Debug(ctx, "cannot subscribe to logind availability: "+err.Error())
		return
	}
	// Installing the match does not imply login1 owns a name on this bus.
	var hasOwner bool
	if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, service).Store(&hasOwner); err != nil {
		return
	}
	var clock *time.Ticker
	var clockTicks <-chan time.Time
	var previous time.Time
	setClockFallback := func(enabled bool) {
		if enabled && clock == nil {
			previous = time.Now()
			clock = time.NewTicker(clockGapInterval)
			clockTicks = clock.C
		} else if !enabled && clock != nil {
			clock.Stop()
			clock, clockTicks = nil, nil
		}
	}
	setClockFallback(!hasOwner)
	defer setClockFallback(false)
	for {
		select {
		case <-conn.Context().Done():
			return
		case <-clockTicks:
			now := time.Now()
			if crossedSleepGap(previous, now) {
				notify()
			}
			previous = now
		case signal, ok := <-signals:
			if !ok {
				return
			}
			if signal.Name == iface+"."+member && len(signal.Body) == 1 && signal.Body[0] == false {
				notify()
			}
			if signal.Name == "org.freedesktop.DBus."+ownerChanged && len(signal.Body) == 3 {
				setClockFallback(signal.Body[2] == "")
			}
		}
	}
}
