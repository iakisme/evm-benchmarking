# BSC EVM Benchmark — Design Spec

**Date**: 2026-05-02
**Author**: kai
**Status**: Draft, pending implementation plan

## 1. Goal

Build a single Go binary, `bscbench`, that replays a fixed 10,000-block window on a local BSC state snapshot, produces EVM-layer and coarse system-layer metrics, and writes them to JSON/CSV. The intended use is comparing EVM execution performance across different cloud VM configurations by running the same binary on the same input dataset across machines and diffing the outputs.

Non-goals:

- Cross-machine orchestration, aggregation, or visualization (single-host tool only).
- Synthetic in-process workloads (dropped — see §2).
- Sync-from-P2P or any network-dependent workload.
- Data preparation (pre-built corpus is an input contract; user prepares it out-of-band).

## 2. Scope and Non-Scope Decisions

| Area | Decision |
|---|---|
| Workload | Historical block replay only. `synth` subcommand dropped. User may generate synthetic blocks offline and feed them through the replay path. |
| BSC integration | Import `github.com/bnb-chain/bsc` as a Go module dependency, pinned to a specific upstream tag. No fork. |
| Consensus | Bypassed. Blocks are executed via a stripped `core.StateProcessor`-equivalent path — no header verification, no `Finalize`, no system-tx replay. Benchmark measures pure EVM + StateDB throughput. |
| Dataset | Deferred to user. Tool accepts a local input directory with a fixed contract (§5). |
| Window | 10,000 blocks by default, canonical unit of comparison. `--from/--to` allow override but non-canonical runs are marked. |
| Output | Local files only. `result.json` + `blocks.csv` + `proc_samples.csv`. No push to external systems. |
| Orchestration | Out of scope. Each machine runs `bscbench replay` independently; user diffs JSONs offline. |

## 3. Architecture

### 3.1 Binary surface

```
bscbench replay   --input=<dir> --out-dir=<dir> [--from=N --to=M] [--skip-warmup]
bscbench sysinfo  --out=<file>
bscbench version
```

`replay` is the primary command. `sysinfo` is redundant with `replay` (replay always captures sysinfo inline) but useful for standalone host inventory. `version` emits bscbench version + resolved BSC dependency version.

### 3.2 Package layout

```
cmd/bscbench/            # main, cobra command assembly
internal/
  chain/                 # BSC BlockChain + StateDB construction (consensus-bypassed)
  corpus/                # local input loader: manifest + blocks.rlp + state/ validation
  runner/                # orchestrates double-pass execution and timing
  metrics/               # EVM hooks + /proc sampler + aggregation
  sysinfo/               # host, cpu, memory, disk, go runtime, cloud metadata
  report/                # JSON/CSV schema and writers
```

Dependencies are strictly unidirectional: `cmd → runner → {chain, corpus, metrics, report, sysinfo}`. `corpus` depends on BSC types only for block RLP decoding. `chain` and `metrics` both touch BSC interfaces but do not depend on each other.

### 3.3 Component responsibilities

- **chain**: opens BSC's on-disk DB (pebble) as a `*state.Database`, constructs enough of a `BlockChain`-equivalent context (chain config, genesis, fork schedule) to execute transactions. Does not start any consensus engine. Exposes `(stateDB, chainConfig, blockContextFor(block))` to the runner.
- **corpus**: reads `manifest.json`, streams `blocks.rlp`, opens `state/`. Validates the triple at load time (manifest schema, block count, first block parentHash vs. state head, state root at from-block matches `manifest.expectedStateRootAtFrom`). Validation failures are hard stops.
- **runner**: owns the double-pass control flow (§4). Iterates blocks from the corpus, applies transactions to the state, drives the metrics collector, reports progress.
- **metrics**: three collectors — EVM tracer (per-tx SLOAD/SSTORE counts via `vm.Config.Tracer`), state hook (per-block `exec_ns` and `trie_commit_ns` wrapping `ApplyTransaction` and `IntermediateRoot`), /proc sampler (goroutine sampling `/proc/self/stat`, `/proc/self/status`, `/proc/self/io` at 1 Hz).
- **sysinfo**: one-shot host inventory. Cloud metadata is best-effort (500 ms timeout, nil on failure).
- **report**: serializes run-level JSON, block-level CSV, and /proc sample CSV to disk. All field names and units fixed in the schema; uses `schema_version` for forward compatibility.

