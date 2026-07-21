package junocashd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// HTTPError reports an HTTP-layer failure that did not contain a JSON-RPC
// error. JSON-RPC failures are returned as *RPCError even when the daemon also
// uses a non-2xx HTTP status.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "junocashd: http error <nil>"
	}
	message := strings.TrimSpace(e.Body)
	if message == "" {
		message = strings.TrimSpace(e.Status)
	}
	if message == "" {
		return fmt.Sprintf("junocashd: http %d", e.StatusCode)
	}
	return fmt.Sprintf("junocashd: http %d: %s", e.StatusCode, message)
}

// Temporary reports whether the HTTP status normally permits a retry.
func (e *HTTPError) Temporary() bool {
	return e != nil && (e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500)
}

// IsRPCErrorCode reports whether err contains a daemon RPC error with code.
func IsRPCErrorCode(err error, code int) bool {
	var rpcErr *RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == code
}

func (e *RPCError) Error() string {
	if e == nil {
		return "junocashd: rpc error <nil>"
	}
	if e.Message == "" {
		return fmt.Sprintf("junocashd: rpc error %d", e.Code)
	}
	return fmt.Sprintf("junocashd: rpc error %d: %s", e.Code, e.Message)
}
