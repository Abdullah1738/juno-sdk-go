package junocashd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultUserAgent = "juno-sdk-go/0"
	rpcVersion       = "1.0"
)

type Client struct {
	endpoint  string
	username  string
	password  string
	userAgent string
	http      *http.Client

	nextID atomic.Uint64
}

type Option func(*Client)

func WithHTTPClient(c *http.Client) Option {
	return func(cli *Client) {
		if c != nil {
			cli.http = c
		}
	}
}

func WithUserAgent(ua string) Option {
	return func(cli *Client) {
		ua = strings.TrimSpace(ua)
		if ua != "" {
			cli.userAgent = ua
		}
	}
}

func New(endpoint, username, password string, opts ...Option) *Client {
	c := &Client{
		endpoint:  strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		username:  username,
		password:  password,
		userAgent: defaultUserAgent,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
	ID     uint64          `json:"id"`
}

func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(method) == "" {
		return errors.New("junocashd: method is required")
	}
	if c.endpoint == "" {
		return errors.New("junocashd: endpoint is required")
	}
	if c.http == nil {
		return errors.New("junocashd: http client is nil")
	}

	id := c.nextID.Add(1)
	if params == nil {
		params = []any{}
	}
	reqBody, err := json.Marshal(rpcRequest{
		JSONRPC: rpcVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("junocashd: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("junocashd: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("junocashd: request: %w", err)
	}
	defer resp.Body.Close()

	const maxBodyBytes = 8 << 20 // 8 MiB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("junocashd: read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err == nil && rpcResp.Error != nil {
		return rpcResp.Error
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("junocashd: unmarshal response: %w", err)
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	if out == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 || bytes.Equal(rpcResp.Result, []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, out); err != nil {
		return fmt.Errorf("junocashd: unmarshal result: %w", err)
	}
	return nil
}