### 3.4 Execution path (single, unified)

There is one execution path. Replay consumes blocks from `corpus`, calls an internal `applyBlock(block, stateDB)` that iterates `block.Transactions()` invoking the BSC equivalent of `core.ApplyTransaction` (consensus bypassed as described in §2), then calls `StateDB.IntermediateRoot`. No `BlockChain.InsertChain`, no `Finalize`, no header verification. System transactions (validator rotation etc.) are detected by convention (`to == systemAddress`) and either skipped or executed but tagged in metrics — exact handling TBD at implementation time based on whether BSC's `core.ApplyTransaction` works cleanly without `Finalize` context.

## 4. Double-Pass Warmup

Each `bscbench replay` invocation runs the full 10,000-block window twice:

```
1. cp --reflink=auto state/ workdir_A/   (fallback: cp -r, with warning)
2. chainA := chain.Open(workdir_A)
3. runner.Run(chainA, blocks, measured=false)    # warmup, discard metrics
4. chainA.Close(); rm -rf workdir_A
5. cp --reflink=auto state/ workdir_B/
6. chainB := chain.Open(workdir_B)
7. runner.Run(chainB, blocks, measured=true)     # record metrics
8. chainB.Close(); rm -rf workdir_B
```

Rationale: the user wants measured runs to observe hot OS page cache and hot Go runtime state. Rewinding state between passes preserves comparability of the measured block range while keeping page cache warm. BSC's internal trie/bloom caches do get rebuilt between passes (they're in-process), but the DB files and their backing pages stay hot.

Cost: two state copies (~7 min each for a 400 GB snapshot on NVMe without reflink) plus two full replays. Total per-run budget: 60–90 minutes on a modern VM.

`--skip-warmup` is available for debugging but produces runs that are **not** comparable to canonical measurements; the output records `skip_warmup: true` and downstream comparison tooling should reject cross-comparing skip-warmup runs against normal runs.

`warmup_state` in the output is always `"warm"` for canonical runs.

## 5. Input Contract

`bscbench replay --input=<dir>` expects exactly this layout:

```
<input-dir>/
  manifest.json
  blocks.rlp
  state/
    chaindata/
    ancient/
```

`manifest.json` schema (all fields required except where marked):

```json
{
  "schema_version": "1",
  "chain_id": 56,
  "from_block": 40000000,
  "to_block": 40010000,
  "block_count": 10000,
  "expected_state_root_at_from": "0x…",
  "fork_schedule": { "…": 0 },
  "generator": "mainnet-export" | "synthetic-<name>" | "…",
  "generated_at": "2026-05-02T00:00:00Z",
  "input_hash": "sha256:…",           // sha256 over blocks.rlp + state tarball
  "bsc_version_recommended": "v1.4.8" // optional, warns on mismatch
}
```

Validation at load (all hard stops):

1. `manifest.json` parses and all required fields present.
2. `blocks.rlp` decodes cleanly block-by-block; count equals `block_count`.
3. `state/` opens successfully as a pebble DB + ancient store.
4. Computed state root at `from_block` equals `expected_state_root_at_from`.
5. First block's `parentHash` matches the hash of the block whose state root is the head of `state/`.

The tool does not attempt to repair or warn-and-continue on any of these. A dataset that fails validation is rejected.

The tool is agnostic to how data was produced (mainnet export, synthetic generator, hand-crafted). The input contract is the only interface.

## 6. Metrics

### 6.1 Run-level (one record per replay)

