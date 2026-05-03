# Integration Test Fixture

The `cmd/bscbench` integration test (gated by `-tags=integration`) expects a
small, real BSC corpus at:

    testdata/integration/chapel-50blocks/
      manifest.json
      blocks.rlp
      state/
        chaindata/
        ancient/

This fixture is **not committed** to the repo (size, churn). To regenerate:

1. Bring up a BSC chapel (testnet) node and snap-sync to a recent height.
2. Stop at height N.
3. Export 50 blocks: `geth --datadir=<dir> export blocks.rlp N N+50`
4. Pack `chaindata/` and `ancient/` into `state/`.
5. Generate `manifest.json` matching the expected schema (see
   `internal/corpus/testdata/manifest_valid.json`).
6. Compute `input_hash` as `sha256` over `blocks.rlp` and the tarred state.

Run the integration test:

    go test -tags=integration ./cmd/bscbench/ -run TestReplayIntegrationChapel -v
