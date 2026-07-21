package junoscan

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

const (
	defaultHTTPTimeout              = 15 * time.Second
	orchardWitnessHTTPTimeoutBudget = 65 * time.Second
)

type Client struct {
	baseURL                  string
	bearerToken              string
	httpClient               *http.Client
	orchardWitnessHTTPClient *http.Client
}

type Option func(*Client)

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithBearerToken configures the token sent to authenticated juno-scan APIs.
// An empty token leaves the Authorization header unset.
func WithBearerToken(token string) Option {
	return func(c *Client) {
		c.bearerToken = strings.TrimSpace(token)
	}
}

func New(baseURL string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("junoscan: base url required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("junoscan: invalid base url %q", baseURL)
	}

	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	c.orchardWitnessHTTPClient = withMinimumTimeout(c.httpClient, orchardWitnessHTTPTimeoutBudget)
	return c, nil
}

type HTTPError struct {
	StatusCode int
	Status     string
	Code       string
	Message    string
	Retryable  bool
	Details    json.RawMessage
	Body       string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "junoscan: http error <nil>"
	}
	code := strings.TrimSpace(e.Code)
	message := strings.TrimSpace(e.Message)
	if code != "" && message != "" {
		return fmt.Sprintf("junoscan: http %d: %s: %s", e.StatusCode, code, message)
	}
	if code != "" {
		return fmt.Sprintf("junoscan: http %d: %s", e.StatusCode, code)
	}
	if message == "" {
		message = strings.TrimSpace(e.Body)
	}
	if message == "" {
		message = strings.TrimSpace(e.Status)
	}
	if message == "" {
		return fmt.Sprintf("junoscan: http %d", e.StatusCode)
	}
	return fmt.Sprintf("junoscan: http %d: %s", e.StatusCode, message)
}

// Temporary reports whether retrying the request may succeed without changing
// it. Explicit upstream metadata takes precedence; throttling and 5xx errors
// are otherwise treated as temporary.
func (e *HTTPError) Temporary() bool {
	if e == nil {
		return false
	}
	return e.Retryable || e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	var resp HealthResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/health", nil, &resp); err != nil {
		return HealthResponse{}, err
	}
	return resp, nil
}

func (c *Client) UpsertWallet(ctx context.Context, walletID, ufvk string) error {
	walletID = strings.TrimSpace(walletID)
	ufvk = strings.TrimSpace(ufvk)
	if walletID == "" || ufvk == "" {
		return errors.New("junoscan: wallet_id and ufvk required")
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/wallets", walletRequest{WalletID: walletID, UFVK: ufvk}, &resp); err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(resp.Status)) != "ok" {
		return errors.New("junoscan: unexpected response")
	}
	return nil
}