| Field | Definition |
|---|---|
| `mgasps` | `total_gas_used / total_wall_sec / 1e6` |
| `total_gas_used` | sum of `receipt.GasUsed` |
| `total_tx_count` | count of transactions in replayed blocks |
| `reverted_tx_count` | count where `receipt.Status == Failed` |
| `total_wall_sec` | wall clock span of the measured pass |
| `tx_per_sec` | `total_tx_count / total_wall_sec` |
| `block_per_sec` | `block_count / total_wall_sec` |
| `exec_ns_{p50,p95,p99}` | percentiles of per-block `exec_ns` |
| `trie_commit_ns_{p50,p95,p99}` | percentiles of per-block `trie_commit_ns` |
| `gas_used_per_block_{p50,p95,p99}` | percentiles of per-block gas |
| `cpu_pct_{avg,max}` | aggregates from /proc sampler; 100% = one logical core |
| `rss_peak_bytes` | max `VmRSS` observed in samples |
| `disk_{read,write}_total_bytes` | delta from first to last sample of `/proc/self/io` |
| `disk_{read,write}_MBps_avg` | total / `total_wall_sec` |

### 6.2 Block-level (`blocks.csv`, 10,000 rows)

```
block_number, tx_count, gas_used, exec_ns, state_read_count,
state_write_count, trie_commit_ns, db_read_bytes, db_write_bytes
```

- `exec_ns`: wall time from first `ApplyTransaction` call for the block to return of `IntermediateRoot`.
- `trie_commit_ns`: wall time of `IntermediateRoot` alone. Note: in BSC the EVM may trigger intermediate commits inside a transaction; `trie_commit_ns` captures only the outer per-block call. Implementation detail confirmed against upstream source.
- `state_read_count` / `state_write_count`: SLOAD / SSTORE counts from the EVM tracer.
- `db_read_bytes` / `db_write_bytes`: counted by wrapping the `ethdb.Database` handle.

### 6.3 System sampling (`proc_samples.csv`, 1 Hz)

```
ts_ms, cpu_pct, rss_bytes, disk_read_cum_bytes, disk_write_cum_bytes
```

Cumulative bytes are raw from `/proc/self/io` — these represent syscall-view and will exceed actual device I/O when pages hit the OS cache. Retaining both the cumulative counters and the run-level deltas lets downstream analysis see the cache-effectiveness signal directly.

### 6.4 Instrumentation points

- `ethdb.Database` wrapper: read/write count and bytes.
- `vm.Config.Tracer`: SLOAD/SSTORE counts, opcode fan-out (not stored per-opcode; aggregated).
- Around `ApplyTransaction` batch per block: block-level timing.
- Around `IntermediateRoot` / `Commit`: trie commit timing.
- Background goroutine at 1 Hz: /proc files.

### 6.5 Explicit non-instrumentation

- No per-opcode histogram (profiling territory, not benchmark).
- No flame graphs (optional `--pprof-addr` flag exposes the standard `net/http/pprof` endpoint for users who want to drill in).
- No network I/O sampling (replay path has none).

## 7. Output

Three files written to `--out-dir`:

- `result.json`: run summary, sysinfo inline, aggregated metrics. Schema below.
- `blocks.csv`: per-block metrics (§6.2).
- `proc_samples.csv`: per-sample system metrics (§6.3).

`result.json` top-level shape:

```json
{
  "schema_version": "1",
  "run": {
    "id": "<ISO8601>_bsc10k_<hostname>_<input_hash_short>",
    "started_at": "…",
    "finished_at": "…",
    "bscbench_version": "…",
    "bsc_version": "…",
    "input_hash": "sha256:…",
    "from_block": …,
    "to_block": …,
    "block_count": 10000,
    "warmup_state": "warm",
    "skip_warmup": false
  },
  "sysinfo": { /* see §8 */ },
  "metrics": { /* §6.1 */ },
  "passes": {
    "warmup":   { "wall_sec": …, "gas_used": … },
    "measured": { /* mirrors metrics for easy access */ }
  }
}
```

Encoding conventions:

- UTF-8, no compression.
- All time durations are nanoseconds, integer.
- All byte counts are bytes, integer.
- All timestamps are RFC 3339 UTC.
- `run.id` is deterministic given inputs and host; two measured runs on the same host with the same dataset within the same second collide — acceptable given this tool is not run in tight loops.

## 8. System Inventory

`sysinfo` block in `result.json`:

