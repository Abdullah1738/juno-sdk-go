package types

type TxState string

const (
	TxStateMempool   TxState = "mempool"
	TxStateConfirmed TxState = "confirmed"
	TxStateOrphaned  TxState = "orphaned"
	TxStateExpired   TxState = "expired"
)

type TxStatus struct {
	State         TxState `json:"state"`
	Height        int64   `json:"height,omitempty"`
	Confirmations int64   `json:"confirmations,omitempty"`
}

type DepositEvent struct {
	Version          Version  `json:"version"`
	WalletID         string   `json:"wallet_id"`
	AccountID        string   `json:"account_id,omitempty"`
	DiversifierIndex uint32   `json:"diversifier_index"`
	TxID             string   `json:"txid"`
	Height           int64    `json:"height"`
	ActionIndex      uint32   `json:"action_index"`
	AmountZatoshis   uint64   `json:"amount_zatoshis"`
	MemoHex          string   `json:"memo_hex,omitempty"`
	Status           TxStatus `json:"status"`
}
