package ipc

import "fmt"

const (
	protocolVersion = 1
	rpcServiceName  = "Puush"
)

// IPCRequest is the request sent by a secondary process to the primary process.
type IPCRequest struct {
	ProtocolVersion int
	Command         Command
}

// IPCResponse is the response sent by the primary process to a secondary process.
// net/rpc requires us to have a response type, even if we don't use it.
type IPCResponse struct{}

// Enqueue validates a command & places it in the IPC inbox.
func (s *Server) Enqueue(request IPCRequest, _ *IPCResponse) error {
	if request.ProtocolVersion != protocolVersion {
		return fmt.Errorf("unsupported IPC protocol version %d", request.ProtocolVersion)
	}
	return s.enqueue(request.Command)
}