```json
{
  "host": { "hostname": "…", "kernel": "…", "os": "…", "uptime_sec": … },
  "cpu": {
    "model": "…", "cores_physical": …, "cores_logical": …,
    "flags_subset": ["avx2", "avx512f", "sse4_2"],
    "governor": "performance", "mhz_base": …
  },
  "memory": { "total_bytes": …, "swap_bytes": …, "hugepages_total": … },
  "disk": [
    {
      "device": "/dev/nvme0n1", "model": "…", "size_bytes": …,
      "fs": "ext4", "mount": "/data", "rotational": false,
      "queue_scheduler": "none", "discard_max_bytes": …
    }
  ],
  "go": { "version": "go1.22.3", "gomaxprocs": …, "gogc": 100 },
  "cloud": {
    "provider": "aws" | "aliyun" | "tencent" | "gcp" | null,
    "instance_type": "…", "az": "…", "region": "…"
  }
}
```

Cloud metadata detection:

- Probes each provider's metadata endpoint in parallel with a 500 ms timeout.
- First successful probe wins.
- All failures → `"cloud": null`. This is expected on bare-metal and local dev hosts.

The `disk` array covers all devices the process touches — at minimum the mount holding `state/` and the mount holding `--out-dir` (if different).

## 9. Testing Strategy

Three layers, distinct purposes:

**Unit tests** (`go test ./...`, fast, always run):

- `corpus`: synthetic manifests and 3–5 block RLP fixtures; cover schema validation, root-mismatch, block-count mismatch, parentHash breakage.
- `metrics`: mocked tracer event streams; assert per-block records, percentile math, CSV serialization via golden files.
- `sysinfo`: `/proc` files from `testdata/`; cloud metadata via `httptest.Server`.
- `report`: given a metrics struct, assert JSON and CSV output byte-for-byte against golden files. The CI-visible schema stability is pinned here.

**Integration tests** (`go test -tags=integration`, slower, optional):

- A small fixture: 50 BSC testnet blocks plus the corresponding state snapshot, tarred into `testdata/integration/` (targeting ~50 MB committed).
- Run a full replay with `--skip-warmup` to keep it under a minute.
- Assert: run completes without panic, output schema validates, `mgasps > 0`, `block_count == 50`.
- Does not assert specific `mgasps` values (would drift across CI runners).

**Baseline regression** (manual/cron, not in CI):

- One canonical dataset, one canonical baseline host.
- Run after any BSC dependency bump or non-trivial bscbench change.
- Compare `mgasps` to the last recorded value; > 5% drift opens an issue.
- Not CI-gated — dataset and runtime are too large.

## 10. Open Questions

- **Dataset sourcing**: out of scope for this tool; user-prepared. The tool's only contract is the input directory (§5). A separate `scripts/prepare-dataset/` area may later hold helper scripts, but that is a distinct project.
- **System transaction handling under consensus bypass**: need to confirm at implementation time whether skipping `Finalize` causes any transactions to apply incorrectly. If so, a minimal replacement for the validator-rotation logic may be required. Documented here so the implementer tests this case explicitly on a known-good block range.
- **BSC fork schedule tracking**: every BSC hard fork changes `ChainConfig`. Dataset `manifest.json` encodes the fork schedule used during preparation; bscbench compares against its compiled-in BSC version's default config and warns on mismatch. Version-upgrade discipline for bscbench itself is to rerun the baseline regression after every BSC dependency bump.

## 11. Summary of Decisions

| # | Decision |
|---|---|
| 1 | Single binary, single execution path (replay only). |
| 2 | BSC imported as Go module; no fork. |
| 3 | Consensus bypassed; stripped `StateProcessor`-equivalent execution. |
| 4 | Input is a local directory with a fixed contract. Dataset preparation is out of scope. |
| 5 | Default window is 10,000 blocks. |
| 6 | Warmup strategy: double-pass with state reset between passes. |
| 7 | Metrics: EVM layer (mgasps, per-block timing, state & DB counters) + /proc sampler (1 Hz, CPU/RSS/IO). |
| 8 | Output: `result.json` (primary) + `blocks.csv` + `proc_samples.csv`. No compression, no cross-machine aggregation. |
| 9 | Sysinfo inline in `result.json`, cloud metadata best-effort. |
| 10 | Testing: unit + integration (small fixture) + manual baseline regression. |
