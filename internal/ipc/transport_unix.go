//go:build linux || darwin

package ipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// listenTransport creates a Unix socket listener for the given endpoint.
func listenTransport(endpoint string) (net.Listener, error) {
	path, err := unixSocketPath(endpoint)
	if err != nil {
		return nil, err
	}

	listener, listenErr := listenUnixSocket(path)
	if listenErr == nil {
		return listener, nil
	}
	if !errors.Is(listenErr, syscall.EADDRINUSE) {
		return nil, listenErr
	}

	// The socket path is already in use
	// There's a chance that no process is actually listening on it, so
	// we can try to recover by removing the stale socket

	recovered, recoverErr := recoverStaleUnixSocket(path)
	if recoverErr != nil {
		return nil, errors.Join(listenErr, recoverErr)
	}
	if !recovered {
		return nil, listenErr
	}

	return listenUnixSocket(path)
}

// dialTransport connects to a Unix socket at the given endpoint.
func dialTransport(ctx context.Context, endpoint string) (net.Conn, error) {
	path, err := unixSocketPath(endpoint)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unix", path)
}

func listenUnixSocket(path string) (net.Listener, error) {
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("secure IPC socket: %w", err)
	}
	return listener, nil
}

func recoverStaleUnixSocket(path string) (bool, error) {
	conn, err := net.DialTimeout("unix", path, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		return false, nil
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return false, nil
	}

	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return false, fmt.Errorf("refusing to remove non-socket IPC path %q", path)
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("remove stale IPC socket: %w", err)
	}
	return true, nil
}

func unixSocketPath(endpoint string) (string, error) {
	directory := os.Getenv("XDG_RUNTIME_DIR")
	if directory == "" {
		directory = filepath.Join(os.TempDir(), fmt.Sprintf("puush-%d", os.Geteuid()))
		if err := ensurePrivateDirectory(directory); err != nil {
			return "", err
		}
	}
	if err := ensureOwnedDirectory(directory); err != nil {
		return "", fmt.Errorf("invalid XDG_RUNTIME_DIR: %w", err)
	}
	return filepath.Join(directory, endpoint+".sock"), nil
}

func ensurePrivateDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create IPC directory: %w", err)
	}
	if err := ensureOwnedDirectory(path); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure IPC directory: %w", err)
	}
	return nil
}

func ensureOwnedDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("%q is not owned by the current user", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%q is accessible by other users", path)
	}
	return nil
}
