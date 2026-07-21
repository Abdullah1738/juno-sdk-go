package junobroadcast_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah1738/juno-sdk-go/junobroadcast"
)

func TestClient_Submit_NoWait(t *testing.T) {
	var gotRaw string
	var gotWait *int64

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tx/submit", func(w http.ResponseWriter, r *http.Request) {
		var req junobroadcast.SubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		gotRaw = strings.TrimSpace(req.RawTxHex)
		gotWait = req.WaitConfirmations
		_ = json.NewEncoder(w).Encode(junobroadcast.SubmitResponse{TxID: "deadbeef"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junobroadcast.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.Submit(ctx, "00", nil)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.TxID != "deadbeef" {
		t.Fatalf("txid=%q", resp.TxID)
	}
	if resp.Status != nil {
		t.Fatalf("unexpected status")
	}
	if gotRaw != "00" {
		t.Fatalf("raw=%q", gotRaw)
	}
	if gotWait != nil {
		t.Fatalf("wait_confirmations should be omitted")
	}
}

func TestClient_Submit_WaitConfirmations(t *testing.T) {
	var gotWait int64

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tx/submit", func(w http.ResponseWriter, r *http.Request) {
		var req junobroadcast.SubmitRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.WaitConfirmations == nil {
			http.Error(w, "missing wait", http.StatusBadRequest)
			return
		}
		gotWait = *req.WaitConfirmations
		_ = json.NewEncoder(w).Encode(junobroadcast.SubmitResponse{
			TxID: "deadbeef",
			Status: &junobroadcast.TxStatus{
				TxID:          "deadbeef",
				InMempool:     false,
				Confirmations: 1,
				BlockHash:     "block",
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junobroadcast.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	one := int64(1)
	resp, err := c.Submit(ctx, "00", &one)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotWait != 1 {
		t.Fatalf("wait=%d", gotWait)
	}
	if resp.Status == nil || resp.Status.BlockHash != "block" {
		t.Fatalf("unexpected status")
	}
}

func TestClient_Status_NotFoundReturnsFoundFalse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/tx/deadbeef", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "not_found",
				"message": "unknown txid",
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junobroadcast.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, found, err := c.Status(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if found {
		t.Fatalf("expected found=false")
	}
}

func TestClient_APIErrorContainsCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":      "scanner_not_ready",
				"message":   "nope",
				"retryable": true,
				"details":   map[string]any{"lag": 4},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := junobroadcast.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.Health(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	var ae *junobroadcast.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if ae.Code != "scanner_not_ready" {
		t.Fatalf("code=%q", ae.Code)
	}
	if !ae.Temporary() || !ae.Retryable || !strings.Contains(string(ae.Details), `"lag":4`) {
		t.Fatalf("unexpected APIError: %#v", ae)
	}
}
