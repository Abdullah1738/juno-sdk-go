package junoscan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	backfillHTTPTimeoutBudget       = 10*time.Minute + 5*time.Second
)

type Client struct {
	baseURL                  string
	bearerToken              string
	httpClient               *http.Client
	orchardWitnessHTTPClient *http.Client
	backfillHTTPClient       *http.Client
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
	c.backfillHTTPClient = withMinimumTimeout(c.httpClient, backfillHTTPTimeoutBudget)
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
	if !isLowerHex64(resp.EventEpoch) {
		return HealthResponse{}, errors.New("junoscan: invalid event_epoch")
	}
	return resp, nil
}

func (c *Client) UpsertWallet(ctx context.Context, walletID, ufvk string) error {
	_, err := c.RegisterWallet(ctx, RegisterWalletRequest{WalletID: walletID, UFVK: ufvk})
	return err
}

// UpsertWalletWithBirthday registers or updates a wallet and its earliest
// scan height. The legacy UpsertWallet method remains available and omits the
// birthday, preserving the scanner's default of height zero.
func (c *Client) UpsertWalletWithBirthday(ctx context.Context, walletID, ufvk string, birthdayHeight int64) error {
	_, err := c.RegisterWallet(ctx, RegisterWalletRequest{
		WalletID:       walletID,
		UFVK:           ufvk,
		BirthdayHeight: &birthdayHeight,
	})
	return err
}

// RegisterWallet performs the typed form of the scanner wallet upsert.
func (c *Client) RegisterWallet(ctx context.Context, request RegisterWalletRequest) (RegisterWalletResponse, error) {
	walletID := strings.TrimSpace(request.WalletID)
	ufvk := strings.TrimSpace(request.UFVK)
	if walletID == "" || ufvk == "" {
		return RegisterWalletResponse{}, errors.New("junoscan: wallet_id and ufvk required")
	}
	if request.BirthdayHeight != nil && *request.BirthdayHeight < 0 {
		return RegisterWalletResponse{}, errors.New("junoscan: birthday_height must be >= 0")
	}
	request.WalletID = walletID
	request.UFVK = ufvk

	var resp RegisterWalletResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/wallets", request, &resp); err != nil {
		return RegisterWalletResponse{}, err
	}
	if strings.ToLower(strings.TrimSpace(resp.Status)) != "ok" || resp.BirthdayHeight < 0 || !isLowerHex64(resp.UFVKFingerprint) || resp.UFVKFingerprint != ufvkFingerprint(ufvk) {
		return RegisterWalletResponse{}, errors.New("junoscan: unexpected response")
	}
	return resp, nil
}

// GetWalletBackfillStatus returns persisted scanner progress. A missing wallet
// is reported as found=false rather than as an error.
func (c *Client) GetWalletBackfillStatus(ctx context.Context, walletID string) (status WalletBackfillStatus, found bool, err error) {
	walletID = strings.TrimSpace(walletID)
	if walletID == "" {
		return WalletBackfillStatus{}, false, errors.New("junoscan: wallet_id required")
	}

	path := fmt.Sprintf("/v1/wallets/%s/backfill", url.PathEscape(walletID))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &status); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return WalletBackfillStatus{}, false, nil
		}
		return WalletBackfillStatus{}, false, err
	}
	if err := validateWalletBackfillStatus(walletID, status); err != nil {
		return WalletBackfillStatus{}, false, err
	}
	return status, true, nil
}

