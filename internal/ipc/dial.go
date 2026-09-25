package ipc

import (
	"context"
	"net/rpc"
	"time"
)

// dialAttempt reports whether an existing IPC server was reached.
// If connected is false, err describes why the connection failed.
// If connected is true, err contains any error returned by the server.
type dialAttempt struct {
	connected bool
	err       error
}

func tryDial(ctx context.Context, endpoint string, command Command) dialAttempt {
	conn, err := dialTransport(ctx, endpoint)
	if err != nil {
		return dialAttempt{err: err}
	}

	// Use deadline from context if available, otherwise use a
	// default timeout to avoid hanging connections.

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(connectionTimeout))
	}

	client := rpc.NewClient(conn)
	defer client.Close()

	request := IPCRequest{ProtocolVersion: protocolVersion, Command: command}
	err = client.Call(rpcServiceName+".Enqueue", request, new(IPCResponse))
	return dialAttempt{connected: true, err: err}
}
