package automationdriver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/ui/automation"
)

// LaunchOptions configures one isolated Wox automation process.
type LaunchOptions struct {
	Args           []string
	Environment    []string
	StartupTimeout time.Duration
}

// Process owns a Wox automation process and its driver client.
type Process struct {
	Client *Client

	command  *exec.Cmd
	infoFile string
	wait     chan error
	close    sync.Once
}

// Launch starts a wox_automation binary and waits for authenticated endpoint metadata.
func Launch(ctx context.Context, executable string, options LaunchOptions) (*Process, error) {
	if strings.TrimSpace(executable) == "" {
		return nil, fmt.Errorf("automation executable is required")
	}
	startupTimeout := options.StartupTimeout
	if startupTimeout <= 0 {
		startupTimeout = 30 * time.Second
	}
	infoDirectory, err := os.MkdirTemp("", "wox-automation-")
	if err != nil {
		return nil, err
	}
	infoFile := filepath.Join(infoDirectory, "endpoint.json")
	token, err := processToken()
	if err != nil {
		_ = os.RemoveAll(infoDirectory)
		return nil, err
	}

	command := exec.CommandContext(ctx, executable, options.Args...)
	command.Env = mergeEnvironment(os.Environ(), append(options.Environment,
		"WOX_AUTOMATION_TOKEN="+token,
		"WOX_AUTOMATION_INFO_FILE="+infoFile,
	))
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	configureProcess(command)
	if err := command.Start(); err != nil {
		_ = os.RemoveAll(infoDirectory)
		return nil, fmt.Errorf("start Wox automation process: %w", err)
	}

	process := &Process{command: command, infoFile: infoFile, wait: make(chan error, 1)}
	go func() {
		process.wait <- command.Wait()
	}()
	startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	infoResult := make(chan struct {
		info automation.Info
		err  error
	}, 1)
	go func() {
		info, readErr := ReadInfo(startupCtx, infoFile)
		infoResult <- struct {
			info automation.Info
			err  error
		}{info: info, err: readErr}
	}()
	select {
	case waitErr := <-process.wait:
		_ = os.RemoveAll(infoDirectory)
		return nil, fmt.Errorf("Wox exited before automation was ready: %w", waitErr)
	case result := <-infoResult:
		if result.err != nil {
			_ = process.Close()
			return nil, fmt.Errorf("wait for Wox automation endpoint: %w", result.err)
		}
		client, clientErr := NewClient(result.info)
		if clientErr != nil {
			_ = process.Close()
			return nil, clientErr
		}
		process.Client = client
		return process, nil
	}
}

// Close terminates the isolated Wox process and removes endpoint metadata.
func (p *Process) Close() error {
	if p == nil {
		return nil
	}
	var closeErr error
	p.close.Do(func() {
		if p.command != nil && p.command.Process != nil {
			closeErr = terminateProcess(p.command)
		}
		select {
		case <-p.wait:
		case <-time.After(5 * time.Second):
		}
		_ = os.RemoveAll(filepath.Dir(p.infoFile))
	})
	return closeErr
}

func processToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func mergeEnvironment(base, overrides []string) []string {
	values := map[string]string{}
	order := []string{}
	for _, entry := range append(append([]string(nil), base...), overrides...) {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = value
	}
	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, key+"="+values[key])
	}
	return result
}
