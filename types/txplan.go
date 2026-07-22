package types

type TxPlanKind string

const (
	TxPlanKindWithdrawal TxPlanKind = "withdrawal"
	TxPlanKindSweep      TxPlanKind = "sweep"
	TxPlanKindRebalance  TxPlanKind = "rebalance"
)

type TxOutput struct {
	ToAddress string `json:"to_address"`
	AmountZat string `json:"amount_zat"`
	MemoHex   string `json:"memo_hex,omitempty"`
}

type OrchardSpendNote struct {
	// NoteID is the stable selected source outpoint, encoded as txid:action_index.
	// Plan validators require it so custody systems have an unambiguous
	// reservation identity before signing.
	NoteID          string   `json:"note_id"`
	ActionNullifier string   `json:"action_nullifier"`
	CMX             string   `json:"cmx"`
	Position        uint32   `json:"position"`
	Path            []string `json:"path"`
	EphemeralKey    string   `json:"ephemeral_key"`
	EncCiphertext   string   `json:"enc_ciphertext"`
}

type TxPlan struct {
	Version       Version            `json:"version"` // pinned TxPlan v0
	Kind          TxPlanKind         `json:"kind"`
	WalletID      string             `json:"wallet_id"`
	CoinType      uint32             `json:"coin_type"`
	Account       uint32             `json:"account"`
	Chain         string             `json:"chain"`
	BranchID      uint32             `json:"branch_id"`
	AnchorHeight  uint32             `json:"anchor_height"`
	Anchor        string             `json:"anchor"`
	ExpiryHeight  uint32             `json:"expiry_height"`
	Outputs       []TxOutput         `json:"outputs"`
	ChangeAddress string             `json:"change_address"`
	FeeZat        string             `json:"fee_zat"`
	Notes         []OrchardSpendNote `json:"notes"`
	Metadata      any                `json:"metadata,omitempty"`
}
