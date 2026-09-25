//go:build windows

package ipc

import (
	"context"
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

const (
	pipePrefix = `\\.\pipe\`

	// pipeSecurityDescriptorTemplate is a Windows Security Descriptor Definition Language (SDDL)
	// string that restricts access to the named pipe
	// Whoever came up with this shit should be investigated for mental health issues
	//
	// D:P             creates a protected discretionary access-control list
	// (A;;GA;;;SY)    allows LocalSystem full access
	// (A;;GA;;;BA)    allows built-in administrators full access
	// (A;;GA;;;%s)    allows the current user full access
	//
	// Each access-control entry uses the following SDDL layout:
	// (type;flags;rights;object-guid;inherit-object-guid;account-sid)
	// Here, `A` means "allow" and `GA` means "generic all" (full access)
	//
	// References:
	// https://learn.microsoft.com/windows/win32/secauthz/security-descriptor-string-format
	// https://learn.microsoft.com/windows/win32/secauthz/ace-strings
	// https://learn.microsoft.com/windows/win32/secauthz/sid-strings
	pipeSecurityDescriptorTemplate = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;%s)"
)

// listenTransport creates a named pipe listener for the given endpoint.
func listenTransport(endpoint string) (net.Listener, error) {
	descriptor, err := currentUserSecurityDescriptor()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(pipePrefix+endpoint, &winio.PipeConfig{
		SecurityDescriptor: descriptor,
		InputBufferSize:    requestLimit,
		OutputBufferSize:   requestLimit,
	})
}

// dialTransport connects to a named pipe at the given endpoint.
func dialTransport(ctx context.Context, endpoint string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, pipePrefix+endpoint)
}

func currentUserSecurityDescriptor() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("get current windows user: %w", err)
	}
	sid := user.User.Sid.String()
	return fmt.Sprintf(pipeSecurityDescriptorTemplate, sid), nil
}
