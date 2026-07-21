package junoscan_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah1738/juno-sdk-go/junoscan"
	"github.com/Abdullah1738/juno-sdk-go/types"
)

func TestClient_UpsertWalletAndList(t *testing.T) {
	var (
		gotWalletID string
		gotUFVK     string
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/wallets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			gotWalletID = strings.TrimSpace(req["wallet_id"])
			gotUFVK = strings.TrimSpace(req["ufvk"])
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"wallets": []map[string]any{
					{"wallet_id": "hot", "created_at": time.Unix(1, 0).UTC()},
				},
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junoscan.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.UpsertWallet(ctx, "hot", "ufvk123"); err != nil {
		t.Fatalf("UpsertWallet: %v", err)
	}
	if gotWalletID != "hot" {
		t.Fatalf("wallet_id=%q", gotWalletID)
	}
	if gotUFVK != "ufvk123" {
		t.Fatalf("ufvk=%q", gotUFVK)
	}

	wallets, err := c.ListWallets(ctx)
	if err != nil {
		t.Fatalf("ListWallets: %v", err)
	}
	if len(wallets) != 1 || wallets[0].WalletID != "hot" {
		t.Fatalf("unexpected wallets")
	}
}

func TestClient_ListWalletEvents(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/wallets/hot/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("cursor") != "7" {
			http.Error(w, "bad cursor", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("limit") != "123" {
			http.Error(w, "bad limit", http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"events": []map[string]any{
				{
					"id":         8,
					"kind":       string(types.WalletEventKindDepositEvent),
					"height":     100,
					"payload":    json.RawMessage(`{"txid":"deadbeef"}`),
					"created_at": time.Unix(2, 0).UTC(),
				},
			},
			"next_cursor": 9,
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junoscan.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	page, err := c.ListWalletEvents(ctx, "hot", 7, 123)
	if err != nil {
		t.Fatalf("ListWalletEvents: %v", err)
	}
	if page.NextCursor != 9 || len(page.Events) != 1 {
		t.Fatalf("unexpected page")
	}
	if page.Events[0].Kind != types.WalletEventKindDepositEvent {
		t.Fatalf("kind=%q", page.Events[0].Kind)
	}
}

func TestClient_HTTPErrorIncludesStatusCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junoscan.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Health(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	var he *junoscan.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if he.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", he.StatusCode)
	}
}

func TestClient_BearerTokenAndStructuredHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer scanner-secret" {
			t.Fatalf("authorization=%q", got)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":      "scanner_not_ready",
				"message":   "scanner is behind node tip",
				"retryable": true,
				"details":   map[string]any{"scanner_lag": 9},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junoscan.New(srv.URL, junoscan.WithBearerToken(" scanner-secret "))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Health(context.Background())
	var httpErr *junoscan.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusServiceUnavailable || httpErr.Code != "scanner_not_ready" {
		t.Fatalf("unexpected error: %#v", httpErr)
	}
	if !httpErr.Temporary() || !httpErr.Retryable {
		t.Fatalf("expected retryable error: %#v", httpErr)
	}
	if !strings.Contains(string(httpErr.Details), `"scanner_lag":9`) {
		t.Fatalf("details=%s", httpErr.Details)
	}
}

func TestClient_HealthEnhancedFields(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":         "degraded",
			"ready":          false,
			"scanned_height": 120,
			"scanned_hash":   "block-120",
			"node_height":    123,
			"scanner_lag":    3,
			"max_ready_lag":  2,
			"action_index": map[string]any{
				"indexed_through": 123,
				"action_heights":  10,
			},
			"shard_cache": map[string]any{
				"enabled":         true,
				"version":         1,
				"leaf_count":      4096,
				"next_index":      4,
				"complete_roots":  4,
				"remaining_roots": 1,
				"last_error":      "",
			},
			"backfills": map[string]any{"active": 1, "queue_depth": 2},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := junoscan.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	health, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.Ready || health.NodeHeight == nil || *health.NodeHeight != 123 || health.ScannerLag == nil || *health.ScannerLag != 3 {
		t.Fatalf("unexpected health: %#v", health)
	}
	if health.ActionIndex == nil || health.ActionIndex.IndexedThrough != 123 {
		t.Fatalf("unexpected action index: %#v", health.ActionIndex)
	}
	if health.ShardCache == nil || health.ShardCache.LeafCount != 4096 || health.ShardCache.RemainingRoots != 1 {
		t.Fatalf("unexpected shard cache: %#v", health.ShardCache)
	}
	if health.Backfills == nil || health.Backfills.Active != 1 || health.Backfills.QueueDepth != 2 {
		t.Fatalf("unexpected backfills: %#v", health.Backfills)
	}
}

func TestClient_ListWalletNotesPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/wallets/hot/notes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		if q.Get("spent") != "false" {
			http.Error(w, "bad spent", http.StatusBadRequest)
			return
		}
		if q.Get("limit") != "200" {
			http.Error(w, "bad limit", http.StatusBadRequest)
			return
		}
		if q.Get("direction") != "incoming" {
			http.Error(w, "bad direction", http.StatusBadRequest)
			return
		}
		if q.Get("min_value_zat") != "1234" {
			http.Error(w, "bad min_value_zat", http.StatusBadRequest)
			return
		}
		if q.Get("recipient_address") != "jregtest1recipient" {
			http.Error(w, "bad recipient_address", http.StatusBadRequest)
			return
		}
		if q.Get("cursor") != "11:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:0" {
			http.Error(w, "bad cursor", http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"notes": []map[string]any{
				{
					"txid":              "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					"action_index":      0,
					"height":            12,
					"value_zat":         5000,
					"note_nullifier":    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
					"recipient_address": "jtest1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqp4f3t7",
					"created_at":        time.Unix(3, 0).UTC(),
				},
			},
			"next_cursor": "12:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb:0",
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junoscan.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	page, err := c.ListWalletNotesPage(ctx, "hot", junoscan.ListWalletNotesOptions{
		OnlyUnspent:      true,
		Direction:        "incoming",
		Limit:            200,
		MinValueZat:      1234,
		RecipientAddress: " jregtest1recipient ",
		Cursor:           "11:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:0",
	})
	if err != nil {
		t.Fatalf("ListWalletNotesPage: %v", err)
	}
	if len(page.Notes) != 1 {
		t.Fatalf("notes=%d", len(page.Notes))
	}
	if page.NextCursor == "" {
		t.Fatalf("expected next_cursor")
	}
}

func TestClient_AddressBalance(t *testing.T) {
	const address = "jregtest1recipient"
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/wallets/hot/addresses/"+address+"/balance", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("min_confirmations") != "100" {
			http.Error(w, "bad min_confirmations", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(junoscan.AddressBalanceResponse{
			WalletID:           "hot",
			RecipientAddress:   address,
			AvailableZat:       1000,
			PendingIncomingZat: 200,
			PendingOutgoingZat: 300,
			TotalUnspentZat:    1500,
			MinConfirmations:   100,
			AsOfNodeHeight:     200,
			AsOfScannerHeight:  198,
			ScannerLag:         2,
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := junoscan.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	balance, err := c.AddressBalance(context.Background(), "hot", address, 100)
	if err != nil {
		t.Fatalf("AddressBalance: %v", err)
	}
	if balance.AvailableZat != 1000 || balance.PendingIncomingZat != 200 || balance.ScannerLag != 2 {
		t.Fatalf("unexpected balance: %#v", balance)
	}
}

func TestClient_AddressBalanceValidatesInput(t *testing.T) {
	c, err := junoscan.New("http://example.invalid")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		name     string
		walletID string
		address  string
		minConfs int64
	}{
		{name: "wallet", address: "addr"},
		{name: "address", walletID: "hot"},
		{name: "confirmations", walletID: "hot", address: "addr", minConfs: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := c.AddressBalance(context.Background(), tt.walletID, tt.address, tt.minConfs); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestClient_OrchardWitnessUsesExtendedTimeoutBudget(t *testing.T) {
	const (
		shortClientTimeout = 40 * time.Millisecond
		serverDelay        = 120 * time.Millisecond
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serverDelay)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})
	mux.HandleFunc("/v1/orchard/witness", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serverDelay)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":        "ok",
			"anchor_height": 123,
			"root":          strings.Repeat("a", 64),
			"paths": []map[string]any{
				{
					"position":  0,
					"auth_path": []string{strings.Repeat("b", 64)},
				},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junoscan.New(srv.URL, junoscan.WithHTTPClient(&http.Client{Timeout: shortClientTimeout}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := c.Health(ctx); err == nil {
		t.Fatalf("Health: expected timeout")
	} else {
		var ne net.Error
		if !errors.As(err, &ne) || !ne.Timeout() {
			t.Fatalf("Health: expected timeout error, got %v", err)
		}
	}

	got, err := c.OrchardWitness(ctx, nil, []uint32{0})
	if err != nil {
		t.Fatalf("OrchardWitness: %v", err)
	}
	if got.AnchorHeight != 123 {
		t.Fatalf("anchor_height=%d", got.AnchorHeight)
	}
	if len(got.Paths) != 1 || got.Paths[0].Position != 0 {
		t.Fatalf("unexpected witness paths: %#v", got.Paths)
	}
}
