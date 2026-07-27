package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wox/test/automationdriver"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type repeatedFlag []string

func (values *repeatedFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *repeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run owns the launched process so every error path executes the deferred cleanup.
func run() error {
	var waitIDs repeatedFlag
	var setValues repeatedFlag
	var keys repeatedFlag
	binary := flag.String("binary", "./.tmp/wox-go-ui-smoke", "automation-enabled Wox binary")
	route := flag.String("route", "/general", "settings route passed to window.open_settings")
	capture := flag.String("capture", "", "absolute or working-directory-relative PNG output path")
	width := flag.Float64("width", 1152, "logical settings window width")
	height := flag.Float64("height", 768, "logical settings window height")
	activate := flag.String("activate", "", "automation ID to activate after the initial capture")
	activateCapture := flag.String("activate-capture", "", "PNG captured after activation")
	hover := flag.String("hover", "", "automation ID whose logical center receives a pointer move")
	hoverWaitID := flag.String("hover-wait-id", "", "automation ID required while hover is active")
	hoverCapture := flag.String("hover-capture", "", "PNG captured while hover is active")
	hoverExitCapture := flag.String("hover-exit-capture", "", "PNG captured after pointer leave")
	hoverStable := flag.Duration("hover-stable", 0, "duration that -hover-wait-id must remain present")
	dataDir := flag.String("data-dir", "", "optional isolated WOX_TEST_DATA_DIR")
	userDir := flag.String("user-dir", "", "optional isolated WOX_TEST_USER_DIR")
	settle := flag.Duration("settle", 500*time.Millisecond, "delay after state-changing actions")
	timeout := flag.Duration("timeout", 120*time.Second, "overall driver timeout")
	flag.Var(&waitIDs, "wait-id", "automation ID required before actions; repeat as needed")
	flag.Var(&setValues, "set-value", "set text as automationID=value; repeat as needed")
	flag.Var(&keys, "key", "portable key name such as arrow-down or enter; repeat as needed")
	flag.Parse()

	if strings.TrimSpace(*capture) == "" {
		return fmt.Errorf("-capture is required")
	}
	if *width <= 0 || *height <= 0 {
		return fmt.Errorf("-width and -height must be positive")
	}
	if strings.TrimSpace(*activateCapture) != "" && strings.TrimSpace(*activate) == "" {
		return fmt.Errorf("-activate-capture requires -activate")
	}
	if (strings.TrimSpace(*hoverWaitID) != "" || strings.TrimSpace(*hoverCapture) != "" || strings.TrimSpace(*hoverExitCapture) != "" || *hoverStable > 0) && strings.TrimSpace(*hover) == "" {
		return fmt.Errorf("hover options require -hover")
	}
	if *hoverStable > 0 && strings.TrimSpace(*hoverWaitID) == "" {
		return fmt.Errorf("-hover-stable requires -hover-wait-id")
	}

	binaryPath, err := absolutePath(*binary)
	if err != nil {
		return err
	}
	capturePath, err := prepareCapturePath(*capture)
	if err != nil {
		return err
	}
	activationCapturePath := ""
	if strings.TrimSpace(*activateCapture) != "" {
		activationCapturePath, err = prepareCapturePath(*activateCapture)
		if err != nil {
			return err
		}
	}
	hoverCapturePath := ""
	if strings.TrimSpace(*hoverCapture) != "" {
		hoverCapturePath, err = prepareCapturePath(*hoverCapture)
		if err != nil {
			return err
		}
	}
	hoverExitCapturePath := ""
	if strings.TrimSpace(*hoverExitCapture) != "" {
		hoverExitCapturePath, err = prepareCapturePath(*hoverExitCapture)
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	port, err := availablePort()
	if err != nil {
		return err
	}
	environment := []string{
		fmt.Sprintf("WOX_TEST_SERVER_PORT=%d", port),
		"WOX_TEST_DISABLE_TELEMETRY=true",
	}
	if strings.TrimSpace(*dataDir) != "" {
		absolute, resolveErr := absolutePath(*dataDir)
		if resolveErr != nil {
			return resolveErr
		}
		environment = append(environment, "WOX_TEST_DATA_DIR="+absolute)
	}
	if strings.TrimSpace(*userDir) != "" {
		absolute, resolveErr := absolutePath(*userDir)
		if resolveErr != nil {
			return resolveErr
		}
		environment = append(environment, "WOX_TEST_USER_DIR="+absolute)
	}
	process, err := automationdriver.Launch(ctx, binaryPath, automationdriver.LaunchOptions{
		Environment:    environment,
		StartupTimeout: min(*timeout, 60*time.Second),
	})
	if err != nil {
		if strings.Contains(err.Error(), "%!w(<nil>)") {
			return fmt.Errorf("Wox exited before automation became ready; verify the stopped boundary because another Wox or IDE/Delve session may own the single instance")
		}
		return fmt.Errorf("launch Wox: %w", err)
	}
	defer func() {
		if err := process.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close Wox automation process: %v\n", err)
		}
	}()

	if err := process.Client.OpenSettings(ctx, *route); err != nil {
		return fmt.Errorf("open settings %q: %w", *route, err)
	}
	requiredIDs := append([]string{"settings.window"}, waitIDs...)
	if _, err := process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, id := range requiredIDs {
			if _, found := automationdriver.Find(snapshot, id); !found {
				return false
			}
		}
		return true
	}); err != nil {
		return fmt.Errorf("wait for %s: %w", strings.Join(requiredIDs, ", "), err)
	}

	bounds, err := process.Client.Bounds(ctx)
	if err != nil {
		return fmt.Errorf("read settings bounds: %w", err)
	}
	bounds.Width = float32(*width)
	bounds.Height = float32(*height)
	if err := process.Client.SetBounds(ctx, bounds); err != nil {
		return fmt.Errorf("set settings bounds: %w", err)
	}
	for _, assignment := range setValues {
		id, value, found := strings.Cut(assignment, "=")
		if !found || strings.TrimSpace(id) == "" {
			return fmt.Errorf("invalid -set-value %q; expected automationID=value", assignment)
		}
		if err := process.Client.Perform(ctx, id, woxui.AccessibilityActionSetValue, value); err != nil {
			return fmt.Errorf("set %s: %w", id, err)
		}
	}
	for _, key := range keys {
		if err := process.Client.PressKey(ctx, woxui.Key(strings.TrimSpace(strings.ToLower(key))), 0); err != nil {
			return fmt.Errorf("press %s: %w", key, err)
		}
	}
	time.Sleep(*settle)
	if err := printSemantics(ctx, process.Client); err != nil {
		return err
	}
	if err := process.Client.Capture(ctx, capturePath); err != nil {
		return fmt.Errorf("capture %s: %w", capturePath, err)
	}
	fmt.Printf("captured %s\n", capturePath)

	if strings.TrimSpace(*activate) != "" {
		if err := process.Client.Perform(ctx, *activate, woxui.AccessibilityActionActivate, ""); err != nil {
			return fmt.Errorf("activate %s: %w", *activate, err)
		}
		time.Sleep(*settle)
		if activationCapturePath != "" {
			if err := process.Client.Capture(ctx, activationCapturePath); err != nil {
				return fmt.Errorf("capture activated state %s: %w", activationCapturePath, err)
			}
			fmt.Printf("captured %s\n", activationCapturePath)
		}
	}
	if strings.TrimSpace(*hover) != "" {
		node, err := process.Client.MovePointerTo(ctx, *hover)
		if err != nil {
			return fmt.Errorf("hover %s: %w", *hover, err)
		}
		fmt.Printf("hovered %s at %+v\n", *hover, node.Bounds)
		if strings.TrimSpace(*hoverWaitID) != "" {
			if err := waitForAutomationID(ctx, process.Client, *hoverWaitID, true); err != nil {
				return err
			}
		}
		time.Sleep(*settle)
		if *hoverStable > 0 {
			if err := assertAutomationIDStable(ctx, process.Client, *hoverWaitID, *hoverStable); err != nil {
				return err
			}
		}
		if hoverCapturePath != "" {
			if err := process.Client.Capture(ctx, hoverCapturePath); err != nil {
				return fmt.Errorf("capture hover state %s: %w", hoverCapturePath, err)
			}
			fmt.Printf("captured %s\n", hoverCapturePath)
		}
		if err := process.Client.LeavePointer(ctx); err != nil {
			return fmt.Errorf("leave pointer: %w", err)
		}
		if strings.TrimSpace(*hoverWaitID) != "" {
			if err := waitForAutomationID(ctx, process.Client, *hoverWaitID, false); err != nil {
				return err
			}
		} else {
			time.Sleep(*settle)
		}
		if hoverExitCapturePath != "" {
			if err := process.Client.Capture(ctx, hoverExitCapturePath); err != nil {
				return fmt.Errorf("capture hover exit state %s: %w", hoverExitCapturePath, err)
			}
			fmt.Printf("captured %s\n", hoverExitCapturePath)
		}
	}
	return nil
}

