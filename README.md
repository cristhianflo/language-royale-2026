# Language Royale 2026

Go implementation of Problem A: a decisioning microservice with `POST /score` and `GET /health`.

## Requirements

- Go `1.25.8`
- `k6` for the API benchmark

## Run the API

Start the server on `http://localhost:8080`:

```bash
go run ./cmd/api
```

Or build a binary first:

```bash
go build -o bin/api ./cmd/api
./bin/api
```

## Endpoints

### `GET /health`

Returns:

```json
{ "ok": true }
```

### `POST /score`

Example request:

```json
{
  "case_id": "C123",
  "signals": {
    "lpr_hits_24h": 3,
    "address_match": true,
    "distance_miles": 2.4,
    "days_since_last_seen": 1
  }
}
```

Example response:

```json
{ "case_id": "C123", "score": 0.87, "tier": "HOT" }
```

## Validation Rules

Valid requests return `200`.

Invalid requests return `400`.

Rules:

- `case_id`: required, string, trimmed length `>= 1`
- `signals`: required, object
- `signals.lpr_hits_24h`: integer in `[0, 10000]`
- `signals.address_match`: boolean
- `signals.distance_miles`: number in `[0, 10000]`
- `signals.days_since_last_seen`: integer in `[0, 36500]`
- extra keys are allowed

## Run Tests

Run the API tests:

```bash
go test ./cmd/api
```

Current tests include:

- `GET /health`
- fixture-driven `POST /score` checks using the first 10 lines from `api_testcases.jsonl/api_testcases.jsonl`

## Run the API Benchmark

Start the API first:

```bash
go run ./cmd/api
```

Then run the k6 benchmark from the repo root:

```bash
k6 run benchmarks/api_load.js
```

The benchmark reads input cases from:

```text
hack/problem_a_generate.jsonl
```

It validates:

- expected status code
- `case_id`
- `tier`
- `score`

## Notes

- The API uses the scoring formula from the discussion and rounds scores to 2 decimal places.
- The k6 script treats both `200` and `400` as expected HTTP statuses because invalid fixtures are part of the benchmark dataset.