func (c *Client) ListWallets(ctx context.Context) ([]Wallet, error) {
	var resp struct {
		Wallets []Wallet `json:"wallets"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v1/wallets", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Wallets, nil
}

func (c *Client) ListWalletEvents(ctx context.Context, walletID string, cursor int64, limit int) (WalletEventsPage, error) {
	walletID = strings.TrimSpace(walletID)
	if walletID == "" {
		return WalletEventsPage{}, errors.New("junoscan: wallet_id required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	path := fmt.Sprintf("/v1/wallets/%s/events?cursor=%d&limit=%d", url.PathEscape(walletID), cursor, limit)
	var resp WalletEventsPage
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return WalletEventsPage{}, err
	}
	return resp, nil
}

func (c *Client) ListWalletNotes(ctx context.Context, walletID string, onlyUnspent bool) ([]WalletNote, error) {
	page, err := c.ListWalletNotesPage(ctx, walletID, ListWalletNotesOptions{
		OnlyUnspent: onlyUnspent,
		Direction:   "incoming",
		Limit:       1000,
	})
	if err != nil {
		return nil, err
	}
	return page.Notes, nil
}

func (c *Client) ListWalletNotesPage(ctx context.Context, walletID string, opts ListWalletNotesOptions) (WalletNotesPage, error) {
	walletID = strings.TrimSpace(walletID)
	if walletID == "" {
		return WalletNotesPage{}, errors.New("junoscan: wallet_id required")
	}

	spentParam := "false"
	if !opts.OnlyUnspent {
		spentParam = "true"
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 1000 {
		limit = 1000
	}

	params := url.Values{}
	params.Set("spent", spentParam)
	params.Set("limit", fmt.Sprintf("%d", limit))
	if direction := strings.TrimSpace(opts.Direction); direction != "" {
		params.Set("direction", direction)
	}
	if opts.MinValueZat > 0 {
		params.Set("min_value_zat", fmt.Sprintf("%d", opts.MinValueZat))
	}
	if recipientAddress := strings.TrimSpace(opts.RecipientAddress); recipientAddress != "" {
		params.Set("recipient_address", recipientAddress)
	}
	cursor := strings.TrimSpace(opts.Cursor)
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	path := fmt.Sprintf("/v1/wallets/%s/notes?%s", url.PathEscape(walletID), params.Encode())
	var resp WalletNotesPage
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return WalletNotesPage{}, err
	}
	return resp, nil
}

// AddressBalance returns the scanner's on-chain balance buckets for one
// recipient address owned by a registered wallet.
func (c *Client) AddressBalance(ctx context.Context, walletID, address string, minConfirmations int64) (AddressBalanceResponse, error) {
	return c.GetAddressBalance(ctx, AddressBalanceRequest{
		WalletID:         walletID,
		RecipientAddress: address,
		MinConfirmations: minConfirmations,
	})
}

// GetAddressBalance is the request-struct form of AddressBalance.
func (c *Client) GetAddressBalance(ctx context.Context, request AddressBalanceRequest) (AddressBalanceResponse, error) {
	walletID := strings.TrimSpace(request.WalletID)
	address := strings.TrimSpace(request.RecipientAddress)
	if walletID == "" {
		return AddressBalanceResponse{}, errors.New("junoscan: wallet_id required")
	}
	if address == "" {
		return AddressBalanceResponse{}, errors.New("junoscan: address required")
	}
	if request.MinConfirmations < 0 {
		return AddressBalanceResponse{}, errors.New("junoscan: min_confirmations must be >= 0")
	}

	path := fmt.Sprintf(
		"/v1/wallets/%s/addresses/%s/balance?min_confirmations=%d",
		url.PathEscape(walletID),
		url.PathEscape(address),
		request.MinConfirmations,
	)
	var resp AddressBalanceResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return AddressBalanceResponse{}, err
	}
	return resp, nil
}

func (c *Client) OrchardWitness(ctx context.Context, anchorHeight *int64, positions []uint32) (OrchardWitnessResponse, error) {
	if len(positions) == 0 {
		return OrchardWitnessResponse{}, errors.New("junoscan: positions required")
	}
	req := WitnessRequest{
		AnchorHeight: anchorHeight,
		Positions:    positions,
	}
	var resp OrchardWitnessResponse
	if err := c.doJSONWithClient(ctx, c.orchardWitnessHTTPClient, http.MethodPost, "/v1/orchard/witness", req, &resp); err != nil {
		return OrchardWitnessResponse{}, err
	}
	return resp, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	return c.doJSONWithClient(ctx, c.httpClient, method, path, in, out)
}

func (c *Client) doJSONWithClient(ctx context.Context, hc *http.Client, method, path string, in any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if hc == nil {
		hc = http.DefaultClient
	}

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return errors.New("junoscan: marshal request")
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return errors.New("junoscan: build request")
	}
	req.Header.Set("Accept", "application/json")
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	const maxBody = 1 << 20
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if readErr != nil {
		return fmt.Errorf("junoscan: read response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newHTTPError(resp, raw)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("junoscan: invalid json response")
	}
	return nil
}

func newHTTPError(resp *http.Response, raw []byte) *HTTPError {
	err := &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       string(raw),
	}
	var envelope struct {
		Error struct {
			Code      string          `json:"code"`
			Message   string          `json:"message"`
			Retryable bool            `json:"retryable"`
			Details   json.RawMessage `json:"details"`
		} `json:"error"`
		Code      string          `json:"code"`
		Message   string          `json:"message"`
		Retryable bool            `json:"retryable"`
		Details   json.RawMessage `json:"details"`
	}
	if json.Unmarshal(raw, &envelope) == nil {
		if strings.TrimSpace(envelope.Error.Code) != "" || strings.TrimSpace(envelope.Error.Message) != "" {
			err.Code = strings.TrimSpace(envelope.Error.Code)
			err.Message = strings.TrimSpace(envelope.Error.Message)
			err.Retryable = envelope.Error.Retryable
			err.Details = envelope.Error.Details
		} else {
			err.Code = strings.TrimSpace(envelope.Code)
			err.Message = strings.TrimSpace(envelope.Message)
			err.Retryable = envelope.Retryable
			err.Details = envelope.Details
		}
	}
	return err
}

func withMinimumTimeout(hc *http.Client, min time.Duration) *http.Client {
	if hc == nil {
		return &http.Client{Timeout: min}
	}
	if hc.Timeout == 0 || hc.Timeout >= min {
		return hc
	}
	clone := *hc
	clone.Timeout = min
	return &clone
}
