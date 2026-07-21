//go:build integration

package junocashd_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Abdullah1738/juno-sdk-go/internal/testutil"
	"github.com/Abdullah1738/juno-sdk-go/junocashd"
)

func TestClient_GetBlockchainInfo_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	r, err := testutil.StartJunocashd(ctx, testutil.JunocashdConfig{})
	if err != nil {
		if errors.Is(err, testutil.ErrJunocashdNotFound) {
			t.Skip("junocashd not found in PATH")
		}
		t.Fatalf("StartJunocashd: %v", err)
	}
	defer func() { _ = r.Stop(context.Background()) }()

	cli := junocashd.New(r.RPCURL, r.RPCUser, r.RPCPassword)

	info, err := cli.GetBlockchainInfo(ctx)
	if err != nil {
		t.Fatalf("GetBlockchainInfo: %v", err)
	}
	if info.Chain != "regtest" {
		t.Fatalf("chain=%q want regtest", info.Chain)
	}
}

func TestClient_GetBlockVerbose_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	r, err := testutil.StartJunocashd(ctx, testutil.JunocashdConfig{})
	if err != nil {
		if errors.Is(err, testutil.ErrJunocashdNotFound) {
			t.Skip("junocashd not found in PATH")
		}
		t.Fatalf("StartJunocashd: %v", err)
	}
	defer func() { _ = r.Stop(context.Background()) }()

	cli := junocashd.New(r.RPCURL, r.RPCUser, r.RPCPassword)
	best, err := cli.GetBestBlockHash(ctx)
	if err != nil {
		t.Fatalf("GetBestBlockHash: %v", err)
	}

	block, err := cli.GetBlockVerbose(ctx, best)
	if err != nil {
		t.Fatalf("GetBlockVerbose: %v", err)
	}
	if block.Hash != best {
		t.Fatalf("hash=%q want %q", block.Hash, best)
	}
	if block.Height != 0 {
		t.Fatalf("height=%d want 0", block.Height)
	}
	if len(block.Tx) == 0 {
		t.Fatalf("expected at least one tx in genesis block")
	}
}

func TestClient_GetRawTransactionVerbose_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	r, err := testutil.StartJunocashd(ctx, testutil.JunocashdConfig{TxIndex: true})
	if err != nil {
		if errors.Is(err, testutil.ErrJunocashdNotFound) {
			t.Skip("junocashd not found in PATH")
		}
		t.Fatalf("StartJunocashd: %v", err)
	}
	defer func() { _ = r.Stop(context.Background()) }()

	cli := junocashd.New(r.RPCURL, r.RPCUser, r.RPCPassword)
	if out, err := r.CLICommand(ctx, "generate", "1").CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, string(out))
	}
	best, err := cli.GetBlockVerbose(ctx, mustBestBlockHash(t, ctx, cli))
	if err != nil {
		t.Fatalf("GetBlockVerbose: %v", err)
	}
	if len(best.Tx) == 0 {
		t.Fatalf("expected genesis transaction")
	}

	tx, err := cli.GetRawTransactionVerbose(ctx, best.Tx[0], true)
	if err != nil {
		t.Fatalf("GetRawTransactionVerbose: %v", err)
	}
	if tx.TxID != best.Tx[0] || tx.Hex == "" {
		t.Fatalf("unexpected transaction: %#v", tx)
	}
	if !tx.Confirmed() || tx.InMempool() {
		t.Fatalf("expected confirmed transaction: %#v", tx)
	}
	if tx.Height == nil || *tx.Height != best.Height {
		t.Fatalf("transaction height=%v want %d", tx.Height, best.Height)
	}
	if tx.BlockTime == nil || *tx.BlockTime != best.Time {
		t.Fatalf("transaction blocktime=%v want %d", tx.BlockTime, best.Time)
	}
}

func mustBestBlockHash(t *testing.T, ctx context.Context, cli *junocashd.Client) string {
	t.Helper()
	hash, err := cli.GetBestBlockHash(ctx)
	if err != nil {
		t.Fatalf("GetBestBlockHash: %v", err)
	}
	return hash
}
