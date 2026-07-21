# Changelog

## Unreleased

- Add authenticated juno-scan access, event epochs, immutable UFVK fingerprints, external deposit origin, birthday-aware wallet registration, typed historical backfill progress, bounded resume calls, address-filtered note queries, typed address balances, and scanner readiness/cache health fields.
- Add typed verbose `junocashd` transaction lookup with optional raw hex and mined/mempool helpers.
- Preserve structured HTTP, API, and JSON-RPC error metadata for gateway retry and status mapping.

## v1.3 (2026-02-10)

- Add `types.TxStateExpired` for representing deterministically expired transactions.
- Expose `pending_spent_expiry_height` on `junoscan.WalletNote`.
- Add `OutgoingOutput*` event kinds and payload types for juno-scan wallet events.
