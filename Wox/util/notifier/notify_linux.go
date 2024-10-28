package notifier

import (
	"github.com/godbus/dbus/v5"
)

func ShowNotification(message string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "Wox", uint32(0),
		"", "Wox", message, []string{}, map[string]dbus.Variant{}, int32(5000))
	if call.Err != nil {
		return
	}
}
