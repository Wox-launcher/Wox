//go:build !windows

package supervisor

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

func defaultControlAddress() string {
	return filepath.Join(dataDirectory(), "supervisor", "control.sock")
}

func dialControl() (io.ReadWriteCloser, error) {
	return net.DialTimeout("unix", controlAddress(), 500*time.Millisecond)
}

func listenControl() (controlListener, error) {
	address := controlAddress()
	if err := os.MkdirAll(filepath.Dir(address), 0755); err != nil {
		return nil, err
	}
	// Each supervisor has a unique address. Never unlink a socket another live
	// supervisor may own during restart handoff.
	listener, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(address, 0600)
	return listener, nil
}

type controlListener interface {
	Accept() (net.Conn, error)
	Close() error
}
