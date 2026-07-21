package junocashd

type BlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers,omitempty"`
	BestBlockHash        string  `json:"bestblockhash,omitempty"`
	InitialBlockDownload bool    `json:"initialblockdownload,omitempty"`
	VerificationProgress float64 `json:"verificationprogress,omitempty"`
	Difficulty           float64 `json:"difficulty,omitempty"`
	ChainWork            string  `json:"chainwork,omitempty"`
	Pruned               bool    `json:"pruned,omitempty"`
	PruneHeight          int64   `json:"pruneheight,omitempty"`
	SizeOnDisk           int64   `json:"size_on_disk,omitempty"`
}

type BlockHeader struct {
	Hash              string `json:"hash"`
	Confirmations     int64  `json:"confirmations,omitempty"`
	Height            int64  `json:"height"`
	Time              int64  `json:"time"`
	PreviousBlockHash string `json:"previousblockhash,omitempty"`
	NextBlockHash     string `json:"nextblockhash,omitempty"`
}

type BlockVerbose struct {
	Hash              string   `json:"hash"`
	Confirmations     int64    `json:"confirmations,omitempty"`
	Height            int64    `json:"height"`
	Time              int64    `json:"time"`
	PreviousBlockHash string   `json:"previousblockhash,omitempty"`
	NextBlockHash     string   `json:"nextblockhash,omitempty"`
	Tx                []string `json:"tx"`
}

// RawTransactionVerbose models the stable fields returned by
// getrawtransaction with verbose=1. Shielded recipients and values are not
// revealed by this generic node response.
type RawTransactionVerbose struct {
	Hex            string        `json:"hex,omitempty"`
	TxID           string        `json:"txid"`
	Hash           string        `json:"hash,omitempty"`
	Size           int64         `json:"size,omitempty"`
	Overwintered   bool          `json:"overwintered,omitempty"`
	Version        int64         `json:"version,omitempty"`
	VersionGroupID string        `json:"versiongroupid,omitempty"`
	LockTime       int64         `json:"locktime,omitempty"`
	ExpiryHeight   int64         `json:"expiryheight,omitempty"`
	Orchard        OrchardBundle `json:"orchard,omitempty"`
	BlockHash      string        `json:"blockhash,omitempty"`
	Height         *int64        `json:"height,omitempty"`
	Confirmations  int64         `json:"confirmations,omitempty"`
	Time           *int64        `json:"time,omitempty"`
	BlockTime      *int64        `json:"blocktime,omitempty"`
}

type OrchardBundle struct {
	Actions []OrchardAction `json:"actions,omitempty"`
}

type OrchardAction struct {
	Nullifier string `json:"nullifier,omitempty"`
	CMX       string `json:"cmx,omitempty"`
}

// InMempool reports whether the verbose response describes an unmined
// transaction. A successful verbose lookup has either mempool or block data.
func (t RawTransactionVerbose) InMempool() bool {
	return t.BlockHash == "" && t.Confirmations == 0
}

func (t RawTransactionVerbose) Confirmed() bool {
	return t.BlockHash != "" && t.Confirmations > 0
}

func (t RawTransactionVerbose) OrchardActionCount() int {
	return len(t.Orchard.Actions)
}
