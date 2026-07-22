package types_test

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/Abdullah1738/juno-sdk-go/types"
)

func TestBrokerEnvelope_JSONRoundTrip(t *testing.T) {
	in := types.BrokerEnvelope{
		Version:  types.V1,
		Kind:     types.WalletEventKindDepositEvent,
		WalletID: "hot",
		Height:   123,
		Payload:  json.RawMessage(`{"txid":"deadbeef"}`),
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out types.BrokerEnvelope
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n  in=%#v\n out=%#v", in, out)
	}
}

func TestDepositLifecyclePayloadsAlwaysIncludeDiversifierIndex(t *testing.T) {
	for _, index := range []uint32{0, 7} {
		base := types.DepositEventPayload{
			DepositEvent: types.DepositEvent{
				Version:          types.V1,
				WalletID:         "hot",
				DiversifierIndex: index,
				TxID:             "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				Height:           100,
				ActionIndex:      0,
				AmountZatoshis:   1,
				Status:           types.TxStatus{State: types.TxStateConfirmed, Height: 100, Confirmations: 1},
			},
			Origin:           types.DepositOriginExternal,
			RecipientAddress: "j1recipient",
		}
		payloads := []any{
			base,
			types.DepositConfirmedPayload{DepositEventPayload: base, ConfirmedHeight: 199, RequiredConfirmations: 100},
			types.DepositUnconfirmedPayload{DepositEventPayload: base, RollbackHeight: 198, PreviousConfirmedHeight: 199},
			types.DepositOrphanedPayload{DepositEventPayload: base, OrphanedAtHeight: 99},
		}
		for _, payload := range payloads {
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal index %d: %v", index, err)
			}
			var decoded map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("decode index %d: %v", index, err)
			}
			value, ok := decoded["diversifier_index"]
			if !ok || string(value) != strconv.FormatUint(uint64(index), 10) {
				t.Fatalf("diversifier_index=%s present=%v want %d in %s", value, ok, index, encoded)
			}
		}
	}
}

func TestDepositConfirmedPayload_JSONRoundTrip(t *testing.T) {
	in := types.DepositConfirmedPayload{
		DepositEventPayload: types.DepositEventPayload{
			DepositEvent: types.DepositEvent{
				Version:          types.V1,
				WalletID:         "hot",
				DiversifierIndex: 7,
				TxID:             "deadbeef",
				Height:           100,
				ActionIndex:      3,
				AmountZatoshis:   5000,
				MemoHex:          "00",
				Status: types.TxStatus{
					State:         types.TxStateConfirmed,
					Height:        100,
					Confirmations: 10,
				},
			},
			Origin:           types.DepositOriginExternal,
			RecipientAddress: "j1recipient",
			NoteNullifier:    "nullifier",
		},
		ConfirmedHeight:       109,
		RequiredConfirmations: 10,
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out types.DepositConfirmedPayload
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n  in=%#v\n out=%#v", in, out)
	}
	if out.Origin != types.DepositOriginExternal {
		t.Fatalf("origin=%q", out.Origin)
	}
}
