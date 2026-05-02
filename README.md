# bscbench

Single-host benchmark tool that replays a fixed 10,000-block BSC window and
emits EVM and system metrics to JSON/CSV. Designed to compare EVM execution
performance across cloud VM configurations.

See `docs/superpowers/specs/2026-05-02-bsc-benchmark-design.md` for the design
spec and `docs/superpowers/plans/2026-05-02-bsc-benchmark.md` for the
implementation plan.

## Build

    go build -o bscbench ./cmd/bscbench

## Usage

    bscbench version
    bscbench sysinfo --out=sysinfo.json
    bscbench replay  --input=<dir> --out-dir=<dir>
