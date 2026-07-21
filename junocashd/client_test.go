package junocashd_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah1738/juno-sdk-go/junocashd"
)

func TestClient_Call_Success(t *testing.T) {
	t.Parallel()

	type req struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      uint64        `json:"id"`
		Method  string        `json:"method"`
		Params  []interface{} `json:"params"`
	}

	var got req
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type=%q", ct)
		}
		auth := r.Header.Get("Authorization")
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if auth != wantAuth {
			t.Fatalf("auth=%q want %q", auth, wantAuth)
		}

		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "ok",
			"error":  nil,
			"id":     got.ID,
		})
	}))
	t.Cleanup(srv.Close)

	cli := junocashd.New(srv.URL, "user", "pass")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var out string
	if err := cli.Call(ctx, "ping", []any{"a", 1}, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != "ok" {
		t.Fatalf("out=%q", out)
	}
	if got.JSONRPC != "1.0" {
		t.Fatalf("jsonrpc=%q", got.JSONRPC)
	}
	if got.Method != "ping" {
		t.Fatalf("method=%q", got.Method)
	}
	if len(got.Params) != 2 {
		t.Fatalf("params=%v", got.Params)
	}
}

func TestClient_Call_RPCErrorWinsOverHTTPStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"result":null,"error":{"code":-1,"message":"boom","data":{"hint":"retry later"}},"id":1}`))
	}))
	t.Cleanup(srv.Close)

	cli := junocashd.New(srv.URL, "", "")
	err := cli.Call(context.Background(), "ping", nil, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%q", err.Error())
	}
	if !junocashd.IsRPCErrorCode(err, -1) {
		t.Fatalf("expected RPC error code -1: %v", err)
	}
	var rpcErr *junocashd.RPCError
	if !errors.As(err, &rpcErr) || !strings.Contains(string(rpcErr.Data), "retry later") {
		t.Fatalf("expected typed RPC data: %#v", rpcErr)
	}
}

func TestClient_Call_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	cli := junocashd.New(srv.URL, "", "")
	err := cli.Call(context.Background(), "ping", nil, nil)
	var httpErr *junocashd.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusServiceUnavailable || !httpErr.Temporary() {
		t.Fatalf("unexpected HTTPError: %#v", httpErr)
	}
}

func TestClient_GetBlockVerbose_Params(t *testing.T) {
	t.Parallel()

	type req struct {
		Method string `json:"method"`
		Params []any  `json:"params"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got req
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got.Method != "getblock" {
			t.Fatalf("method=%q", got.Method)
		}
		if len(got.Params) != 2 {
			t.Fatalf("params=%v", got.Params)
		}
		if got.Params[0] != "hash" {
			t.Fatalf("param0=%v", got.Params[0])
		}
		if got.Params[1] != float64(1) { // JSON numbers decode to float64
			t.Fatalf("param1=%v", got.Params[1])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"hash":   "hash",
				"height": 1,
				"time":   0,
				"tx":     []string{},
			},
			"error": nil,
			"id":    1,
		})
	}))
	t.Cleanup(srv.Close)

	cli := junocashd.New(srv.URL, "", "")
	_, err := cli.GetBlockVerbose(context.Background(), "hash")
	if err != nil {
		t.Fatalf("GetBlockVerbose: %v", err)
	}
}

func TestClient_GetBlockHeader_Params(t *testing.T) {
	t.Parallel()

	type req struct {
		Method string `json:"method"`
		Params []any  `json:"params"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got req
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got.Method != "getblockheader" {
			t.Fatalf("method=%q", got.Method)
		}
		if len(got.Params) != 2 {
			t.Fatalf("params=%v", got.Params)
		}
		if got.Params[0] != "hash" {
			t.Fatalf("param0=%v", got.Params[0])
		}
		if got.Params[1] != true {
			t.Fatalf("param1=%v", got.Params[1])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"hash":   "hash",
				"height": 1,
				"time":   0,
			},
			"error": nil,
			"id":    1,
		})
	}))
	t.Cleanup(srv.Close)

	cli := junocashd.New(srv.URL, "", "")
	_, err := cli.GetBlockHeader(context.Background(), "hash")
	if err != nil {
		t.Fatalf("GetBlockHeader: %v", err)
	}
}

func TestClient_GetRawTransactionVerbose(t *testing.T) {
	t.Parallel()

	type req struct {
		Method string `json:"method"`
		Params []any  `json:"params"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got req
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got.Method != "getrawtransaction" {
			t.Fatalf("method=%q", got.Method)
		}
		if len(got.Params) != 2 || got.Params[0] != "txid" || got.Params[1] != float64(1) {
			t.Fatalf("params=%v", got.Params)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"hex":           "deadbeef",
				"txid":          "txid",
				"size":          123,
				"expiryheight":  456,
				"blockhash":     "block",
				"height":        10,
				"confirmations": 3,
				"blocktime":     999,
				"orchard": map[string]any{
					"actions": []map[string]any{
						{"nullifier": "nf1", "cmx": "cmx1"},
						{"nullifier": "nf2", "cmx": "cmx2"},
					},
				},
			},
			"error": nil,
			"id":    1,
		})
	}))
	t.Cleanup(srv.Close)

	cli := junocashd.New(srv.URL, "", "")
	tx, err := cli.GetRawTransactionVerbose(context.Background(), "txid", false)
	if err != nil {
		t.Fatalf("GetRawTransactionVerbose: %v", err)
	}
	if tx.Hex != "" {
		t.Fatalf("hex should be omitted: %q", tx.Hex)
	}
	if !tx.Confirmed() || tx.InMempool() || tx.OrchardActionCount() != 2 {
		t.Fatalf("unexpected transaction: %#v", tx)
	}
	if tx.Height == nil || *tx.Height != 10 || tx.BlockTime == nil || *tx.BlockTime != 999 {
		t.Fatalf("unexpected block metadata: %#v", tx)
	}

	tx, err = cli.GetRawTransactionVerbose(context.Background(), "txid", true)
	if err != nil {
		t.Fatalf("GetRawTransactionVerbose include raw: %v", err)
	}
	if tx.Hex != "deadbeef" {
		t.Fatalf("hex=%q", tx.Hex)
	}
}

func TestRawTransactionVerbose_Mempool(t *testing.T) {
	tx := junocashd.RawTransactionVerbose{TxID: "txid"}
	if !tx.InMempool() || tx.Confirmed() {
		t.Fatalf("unexpected mempool state")
	}
}
