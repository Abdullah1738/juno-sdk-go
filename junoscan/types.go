package junoscan

import (
	"encoding/json"
	"time"

	"github.com/Abdullah1738/juno-sdk-go/types"
)

type HealthResponse struct {
	Status                string             `json:"status"`
	Ready                 bool               `json:"ready"`
	Network               string             `json:"network,omitempty"`
	UAHRP                 string             `json:"ua_hrp,omitempty"`
	ScannedHeight         *int64             `json:"scanned_height,omitempty"`
	ScannedHash           *string            `json:"scanned_hash,omitempty"`
	NodeHeight            *int64             `json:"node_height,omitempty"`
	ScannerLag            *int64             `json:"scanner_lag,omitempty"`
	MaxReadyLag           *int64             `json:"max_ready_lag,omitempty"`
	HistoryComplete       *bool              `json:"history_complete,omitempty"`
	HistoryPendingWallets *int               `json:"history_pending_wallets,omitempty"`
	ActionIndex           *HealthActionIndex `json:"action_index,omitempty"`
	ShardCache            *HealthShardCache  `json:"shard_cache,omitempty"`
	Backfills             *HealthBackfills   `json:"backfills,omitempty"`
}

type HealthActionIndex struct {
	IndexedThrough int64 `json:"indexed_through"`
	ActionHeights  int64 `json:"action_heights"`
}

type HealthShardCache struct {
	Enabled        bool   `json:"enabled"`
	Version        int    `json:"version"`
	LeafCount      int64  `json:"leaf_count"`
	NextIndex      int64  `json:"next_index"`
	CompleteRoots  int64  `json:"complete_roots"`
	RemainingRoots int64  `json:"remaining_roots"`
	LastError      string `json:"last_error"`
}

type HealthBackfills struct {
	Active     int `json:"active"`
	QueueDepth int `json:"queue_depth"`
}

type Wallet struct {
	WalletID       string     `json:"wallet_id"`
	BirthdayHeight int64      `json:"birthday_height"`
	CreatedAt      time.Time  `json:"created_at"`
	DisabledAt     *time.Time `json:"disabled_at,omitempty"`
}

type RegisterWalletRequest struct {
	WalletID       string `json:"wallet_id"`
	UFVK           string `json:"ufvk"`
	BirthdayHeight *int64 `json:"birthday_height,omitempty"`
}

type RegisterWalletResponse struct {
	Status         string `json:"status"`
	BirthdayHeight int64  `json:"birthday_height"`
}

type WalletBackfillState string

const (
	WalletBackfillPending  WalletBackfillState = "pending"
	WalletBackfillRunning  WalletBackfillState = "running"
	WalletBackfillComplete WalletBackfillState = "complete"
	WalletBackfillError    WalletBackfillState = "error"
)

type WalletBackfillStatus struct {
	WalletID       string              `json:"wallet_id"`
	BirthdayHeight int64               `json:"birthday_height"`
	NextHeight     int64               `json:"next_height"`
	TargetHeight   int64               `json:"target_height"`
	State          WalletBackfillState `json:"state"`
	LastError      string              `json:"last_error,omitempty"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

// WalletBackfillRequest controls one bounded scanner pass. Leave FromHeight
// nil to resume from the scanner's persisted NextHeight.
type WalletBackfillRequest struct {
	FromHeight *int64 `json:"from_height,omitempty"`
	ToHeight   *int64 `json:"to_height,omitempty"`
	BatchSize  int64  `json:"batch_size,omitempty"`
}

type WalletBackfillResponse struct {
	Status               string `json:"status"`
	WalletID             string `json:"wallet_id"`
	FromHeight           int64  `json:"from_height"`
	ToHeight             int64  `json:"to_height"`
	ScannedFrom          int64  `json:"scanned_from"`
	ScannedTo            int64  `json:"scanned_to"`
	NextHeight           int64  `json:"next_height"`
	InsertedNotes        int64  `json:"inserted_notes"`
	InsertedEvents       int64  `json:"inserted_events"`
	VisitedActionHeights int64  `json:"visited_action_heights"`
	SkippedHeights       int64  `json:"skipped_heights"`
	RPCCalls             int64  `json:"rpc_calls"`
}

type WalletEvent struct {
	ID        int64                 `json:"id"`
	Kind      types.WalletEventKind `json:"kind"`
	Height    int64                 `json:"height"`
	Payload   json.RawMessage       `json:"payload"`
	CreatedAt time.Time             `json:"created_at"`
}

type WalletEventsPage struct {
	Events     []WalletEvent `json:"events"`
	NextCursor int64         `json:"next_cursor"`
}

type WalletNote struct {
	Direction                string     `json:"direction,omitempty"`
	TxID                     string     `json:"txid"`
	ActionIndex              int32      `json:"action_index"`
	Height                   int64      `json:"height"`
	Position                 *int64     `json:"position,omitempty"`
	RecipientAddress         string     `json:"recipient_address"`
	ValueZat                 int64      `json:"value_zat"`
	NoteNullifier            string     `json:"note_nullifier"`
	PendingSpentTxID         *string    `json:"pending_spent_txid,omitempty"`
	PendingSpentAt           *time.Time `json:"pending_spent_at,omitempty"`
	PendingSpentExpiryHeight *int64     `json:"pending_spent_expiry_height,omitempty"`
	SpentHeight              *int64     `json:"spent_height,omitempty"`
	SpentTxID                *string    `json:"spent_txid,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

type WalletNotesPage struct {
	Notes      []WalletNote `json:"notes"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

type ListWalletNotesOptions struct {
	OnlyUnspent      bool
	Direction        string
	MinValueZat      int64
	RecipientAddress string
	Limit            int
	Cursor           string
}

type AddressBalanceResponse struct {
	WalletID           string `json:"wallet_id"`
	RecipientAddress   string `json:"recipient_address"`
	AvailableZat       int64  `json:"available_zat"`
	PendingIncomingZat int64  `json:"pending_incoming_zat"`
	PendingOutgoingZat int64  `json:"pending_outgoing_zat"`
	TotalUnspentZat    int64  `json:"total_unspent_zat"`
	MinConfirmations   int64  `json:"min_confirmations"`
	AsOfNodeHeight     int64  `json:"as_of_node_height"`
	AsOfScannerHeight  int64  `json:"as_of_scanner_height"`
	ScannerLag         int64  `json:"scanner_lag"`
}

type AddressBalanceRequest struct {
	WalletID         string `json:"wallet_id"`
	RecipientAddress string `json:"recipient_address"`
	MinConfirmations int64  `json:"min_confirmations"`
}

type WitnessRequest struct {
	AnchorHeight *int64   `json:"anchor_height,omitempty"`
	Positions    []uint32 `json:"positions"`
}

type OrchardWitnessPath struct {
	Position uint32   `json:"position"`
	AuthPath []string `json:"auth_path"`
}

type OrchardWitnessResponse struct {
	Status       string               `json:"status"`
	AnchorHeight int64                `json:"anchor_height"`
	Root         string               `json:"root"`
	Paths        []OrchardWitnessPath `json:"paths"`
}
