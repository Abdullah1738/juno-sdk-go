# juno-sdk-go

Go SDK and shared types for the Juno Cash toolchain.

Includes shared types (events, notes, TxPlan, error codes) for withdrawals, sweeps, and wallet tiering.

Status: work in progress.

## Development

Prereqs:

- Go 1.24+
- Either:
  - `junocashd` and `junocash-cli` in `PATH` (for local integration/e2e), or
  - Docker (for `*-docker` test targets)

Commands:

- Unit tests: `make test`
- Integration tests (local `junocashd -regtest`): `make test-integration`
- Integration tests (Docker): `make test-integration-docker`
- E2E tests (local daemon + CLI): `make test-e2e`
- E2E tests (Docker): `make test-e2e-docker`
- Docker suite: `make test-docker`

Set `JUNO_TEST_LOG=1` to show `junocashd` logs while running tests (it also enables `go test -v` via the Makefile).

## Packages

- `junocashd`: typed JSON-RPC client helpers for `junocashd` (blocks, headers, verbose transaction lookup, tx broadcast)
- `junoscan`: authenticated client helpers for scanner health/epochs, fingerprinted wallet identity, birthday-aware registration, bounded historical backfills, events, notes, address balances, and Orchard witnesses
- `junobroadcast`: typed client helpers for signed transaction submission and status polling
- `types`: shared payload types (TxPlan, DepositEvent, ChainCursor, stable error codes)
