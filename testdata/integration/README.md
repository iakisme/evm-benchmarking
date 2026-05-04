# Integration Test Fixture

The `cmd/bscbench` integration test (gated by `-tags=integration`) expects a
small BSC corpus at:

    testdata/integration/chapel-50blocks/
      manifest.json
      blocks.rlp
      state/
        chaindata/
        ancient/

This fixture is **not committed** to the repo (pebble metadata churns across
regenerations). Build it locally with:

    go run ./scripts/prepare-fixture --out=testdata/integration/chapel-50blocks

The script generates a synthetic 50-block chapel-compatible chain
(chain_id=97, block heights well below any BSC fork) using
`core.GenerateChain` with a faked ethash engine. It is purely structural —
the goal is to exercise bscbench's pipeline (corpus loader, state DB open,
ApplyBlock, metrics, report writers) end-to-end. It is **not** a check on
real BSC mainnet behavior; for that, prepare a corpus from a real BSC node
following the spec's input contract.

Run the integration test:

    go test -tags=integration -v ./cmd/bscbench/ -run TestReplayIntegrationChapel

Without the fixture, the test self-skips with an `os.Stat` error message.
