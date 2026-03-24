# Load Testing Results

## Scenario

- Endpoint under test: `GET /rooms/{roomId}/slots/list?date=YYYY-MM-DD`
- Method: Go benchmark (`BenchmarkSlotsListParallel`) with parallel requests
- Command:

```bash
make loadtest
```

## Latest run

- Date: 2026-03-24
- Environment: local machine (darwin/arm64)
- Raw output:

```text
BenchmarkSlotsListParallel-8   	  831678	     24310 ns/op	   24378 B/op	     238 allocs/op
```

- Estimated throughput: ~41k req/s (`1s / 24.31us`)
- Estimated average latency: ~0.024 ms per request (benchmark ns/op)

## Notes

- This is an in-process benchmark with `httptest` + `MemoryStore`; it measures handler-layer performance without network/DB/container overhead.
- For production-like numbers, run an external load tool (`k6`/`vegeta`) against `docker compose` deployment with PostgreSQL.