// waitForAutomationID waits until one semantic node reaches the expected presence state.
func waitForAutomationID(ctx context.Context, client *automationdriver.Client, automationID string, present bool) error {
	_, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, automationID)
		return found == present
	})
	if err != nil {
		return fmt.Errorf("wait for %s present=%t: %w", automationID, present, err)
	}
	return nil
}

// assertAutomationIDStable samples semantics while the pointer remains stationary.
func assertAutomationIDStable(ctx context.Context, client *automationdriver.Client, automationID string, duration time.Duration) error {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			return fmt.Errorf("read stable hover semantics: %w", err)
		}
		if _, found := automationdriver.Find(snapshot, automationID); !found {
			return fmt.Errorf("automation node %q disappeared during %s stable hover", automationID, duration)
		}
		timer := time.NewTimer(min(50*time.Millisecond, time.Until(deadline)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

// printSemantics exposes stable logical bounds and roles for parity measurements.
func printSemantics(ctx context.Context, client *automationdriver.Client) error {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("read semantics: %w", err)
	}
	for _, node := range snapshot.Tree.Nodes {
		if node.AutomationID == "" {
			continue
		}
		fmt.Printf("id=%q role=%q label=%q value=%q bounds=%+v\n", node.AutomationID, node.Role, node.Label, node.Value, node.Bounds)
	}
	return nil
}

// availablePort reserves and releases a loopback port for Wox's test server.
func availablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate test server port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// absolutePath normalizes paths before passing them across the process boundary.
func absolutePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}
	return absolute, nil
}

// prepareCapturePath validates PNG output and creates its parent directory.
func prepareCapturePath(path string) (string, error) {
	absolute, err := absolutePath(path)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(absolute)) != ".png" {
		return "", fmt.Errorf("capture path must end in .png: %s", absolute)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", fmt.Errorf("create capture directory: %w", err)
	}
	return absolute, nil
}
