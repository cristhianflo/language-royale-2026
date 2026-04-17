#!/usr/bin/env python3
from __future__ import annotations

import argparse
import asyncio
import json
import time
from collections import Counter, defaultdict
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

import aiohttp


@dataclass
class TestCase:
    expected_status_code: int
    input_body: Dict[str, Any]
    expected_output: Optional[Dict[str, Any]]


@dataclass
class Result:
    expected_status_code: int
    actual_status: Optional[int]  # None if request failed before response
    ok_status: bool  # 2xx
    matched_status: bool
    matched_output: bool
    matched_expectation: bool
    latency_ms: float
    t_end: float
    error: Optional[str] = None


def compare_json(expected: Any, actual: Any) -> bool:
    return expected == actual


def percentile(sorted_vals: List[float], p: float) -> float:
    if not sorted_vals:
        return float("nan")
    if p <= 0:
        return sorted_vals[0]
    if p >= 100:
        return sorted_vals[-1]
    k = (len(sorted_vals) - 1) * (p / 100.0)
    f = int(k)
    c = min(f + 1, len(sorted_vals) - 1)
    if f == c:
        return sorted_vals[f]
    return sorted_vals[f] + (sorted_vals[c] - sorted_vals[f]) * (k - f)


async def worker(
    session: aiohttp.ClientSession,
    url: str,
    q: "asyncio.Queue[TestCase]",
    timeout_s: float,
    results: List[Result],
    stop_at: float,
) -> None:
    while True:
        if time.perf_counter() >= stop_at:
            return

        try:
            tc = q.get_nowait()
        except asyncio.QueueEmpty:
            await asyncio.sleep(0.001)
            continue

        t0 = time.perf_counter()
        try:
            async with session.post(url, json=tc.input_body, timeout=timeout_s) as resp:
                raw_body = await resp.read()
                t1 = time.perf_counter()
                actual = resp.status
                matched_status = actual == tc.expected_status_code
                matched_output = True

                if matched_status and tc.expected_output is not None:
                    try:
                        actual_json = json.loads(raw_body)
                    except json.JSONDecodeError:
                        matched_output = False
                    else:
                        matched_output = compare_json(tc.expected_output, actual_json)

                matched = matched_status and matched_output
                results.append(
                    Result(
                        expected_status_code=tc.expected_status_code,
                        actual_status=actual,
                        ok_status=(200 <= actual < 300),
                        matched_status=matched_status,
                        matched_output=matched_output,
                        matched_expectation=matched,
                        latency_ms=(t1 - t0) * 1000.0,
                        t_end=t1,
                    )
                )
        except Exception as e:
            t1 = time.perf_counter()
            # Treat request failures as a mismatch (no status)
            results.append(
                Result(
                    expected_status_code=tc.expected_status_code,
                    actual_status=None,
                    ok_status=False,
                    matched_status=False,
                    matched_output=False,
                    matched_expectation=False,
                    latency_ms=(t1 - t0) * 1000.0,
                    t_end=t1,
                    error=type(e).__name__,
                )
            )
        finally:
            q.task_done()


def bucket_max_rps(results: List[Result]) -> float:
    if not results:
        return 0.0
    buckets = defaultdict(int)
    for r in results:
        buckets[int(r.t_end)] += 1
    return float(max(buckets.values()))


def print_report(results: List[Result], started_at: float, ended_at: float) -> None:
    total_s = ended_at - started_at
    latencies = [r.latency_ms for r in results]
    lat_sorted = sorted(latencies)
    avg = (sum(latencies) / len(latencies)) if latencies else float("nan")
    p50 = percentile(lat_sorted, 50)
    p95 = percentile(lat_sorted, 95)

    total_count = len(results)
    rps = (total_count / total_s) if total_s > 0 else 0.0
    max_rps = bucket_max_rps(results)

    status_counts = Counter(
        [r.actual_status for r in results if r.actual_status is not None]
    )
    error_counts = Counter([r.error for r in results if r.error is not None])

    matched = sum(1 for r in results if r.matched_expectation)
    matched_status = sum(1 for r in results if r.matched_status)
    matched_output = sum(1 for r in results if r.matched_output)
    match_rate = (matched / total_count) if total_count else 0.0

    # mismatch matrix
    matrix = Counter()
    for r in results:
        exp = r.expected_status_code
        act = r.actual_status if r.actual_status is not None else "NO_RESPONSE"
        if act != exp:
            matrix[(exp, act)] += 1

    print("\n=== Load Test Report (with expectations) ===")
    print(f"Total requests: {total_count:,}")
    print(f"Total time (wall): {total_s:.3f}s")
    print(f"Avg latency: {avg:.2f} ms")
    print(f"p50 latency: {p50:.2f} ms")
    print(f"p95 latency: {p95:.2f} ms")
    print(f"Avg req/sec: {rps:.2f}")
    print(f"Max req/sec (peak 1s bucket): {max_rps:.2f}")

    print("\nExpectation check:")
    print(
        f"Matched expected status: {matched_status:,}/{total_count:,} "
        f"({(matched_status / total_count) if total_count else 0.0:.2%})"
    )
    print(
        f"Matched expected output: {matched_output:,}/{total_count:,} "
        f"({(matched_output / total_count) if total_count else 0.0:.2%})"
    )
    print(f"Matched full expectation: {matched:,}/{total_count:,} ({match_rate:.2%})")

    if status_counts:
        print("\nStatus codes observed:")
        for k in sorted(status_counts.keys()):
            print(f"  {k}: {status_counts[k]:,}")

    if matrix:
        print("\nMismatches (expected -> actual):")
        for (exp, act), cnt in matrix.most_common(20):
            print(f"  {exp} -> {act}: {cnt:,}")

    if error_counts:
        print("\nErrors:")
        for k, v in error_counts.most_common():
            print(f"  {k}: {v:,}")


async def main_async() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", default="http://localhost:8000/score")
    ap.add_argument("--testcases", default="problem_a_generate.jsonl")
    ap.add_argument("--concurrency", type=int, default=100)
    ap.add_argument("--duration", type=float, default=60.0)
    ap.add_argument("--timeout", type=float, default=5.0)
    ap.add_argument("--max_cases", type=int, default=0, help="0 = all")
    args = ap.parse_args()

    # Load testcases
    cases: List[TestCase] = []
    with open(args.testcases, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rec = json.loads(line)
            cases.append(
                TestCase(
                    expected_status_code=int(rec["status_code"]),
                    input_body=rec["input"],
                    expected_output=rec["output"],
                )
            )

    if args.max_cases and args.max_cases > 0:
        cases = cases[: args.max_cases]

    q: asyncio.Queue[TestCase] = asyncio.Queue()
    for tc in cases:
        q.put_nowait(tc)

    results: List[Result] = []
    connector = aiohttp.TCPConnector(limit=0, ttl_dns_cache=300)

    started_at = time.perf_counter()
    stop_at = started_at + args.duration

    async with aiohttp.ClientSession(connector=connector) as session:
        workers = [
            asyncio.create_task(
                worker(session, args.url, q, args.timeout, results, stop_at)
            )
            for _ in range(args.concurrency)
        ]
        await asyncio.gather(*workers)

    ended_at = time.perf_counter()
    print_report(results, started_at, ended_at)


def main() -> None:
    asyncio.run(main_async())


if __name__ == "__main__":
    main()
