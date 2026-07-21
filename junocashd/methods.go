package junocashd

import (
	"context"
)

func (c *Client) GetBlockchainInfo(ctx context.Context) (*BlockchainInfo, error) {
	var out BlockchainInfo
	if err := c.Call(ctx, "getblockchaininfo", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBlockCount(ctx context.Context) (int64, error) {
	var out int64
	if err := c.Call(ctx, "getblockcount", nil, &out); err != nil {
		return 0, err
	}
	return out, nil
}

func (c *Client) GetBestBlockHash(ctx context.Context) (string, error) {
	var out string
	if err := c.Call(ctx, "getbestblockhash", nil, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (c *Client) GetBlockHash(ctx context.Context, height int64) (string, error) {
	var out string
	if err := c.Call(ctx, "getblockhash", []any{height}, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (c *Client) GetBlockHeader(ctx context.Context, blockHash string) (*BlockHeader, error) {
	var out BlockHeader
	if err := c.Call(ctx, "getblockheader", []any{blockHash, true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBlockVerbose(ctx context.Context, blockHash string) (*BlockVerbose, error) {
	var out BlockVerbose
	if err := c.Call(ctx, "getblock", []any{blockHash, 1}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRawTransactionHex(ctx context.Context, txid string) (string, error) {
	var out string
	if err := c.Call(ctx, "getrawtransaction", []any{txid, 0}, &out); err != nil {
		return "", err
	}
	return out, nil
}

// GetRawTransactionVerbose returns the daemon's decoded transaction response.
// The daemon requires txindex for arbitrary confirmed transaction lookups.
// When includeRaw is false, Hex is cleared before the result is returned.
func (c *Client) GetRawTransactionVerbose(ctx context.Context, txid string, includeRaw bool) (*RawTransactionVerbose, error) {
	var out RawTransactionVerbose
	if err := c.Call(ctx, "getrawtransaction", []any{txid, 1}, &out); err != nil {
		return nil, err
	}
	if !includeRaw {
		out.Hex = ""
	}
	return &out, nil
}

func (c *Client) SendRawTransaction(ctx context.Context, txHex string) (string, error) {
	var out string
	if err := c.Call(ctx, "sendrawtransaction", []any{txHex}, &out); err != nil {
		return "", err
	}
	return out, nil
}
