package dictation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"wox/setting/definition"
	"wox/util/speech"
)

func TestBuildInputDeviceOptionsIncludesUnavailableSelectedDevice(t *testing.T) {
	options := buildInputDeviceOptions(context.Background(), "missing-device", "DJI MIC", []speech.AudioDevice{
		{ID: "built-in", Name: "Studio Display Microphone"},
	})

	option, ok := findInputDeviceOption(options, "missing-device")
	if !ok {
		t.Fatalf("expected missing selected device to remain in options: %#v", options)
	}
	if !strings.Contains(option.Label, "DJI MIC") {
		t.Fatalf("expected unavailable option label to use saved device name, got %q", option.Label)
	}
	if option.Value == inputDeviceSystem {
		t.Fatalf("missing selected device must not be represented as system default")
	}
}

func TestBuildInputDeviceOptionsDoesNotInsertUnavailableWhenSelectedDeviceExists(t *testing.T) {
	options := buildInputDeviceOptions(context.Background(), "dji-device", "DJI MIC", []speech.AudioDevice{
		{ID: "dji-device", Name: "DJI MIC"},
		{ID: "built-in", Name: "Studio Display Microphone"},
	})

	var count int
	for _, option := range options {
		if option.Value == "dji-device" {
			count++
			if option.Label != "DJI MIC" {
				t.Fatalf("expected existing device label to come from current device list, got %q", option.Label)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one option for existing selected device, got %d in %#v", count, options)
	}
}

func TestResolveInputDeviceForStartSkipsEnumerationForSystemDefault(t *testing.T) {
	restore := stubListCaptureDevices(t, func(context.Context) ([]speech.AudioDevice, error) {
		t.Fatal("system default should not enumerate capture devices")
		return nil, nil
	})
	defer restore()

	deviceID, deviceName, err := resolveInputDeviceForStart(context.Background(), "", "DJI MIC")
	if err != nil {
		t.Fatalf("expected empty device to resolve to system default without error, got %v", err)
	}
	if deviceID != inputDeviceSystem {
		t.Fatalf("expected empty device to resolve to %q, got %q", inputDeviceSystem, deviceID)
	}
	if deviceName != "" {
		t.Fatalf("expected system default to return empty display name, got %q", deviceName)
	}

	deviceID, deviceName, err = resolveInputDeviceForStart(context.Background(), inputDeviceSystem, "DJI MIC")
	if err != nil {
		t.Fatalf("expected explicit system device to resolve without error, got %v", err)
	}
	if deviceID != inputDeviceSystem {
		t.Fatalf("expected explicit system device to stay %q, got %q", inputDeviceSystem, deviceID)
	}
	if deviceName != "" {
		t.Fatalf("expected explicit system default to return empty display name, got %q", deviceName)
	}
}

func TestResolveInputDeviceForStartAllowsExistingSpecificDevice(t *testing.T) {
	restore := stubListCaptureDevices(t, func(context.Context) ([]speech.AudioDevice, error) {
		return []speech.AudioDevice{
			{ID: "dji-device", Name: "DJI MIC"},
			{ID: "built-in", Name: "Studio Display Microphone"},
		}, nil
	})
	defer restore()

	deviceID, deviceName, err := resolveInputDeviceForStart(context.Background(), "dji-device", "")
	if err != nil {
		t.Fatalf("expected existing device to resolve without error, got %v", err)
	}
	if deviceID != "dji-device" {
		t.Fatalf("expected selected device id, got %q", deviceID)
	}
	if deviceName != "DJI MIC" {
		t.Fatalf("expected resolved device name, got %q", deviceName)
	}
}

func TestResolveInputDeviceForStartRejectsMissingSpecificDevice(t *testing.T) {
	restore := stubListCaptureDevices(t, func(context.Context) ([]speech.AudioDevice, error) {
		return []speech.AudioDevice{
			{ID: "built-in", Name: "Studio Display Microphone"},
		}, nil
	})
	defer restore()

	deviceID, deviceName, err := resolveInputDeviceForStart(context.Background(), "missing-device", "DJI MIC")
	if !errors.Is(err, errInputDeviceMissing) {
		t.Fatalf("expected missing device error, got %v", err)
	}
	if deviceID != "missing-device" {
		t.Fatalf("expected original selected device id, got %q", deviceID)
	}
	if deviceName != "DJI MIC" {
		t.Fatalf("expected saved device name, got %q", deviceName)
	}
}

func findInputDeviceOption(options []definition.PluginSettingValueSelectOption, value string) (definition.PluginSettingValueSelectOption, bool) {
	for _, option := range options {
		if option.Value == value {
			return option, true
		}
	}
	return definition.PluginSettingValueSelectOption{}, false
}

func stubListCaptureDevices(t *testing.T, fn func(context.Context) ([]speech.AudioDevice, error)) func() {
	t.Helper()
	original := listCaptureDevices
	listCaptureDevices = fn
	return func() {
		listCaptureDevices = original
	}
}
