package ipc

import (
	"context"
	"net"
)

const defaultEndpoint = "yeet-ee.tupsujumal.yeet"

// Role indicates how this process handled the IPC command.
// That is, forwarded it to an existing process or became the primary process.
type Role uint8

const (
	RolePrimary Role = iota + 1
	RoleForwarded
)

// OpenResult indicates whether this process became primary or forwarded its command.
type OpenResult struct {
	Role   Role
	Server *Server
}

type endpointResult struct {
	role     Role
	listener net.Listener
}

// Open forwards command to an existing process when possible.
// Otherwise it claims the application endpoint for the primary process.
func Open(ctx context.Context, command Command) (OpenResult, error) {
	return openAt(ctx, defaultEndpoint, command)
}

func openAt(ctx context.Context, endpoint string, command Command) (OpenResult, error) {
	if err := command.Validate(); err != nil {
		return OpenResult{}, err
	}

	// Attempt to claim the endpoint for this process, or forward the command to an existing process
	endpointState, err := dialOrClaim(ctx, endpoint, command)
	if err != nil {
		return OpenResult{Role: endpointState.role}, err
	}
	if endpointState.role == RoleForwarded {
		// We have successfully forwarded the command to an existing process
		return OpenResult{Role: RoleForwarded}, nil
	}

	server, err := newServer()
	if err != nil {
		endpointState.listener.Close()
		return OpenResult{}, err
	}
	server.listener = endpointState.listener
	go server.serve()

	// Enqueue the command before launching the ipc processor
	if command.Action != ActionAttention {
		if err := server.enqueue(command); err != nil {
			server.Close()
			return OpenResult{}, err
		}
	}

	return OpenResult{Role: RolePrimary, Server: server}, nil
}

func dialOrClaim(ctx context.Context, endpoint string, command Command) (endpointResult, error) {
	attempt := tryDial(ctx, endpoint, command)
	if attempt.connected {
		return endpointResult{role: RoleForwarded}, attempt.err
	}
	if err := ctx.Err(); err != nil {
		return endpointResult{}, err
	}

	listener, err := listenTransport(endpoint)
	if err != nil {
		return endpointResult{}, err
	}

	return endpointResult{role: RolePrimary, listener: listener}, nil
}
