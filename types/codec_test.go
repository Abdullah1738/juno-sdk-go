package types_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Abdullah1738/juno-sdk-go/types"
)

func TestTxPlan_JSONRoundTrip(t *testing.T) {
	in := types.TxPlan{
		Version:      types.V0,
		Kind:         types.TxPlanKindSweep,
		WalletID:     "hot",
		CoinType:     1337,
		Account:      0,
		Chain:        "regtest",
		BranchID:     0x4dec4df0,
		AnchorHeight: 123,
		Anchor:       "deadbeef",
		ExpiryHeight: 456,
		Outputs: []types.TxOutput{
			{ToAddress: "j1test", AmountZat: "12345", MemoHex: "00"},
		},
		ChangeAddress: "j1change",
		FeeZat:        "1000",
		Notes: []types.OrchardSpendNote{
			{
				NoteID:          "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef:7",
				ActionNullifier: "00",
				CMX:             "00",
				Position:        7,
				Path:            make([]string, 32),
				EphemeralKey:    "00",
				EncCiphertext:   "00",
			},
		},
		Metadata: map[string]any{
			"reason": "test",
		},
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out types.TxPlan
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n  in=%#v\n out=%#v", in, out)
	}
}

func TestOrchardSpendNote_JSONAlwaysIncludesNoteID(t *testing.T) {
	b, err := json.Marshal(types.OrchardSpendNote{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["note_id"]; !ok {
		t.Fatal("note_id was omitted")
	}
}

func TestDepositEvent_JSONRoundTrip(t *testing.T) {
	in := types.DepositEvent{
		Version:          types.V1,
		WalletID:         "hot",
		AccountID:        "acct_123",
		DiversifierIndex: 1,
		TxID:             "deadbeef",
		Height:           100,
		ActionIndex:      0,
		AmountZatoshis:   5000,
		MemoHex:          "",
		Status: types.TxStatus{
			State:         types.TxStateConfirmed,
			Height:        100,
			Confirmations: 3,
		},
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out types.DepositEvent
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n  in=%#v\n out=%#v", in, out)
	}
}

func TestChainCursor_JSONRoundTrip(t *testing.T) {
	in := types.ChainCursor{
		Height: 123,
		Hash:   "deadbeef",
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out types.ChainCursor
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n  in=%#v\n out=%#v", in, out)
	}
}
