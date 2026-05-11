# bscbench

Single-host benchmark tool that replays a fixed BSC block window against a
local state snapshot and emits EVM-layer and system-layer metrics to JSON
and CSV. Designed for comparing EVM execution performance across cloud VM
configurations: run the same binary on the same input on two machines,
diff the `result.json` files.

This is **not** a sync benchmark, **not** a network benchmark, and **not**
a wall-clock benchmark of an entire BSC node. It measures EVM + StateDB
throughput on a stripped, consensus-bypassed execution path.

## Status

- Tested against `github.com/bnb-chain/bsc v1.7.3`
- Go 1.25+
- Linux x86_64 (other Unix-likes likely work but unverified)

## What it measures

Per replay block bscbench records:

- **`exec_ns`** — wall time from first `ApplyTransaction` to `IntermediateRoot` return
- **`trie_commit_ns`** — wall time of the per-block `IntermediateRoot` alone
- **`state_read_count` / `state_write_count`** — SLOAD / SSTORE counts (via `vm.Config.Tracer`)
- **`db_read_bytes` / `db_write_bytes`** — bytes through a wrapper around `ethdb.Database`

Plus run-level aggregates:

- **`mgasps`** = `total_gas_used / total_wall_sec / 1e6`
- p50/p95/p99 percentiles of every per-block metric above
- /proc-sampled CPU%, RSS peak, disk read/write totals at the configured period

Two runs with the same `input_hash` and same canonical mode produce
directly comparable `result.json` files.

## Install

### From a release

Grab a binary from [Releases](https://github.com/iakisme/evm-benchmarking/releases):

```bash
curl -L https://github.com/iakisme/evm-benchmarking/releases/download/vX.Y.Z/bscbench-vX.Y.Z-linux-amd64.tar.gz \
    | tar -xz
./bscbench-vX.Y.Z-linux-amd64/bscbench version
```

### Container

```bash
# Pull
docker pull ghcr.io/iakisme/evm-benchmarking:latest

# Run against a corpus on the host. --input is required because the
# container has no default fixture inside.
docker run --rm \
    -v "$PWD/my-corpus:/in:ro" \
    -v "$PWD/results:/out" \
    ghcr.io/iakisme/evm-benchmarking:latest \
    replay --input=/in --out-dir=/out
```

### From source

```bash
go install github.com/iakisme/evm-benchmarking/cmd/bscbench@latest
```

or build inside a checkout:

```bash
git clone https://github.com/iakisme/evm-benchmarking
cd evm-benchmarking
go build -o bscbench ./cmd/bscbench
```

## Quickstart

```bash
git clone https://github.com/iakisme/evm-benchmarking
cd evm-benchmarking

# 1. Build the binary (or use `go run` below)
go build -o bscbench ./cmd/bscbench

# 2. Generate a synthetic 40k-block fixture (~2 min, ~92 MiB on disk)
go run ./scripts/prepare-fixture

# 3. Run the canonical double-pass replay (~6 min on a modern x86)
./bscbench replay
# → results/result.json + blocks.csv + proc_samples.csv
```

`bscbench replay` defaults to `--input=testdata/integration/chapel-bench`
and `--out-dir=results`. Override `--input=<dir>` for a real corpus.

For a quick local probe without the warmup pass:

```bash
./bscbench replay --skip-warmup --sampler-interval=50ms
```

## Subcommands

```
bscbench version                        # version of bscbench, BSC dep, Go
bscbench sysinfo --out=sysinfo.json     # one-shot host inventory
bscbench replay  [flags]                # replay a fixed block window
```

`bscbench replay --help` lists every flag.

## Input contract

`bscbench replay --input=<dir>` expects:

```
<input-dir>/
  manifest.json      # schema in internal/corpus/manifest.go
  blocks.rlp         # concatenated RLP-encoded blocks
  state/
    chaindata/       # pebble KV
    ancient/         # freezer (chain + state-history)
```

`scripts/prepare-fixture/main.go` is a self-contained generator that
produces a chapel-compatible synthetic corpus (chain_id 97, block heights
below any BSC fork). It supports both hash-based and path-based state
schemes; default is path:

```bash
go run ./scripts/prepare-fixture                        # path scheme, 40k blocks
go run ./scripts/prepare-fixture --scheme=hash          # hash scheme
go run ./scripts/prepare-fixture --blocks=200 --tx-per-block=2 \
    --out=testdata/integration/chapel-smoke             # CI smoke fixture
```

Real-mainnet replay needs a state snapshot exported from a BSC node — the
data preparation step is out of scope for this tool; only the input
contract above is. See [`docs/design.md`](docs/design.md) for the original
end-to-end design and [`docs/plan.md`](docs/plan.md) for the implementation
plan (historical, captured at time of v0.1 build).

## Output

| File | Content |
|---|---|
| `result.json` | run summary, sysinfo, aggregated metrics, schema_version |
| `blocks.csv` | per-block: gas, exec_ns, trie_commit_ns, SLOAD/SSTORE counts, IO bytes |
| `proc_samples.csv` | sampled CPU%, RSS, `/proc/self/io` at `--sampler-interval` |

## Reference perf

The numbers below come from a 40,000-block **synthetic** fixture
(10 cold-SSTORE writer-contract calls + 5 fresh-account transfers per
block) on AMD Ryzen 9 8945HS · 16 logical cores · NVMe · Linux 6.14,
Go 1.25, BSC v1.7.3, 10 trials each:

| scheme | mgasps (mean ± std) | wall (canonical) | fixture size |
|---|---:|---:|---:|
| hash | 126.0 ± 5.2 | ~6 min | 1.5 GiB |
| path | 122.6 ± 3.5 | ~6 min | **92 MiB** |

This synthetic stresses the **cold-write trie-growth path**:
every SSTORE writes a brand-new slot, so the trie grows monotonically
and trie commit dominates per-block time. Real BSC mainnet workloads
have a much higher hot-modify ratio, fewer cold creates, and richer
opcode mixes — they typically run faster per gas on the same hardware
because trie commit takes constant-ish time once the state is stable.
**Don't infer mainnet capacity from this number.**

## License

LGPL-3.0-or-later. See [`LICENSE`](LICENSE) and [`LICENSE.GPL`](LICENSE.GPL).

This project statically links against `github.com/bnb-chain/bsc`, which
is itself LGPL-3.0. Distributed binaries inherit LGPL-3.0 obligations:
source must be made available, and downstream consumers must remain able
to relink with a modified BSC.

`bsc`, `BNB Smart Chain` and related names are referenced descriptively
in this project as the blockchain it benchmarks; the project is not
affiliated with or endorsed by BNB Chain.

## Layout

```
cmd/bscbench/        # CLI: version / sysinfo / replay
internal/
  chain/             # consensus-bypassed BSC state DB + ApplyBlock
  corpus/            # input loader (manifest + blocks.rlp + state/)
  metrics/           # EVM tracer, /proc sampler, percentile aggregator
  report/            # JSON / CSV writers
  runner/            # double-pass coordinator (warmup → measured)
  sysinfo/           # host inventory (CPU, mem, disk, cloud metadata)
scripts/prepare-fixture/   # synthetic corpus generator
docs/design.md       # design spec
docs/plan.md         # implementation plan (historical)
```

## Contributing

Bug reports and small PRs welcome. For larger changes, open an issue
first to talk through the approach — the project intentionally keeps a
narrow scope (single-host EVM benchmark; no orchestration, no sync, no
aggregation).
