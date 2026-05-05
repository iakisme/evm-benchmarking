# bscbench

Single-host benchmark tool that replays a fixed BSC block window against a
local state snapshot and emits EVM-layer and system-layer metrics to JSON
and CSV. Designed for comparing EVM execution performance across cloud VM
configurations: run the same binary on the same input on two machines,
diff the result.json files.

This is **not** a sync benchmark, **not** a network benchmark, and **not**
a wall-clock benchmark of an entire BSC node. It measures EVM + StateDB
throughput on a stripped, consensus-bypassed execution path.

## Status

- Tested against `github.com/bnb-chain/bsc v1.7.3`
- Go 1.25+
- Linux x86_64

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
docker pull ghcr.io/iakisme/evm-benchmarking:latest
docker run --rm -v "$PWD:/work" -w /work ghcr.io/iakisme/evm-benchmarking:latest replay
```

### From source

```bash
go install github.com/iakisme/evm-benchmarking/cmd/bscbench@latest
```

or

```bash
git clone https://github.com/iakisme/evm-benchmarking
cd evm-benchmarking
go build -o bscbench ./cmd/bscbench
```

## Quickstart

```bash
# 1. Generate a synthetic 40k-block fixture (~2 min, ~92 MiB on disk)
go run ./scripts/prepare-fixture

# 2. Run the canonical double-pass replay (~6 min on a modern x86)
./bscbench replay
# → results/result.json + blocks.csv + proc_samples.csv
```

`bscbench replay` defaults to `--input=testdata/integration/chapel-bench`
and `--out-dir=results`. Override with `--input=<dir>` for a real corpus.

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
data preparation is out of scope for this tool. The spec under
`docs/superpowers/specs/2026-05-02-bsc-benchmark-design.md` describes the
end-to-end intended workflow.

## Output

| File | Content |
|---|---|
| `result.json` | run summary, sysinfo, aggregated metrics, schema_version |
| `blocks.csv` | per-block: gas, exec_ns, trie_commit_ns, SLOAD/SSTORE counts, IO bytes |
| `proc_samples.csv` | 1 Hz (or `--sampler-interval=...`) sampled CPU%, RSS, /proc/self/io |

Two runs with the same `input_hash` and same canonical mode produce
directly comparable result.json files.

## Reference perf

AMD Ryzen 9 8945HS · 16 logical cores · NVMe · Linux 6.14, Go 1.25, BSC v1.7.3
40,000-block synthetic fixture (10 cold-SSTORE writers + 5 transfers per block):

| scheme | mgasps (mean ± std, n=10) | wall (canonical) | fixture size |
|---|---:|---:|---:|
| hash | 126.0 ± 5.2 | ~6 min | 1.5 GiB |
| path | 122.6 ± 3.5 | ~6 min | **92 MiB** |

For real BSC mainnet workloads (mixed cold/warm SSTORE, contract calls,
event-heavy txs), expect 2–3× higher mgasps on the same hardware. The
synthetic above intentionally stresses the cold-write trie-growth path.

## License

LGPL-3.0-or-later. See [`LICENSE`](LICENSE) and [`LICENSE.GPL`](LICENSE.GPL).

This project links against `github.com/bnb-chain/bsc` (LGPL-3.0), so
distributed binaries inherit the LGPL-3.0 obligations: source must be
made available, and downstream consumers must remain able to relink with
a modified BSC.

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
docs/superpowers/    # design spec + implementation plan (historical)
```
