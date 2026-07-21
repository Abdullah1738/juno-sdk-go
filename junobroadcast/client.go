package junobroadcast

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client

	pollInterval time.Duration
}

type Option func(*Client)

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

func WithPollInterval(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.pollInterval = d
		}
	}
}

func New(baseURL string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("junobroadcast: base url required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("junobroadcast: invalid base url %q", baseURL)
	}

	c := &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		pollInterval: 500 * time.Millisecond,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c, nil
}

type APIError struct {
	StatusCode int
	Status     string
	Code       string
	Message    string
	Retryable  bool
	Details    json.RawMessage
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return "junobroadcast: api error <nil>"
	}
	msg := strings.TrimSpace(e.Message)
	code := strings.TrimSpace(e.Code)
	switch {
	case code != "" && msg != "":
		return fmt.Sprintf("junobroadcast: %s: %s", code, msg)
	case code != "":
		return fmt.Sprintf("junobroadcast: %s", code)
	default:
		if msg == "" {
			return fmt.Sprintf("junobroadcast: http %d", e.StatusCode)
		}
		return fmt.Sprintf("junobroadcast: http %d: %s", e.StatusCode, msg)
	}
}

// Temporary reports whether the failed HTTP request can reasonably be
// retried without changing it.
func (e *APIError) Temporary() bool {
	return e != nil && (e.Retryable || e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500)
}

type HealthResponse struct {
	Status string `json:"status"`
}

func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	var resp HealthResponse
	if err := c.doJSON(ctx, http.MethodGet, "/healthz", nil, &resp); err != nil {
		return HealthResponse{}, err
	}
	return resp, nil
}

type SubmitRequest struct {
	RawTxHex          string `json:"raw_tx_hex"`
	WaitConfirmations *int64 `json:"wait_confirmations,omitempty"`
}

type TxStatus struct {
	TxID          string `json:"txid"`
	InMempool     bool   `json:"in_mempool"`
	Confirmations int64  `json:"confirmations"`
	BlockHash     string `json:"blockhash,omitempty"`
}

type SubmitResponse struct {
	TxID   string    `json:"txid"`
	Status *TxStatus `json:"status,omitempty"`
}

func (c *Client) Submit(ctx context.Context, rawTxHex string, waitConfirmations *int64) (SubmitResponse, error) {
	rawTxHex = strings.TrimSpace(rawTxHex)
	if rawTxHex == "" {
		return SubmitResponse{}, errors.New("junobroadcast: raw_tx_hex required")
	}

	var resp SubmitResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/tx/submit", SubmitRequest{
		RawTxHex:          rawTxHex,
		WaitConfirmations: waitConfirmations,
	}, &resp); err != nil {
		return SubmitResponse{}, err
	}
	if strings.TrimSpace(resp.TxID) == "" {
		return SubmitResponse{}, errors.New("junobroadcast: invalid response")
	}
	return resp, nil
}

func (c *Client) Status(ctx context.Context, txid string) (TxStatus, bool, error) {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid == "" {
		return TxStatus{}, false, errors.New("junobroadcast: txid required")
	}

	var st TxStatus
	if err := c.doJSON(ctx, http.MethodGet, "/v1/tx/"+url.PathEscape(txid), nil, &st); err != nil {
		var ae *APIError
		if errors.As(err, &ae) && ae.StatusCode == http.StatusNotFound {
			return TxStatus{}, false, nil
		}
		return TxStatus{}, false, err
	}
	return st, true, nil
}

func (c *Client) WaitForConfirmations(ctx context.Context, txid string, confirmations int64) (TxStatus, error) {
	if confirmations < 0 {
		return TxStatus{}, errors.New("junobroadcast: confirmations must be >= 0")
	}

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		st, found, err := c.Status(ctx, txid)
		if err != nil {
			return TxStatus{}, err
		}
		if found && st.Confirmations >= confirmations {
			return st, nil
		}

		select {
		case <-ctx.Done():
			return TxStatus{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return errors.New("junobroadcast: marshal request")
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return errors.New("junobroadcast: build request")
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	const maxBody = 1 << 20
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if readErr != nil {
		return fmt.Errorf("junobroadcast: read response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var er struct {
			Error struct {
				Code      string          `json:"code"`
				Message   string          `json:"message"`
				Retryable bool            `json:"retryable"`
				Details   json.RawMessage `json:"details"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &er) == nil && strings.TrimSpace(er.Error.Code) != "" {
			return &APIError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Code:       strings.TrimSpace(er.Error.Code),
				Message:    strings.TrimSpace(er.Error.Message),
				Retryable:  er.Error.Retryable,
				Details:    er.Error.Details,
				Body:       string(raw),
			}
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Message:    strings.TrimSpace(string(raw)),
			Body:       string(raw),
		}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("junobroadcast: invalid json response")
	}
	return nil
}
