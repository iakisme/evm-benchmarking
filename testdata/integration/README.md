# Test / Bench Fixtures

Two synthetic chapel fixtures live under this directory. **Neither is
committed** (pebble metadata churns across regenerations).

| Path | Size | Purpose |
|---|---|---|
| `chapel-smoke/` | ~1 MiB, 200 blk × 2 tx | gated integration test (`-tags=integration`) |
| `chapel-bench/` | ~1.5 GiB, 40k blk × 15 tx | local perf run; default for `evmbench replay` |

## Regenerate

The script defaults to the bench fixture:

    go run ./scripts/prepare-fixture                                            # → chapel-bench (~2 min, ~1.5 GiB)

Override for the smoke fixture used by the integration test:

    go run ./scripts/prepare-fixture --blocks=200 --tx-per-block=2 \
        --out=testdata/integration/chapel-smoke                                  # → chapel-smoke (<1 s)

## Run

```
# canonical double-pass replay against the bench fixture (no flags needed)
evmbench replay
```

Defaults: `--input=testdata/integration/chapel-bench --out-dir=results`.

```
# integration test against the smoke fixture
go test -tags=integration ./cmd/evmbench/ -run TestReplayIntegrationChapel -v
```

## Workload (both fixtures)

Each replay block generates a mix of:

- **N writer-contract calls** — 8-byte fallback that reads slot 0
  (counter), increments it, writes back (warm SSTORE), writes a unique
  cold slot derived from calldata, emits one `LOG1`. ~50k gas/call.
- **M value transfers** — to fresh per-block-and-index recipient EOAs.
  ~21k gas each.

Writer-to-transfer ratio is fixed at ~67/33; default 10/5 per block at
15 tx/block. Each block also exercises one `SLOAD`, two `SSTORE`s, and
one `LOG1` per writer call, so cold-storage growth, warm-modify, and
log emission paths are all covered.

This is **not** a check on real BSC mainnet behavior. The fixture
produces stable structure for measuring evmbench itself; for real
performance signals against mainnet workloads, prepare a corpus from a
real BSC node following the spec input contract.

## Reference timings (AMD Ryzen 9 8945HS, NVMe, Linux 6.14)

| Fixture | wall (canonical) | mgasps |
|---|---:|---:|
| smoke (200 blk) | <1 s (skip-warmup default in test) | ~530 |
| bench (40k blk) | ~6 min | ~130 |

Slower hosts will run longer; the canonical double-pass is faithfully
reflecting trie-commit growth as state expands.