// BackfillWallet runs one bounded historical scan pass. Omitting FromHeight
// resumes from the scanner's persisted progress.
func (c *Client) BackfillWallet(ctx context.Context, walletID string, request WalletBackfillRequest) (WalletBackfillResponse, error) {
	walletID = strings.TrimSpace(walletID)
	if walletID == "" {
		return WalletBackfillResponse{}, errors.New("junoscan: wallet_id required")
	}
	if request.FromHeight != nil && *request.FromHeight < 0 {
		return WalletBackfillResponse{}, errors.New("junoscan: from_height must be >= 0")
	}
	if request.ToHeight != nil && *request.ToHeight < 0 {
		return WalletBackfillResponse{}, errors.New("junoscan: to_height must be >= 0")
	}
	if request.FromHeight != nil && request.ToHeight != nil && *request.ToHeight < *request.FromHeight {
		return WalletBackfillResponse{}, errors.New("junoscan: to_height must be >= from_height")
	}
	if request.BatchSize < 0 || request.BatchSize > 10_000 {
		return WalletBackfillResponse{}, errors.New("junoscan: batch_size must be between 1 and 10000 when set")
	}

	path := fmt.Sprintf("/v1/wallets/%s/backfill", url.PathEscape(walletID))
	var resp WalletBackfillResponse
	if err := c.doJSONWithClient(ctx, c.backfillHTTPClient, http.MethodPost, path, request, &resp); err != nil {
		return WalletBackfillResponse{}, err
	}
	if strings.ToLower(strings.TrimSpace(resp.Status)) != "ok" || resp.WalletID != walletID || resp.NextHeight < 0 {
		return WalletBackfillResponse{}, errors.New("junoscan: invalid backfill response")
	}
	if request.ToHeight != nil && resp.NextHeight > *request.ToHeight+1 {
		return WalletBackfillResponse{}, errors.New("junoscan: invalid backfill progress")
	}
	return resp, nil
}

// ResumeWalletBackfill runs one pass from persisted progress through toHeight.
func (c *Client) ResumeWalletBackfill(ctx context.Context, walletID string, toHeight, batchSize int64) (WalletBackfillResponse, error) {
	if toHeight < 0 {
		return WalletBackfillResponse{}, errors.New("junoscan: to_height must be >= 0")
	}
	if batchSize < 1 || batchSize > 10_000 {
		return WalletBackfillResponse{}, errors.New("junoscan: batch_size must be between 1 and 10000")
	}
	return c.BackfillWallet(ctx, walletID, WalletBackfillRequest{ToHeight: &toHeight, BatchSize: batchSize})
}

func (c *Client) ListWallets(ctx context.Context) ([]Wallet, error) {
	var resp struct {
		Wallets []Wallet `json:"wallets"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v1/wallets", nil, &resp); err != nil {
		return nil, err
	}
	for _, wallet := range resp.Wallets {
		if strings.TrimSpace(wallet.WalletID) == "" || !isLowerHex64(wallet.UFVKFingerprint) || wallet.BirthdayHeight < 0 {
			return nil, errors.New("junoscan: invalid wallet identity")
		}
	}
	return resp.Wallets, nil
}

func (c *Client) ListWalletEvents(ctx context.Context, walletID string, cursor int64, limit int) (WalletEventsPage, error) {
	walletID = strings.TrimSpace(walletID)
	if walletID == "" {
		return WalletEventsPage{}, errors.New("junoscan: wallet_id required")
	}
	if cursor < 0 {
		return WalletEventsPage{}, errors.New("junoscan: cursor must be >= 0")
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
	if !isLowerHex64(resp.EventEpoch) || resp.NextCursor < cursor {
		return WalletEventsPage{}, errors.New("junoscan: invalid wallet events page")
	}
	position := cursor
	for _, event := range resp.Events {
		if event.ID <= position {
			return WalletEventsPage{}, errors.New("junoscan: invalid wallet events page")
		}
		position = event.ID
	}
	if resp.NextCursor != position {
		return WalletEventsPage{}, errors.New("junoscan: invalid wallet events cursor")
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

func validateWalletBackfillStatus(walletID string, status WalletBackfillStatus) error {
	if status.WalletID != walletID || !isLowerHex64(status.UFVKFingerprint) || status.BirthdayHeight < 0 || status.NextHeight < status.BirthdayHeight || status.TargetHeight < 0 {
		return errors.New("junoscan: invalid backfill status")
	}
	switch status.State {
	case WalletBackfillPending, WalletBackfillRunning, WalletBackfillComplete, WalletBackfillError:
		return nil
	default:
		return errors.New("junoscan: invalid backfill state")
	}
}

func ufvkFingerprint(ufvk string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ufvk)))
	return hex.EncodeToString(sum[:])
}

func isLowerHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
