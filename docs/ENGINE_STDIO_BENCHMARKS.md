# Local Stdio Engine Benchmark Baseline

Status: candidate-only regression evidence. These numbers are not service
objectives and are not CI thresholds.

Snapshot: 2026-07-12, Apple M1 (`darwin/arm64`), Go 1.26.5, no live
credentials. Fixtures are deterministic and setup is outside timed regions
where the benchmark name describes only preflight or streaming work.

## Reproduce

```sh
go test ./internal/enginehost -run '^$' -bench . -benchmem -benchtime=1s -count=5
go test ./cmd/zscalerctl-engine -run '^$' -bench BenchmarkProcess -benchmem -benchtime=20x -count=5
```

The package benchmarks live beside the host and process tests. They cover a
full in-process handshake/request lifecycle, resource milestones,
cancel-to-terminal latency, 1,000-record preflight, an 8 MiB fragmented item,
real child-process startup, and a config-free manifest lifecycle.

## Initial baseline

Medians across five benchmark runs unless noted:

| Measurement | Baseline |
| --- | ---: |
| real process: start to `hello` | 11.25 ms |
| real process: start through manifest `completed` | 11.81 ms |
| in-process host: start to `hello` | 30.23 µs |
| in-process host: start to `ready` | 109.66 µs |
| in-process host: start to `started` | 159.03 µs |
| in-process host: start to first resource item | 189.47 µs |
| in-process host: start to resource `completed` | 196.93 µs |
| in-process host: cancel to `canceled` terminal | 25.95 µs |
| 1,000-record whole-response preflight | 8.47 ms; 5.72 MB; 116,072 allocations |
| 8 MiB item post-commit encode/fragment throughput | 81.69 MB/s; 80.42 MB cumulative allocations |
| real process steady RSS, 20-run probe | 15.87 MiB median; 17.20 MiB maximum |

The process benchmark includes OS process creation and scheduler variance; cold
or contended outliers are expected. The fragmented-item allocation figure is
cumulative work across bounded 512 KiB chunks, not an 80 MiB live-memory claim.
The protocol permits each semantic item to reach 64 MiB but has no total
response-byte limit. Preflight therefore retains one payload for every
fragmented admitted item, and peak memory can scale with the number of such
items; the measured case contains one 8 MiB item. Future optimizations must
preserve strict validation, exact-number handling, the all-items success
barrier, digest verification, and fail-closed output policy.
