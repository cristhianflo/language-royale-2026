#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import random
import string
from collections import Counter, defaultdict
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple


def is_nonempty_str(x: Any) -> bool:
    return isinstance(x, str) and x.strip() != ""


def is_number(x: Any) -> bool:
    return isinstance(x, (int, float)) and not isinstance(x, bool)


def parse_ts(ts: str) -> Optional[datetime]:
    if not is_nonempty_str(ts):
        return None
    try:
        return datetime.strptime(ts, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    except ValueError:
        return None


def validate_record(record: Any) -> Tuple[bool, str]:
    if not isinstance(record, dict):
        return False, "record_not_object"

    if parse_ts(record.get("ts")) is None:
        return False, "ts_invalid"

    if not is_nonempty_str(record.get("camera_id")):
        return False, "camera_id_invalid"

    if not is_nonempty_str(record.get("plate")):
        return False, "plate_invalid"

    vin8 = record.get("vin8")
    if not is_nonempty_str(vin8) or len(vin8.strip()) != 8:
        return False, "vin8_invalid"

    zip_code = record.get("zip")
    if not (isinstance(zip_code, str) and len(zip_code) == 5 and zip_code.isdigit()):
        return False, "zip_invalid"

    confidence = record.get("confidence")
    if not is_number(confidence):
        return False, "confidence_type"
    if not (0.0 <= float(confidence) <= 1.0):
        return False, "confidence_range"

    return True, "ok"


def format_ts(dt: datetime) -> str:
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def minute_bucket(dt: datetime) -> str:
    return (
        dt.astimezone(timezone.utc)
        .replace(second=0, microsecond=0)
        .strftime("%Y-%m-%dT%H:%M:%SZ")
    )


def sorted_top_counts(
    counter: Counter[str], key_name: str, limit: int
) -> List[Dict[str, Any]]:
    items = sorted(counter.items(), key=lambda item: (-item[1], item[0]))[:limit]
    return [{key_name: key, "hit_count": count} for key, count in items]


def aggregate_records(records: List[Dict[str, Any]]) -> Dict[str, Any]:
    plate_counts: Counter[str] = Counter()
    zip_counts: Counter[str] = Counter()
    minute_counts: Counter[str] = Counter()
    camera_totals: Counter[str] = Counter()
    camera_confidence_sums: defaultdict[str, float] = defaultdict(float)

    for record in records:
        ts = parse_ts(record["ts"])
        assert ts is not None

        plate_counts[record["plate"]] += 1
        zip_counts[record["zip"]] += 1
        minute_counts[minute_bucket(ts)] += 1
        camera_totals[record["camera_id"]] += 1
        camera_confidence_sums[record["camera_id"]] += float(record["confidence"])

    camera_stats = []
    for camera_id in sorted(camera_totals):
        total_hits = camera_totals[camera_id]
        average_confidence = round(camera_confidence_sums[camera_id] / total_hits, 4)
        camera_stats.append(
            {
                "camera_id": camera_id,
                "total_hits": total_hits,
                "average_confidence": average_confidence,
            }
        )

    busiest_minutes = [
        {"minute": minute, "hit_count": count}
        for minute, count in sorted(
            minute_counts.items(), key=lambda item: (-item[1], item[0])
        )[:5]
    ]

    return {
        "top_plates": sorted_top_counts(plate_counts, "plate", 10),
        "top_zips": sorted_top_counts(zip_counts, "zip", 10),
        "camera_stats": camera_stats,
        "busiest_minutes": busiest_minutes,
    }


def maybe(p: float) -> bool:
    return random.random() < p


def random_plate() -> str:
    chars = string.ascii_uppercase + string.digits
    return "".join(random.choice(chars) for _ in range(6))


def random_vin8() -> str:
    alphabet = "0123456789ABCDEF"
    return "".join(random.choice(alphabet) for _ in range(8))


def random_zip() -> str:
    return f"{random.randint(0, 99999):05d}"


def random_camera_id() -> str:
    return f"CAM-{random.randint(1, 20)}"


def gen_valid_record(i: int, j: int) -> Dict[str, Any]:
    base_dt = datetime(2026, 4, 1, tzinfo=timezone.utc)
    dt = base_dt + timedelta(
        minutes=random.randint(0, 60 * 24 * 7),
        seconds=random.randint(0, 59),
    )
    record: Dict[str, Any] = {
        "ts": format_ts(dt),
        "camera_id": random_camera_id(),
        "plate": random.choice(
            [
                f"PLT{i % 50:03d}",
                f"TAG{j % 30:03d}",
                random_plate(),
            ]
        ),
        "vin8": random_vin8(),
        "zip": random.choice(["33324", "10001", "94105", "77002", random_zip()]),
        "confidence": round(random.uniform(0.35, 0.99), 2),
    }
    if maybe(0.15):
        record["lane"] = random.randint(1, 8)
    if maybe(0.10):
        record["vehicle_type"] = random.choice(["sedan", "suv", "truck", None])
    return record


def gen_invalid_record(mode: str) -> Any:
    record = gen_valid_record(0, 0)
    if mode == "bad_json":
        return '{"ts":"oops"'
    if mode == "missing_field":
        record.pop(
            random.choice(["ts", "camera_id", "plate", "vin8", "zip", "confidence"])
        )
        return record
    if mode == "bad_ts":
        record["ts"] = random.choice(["2026/04/03 17:02:11", "", None])
        return record
    if mode == "bad_confidence_type":
        record["confidence"] = random.choice(["0.91", None, {"n": 1}])
        return record
    if mode == "bad_confidence_range":
        record["confidence"] = random.choice([-0.1, 1.1, 99])
        return record
    if mode == "bad_zip":
        record["zip"] = random.choice(["3332", "ABCDE", 33324, None])
        return record
    if mode == "bad_vin8":
        record["vin8"] = random.choice(["1234567", "", None, "TOO-LONG-123"])
        return record
    if mode == "bad_camera":
        record["camera_id"] = random.choice(["", "   ", None, 9])
        return record
    return record


def validate_dataset(lines: List[Any]) -> Tuple[bool, str, List[Dict[str, Any]]]:
    records: List[Dict[str, Any]] = []
    for line in lines:
        if isinstance(line, str):
            return False, "json_decode_error", []
        ok, reason = validate_record(line)
        if not ok:
            return False, reason, []
        records.append(line)

    if not records:
        return False, "empty_input", []

    return True, "ok", records


def gen_dataset(count: int, mix: str) -> List[Any]:
    if mix == "mostly_valid":
        probs = {"valid": 0.90, "bad_json": 0.03, "invalid_record": 0.07}
    elif mix == "nasty":
        probs = {"valid": 0.45, "bad_json": 0.20, "invalid_record": 0.35}
    else:
        probs = {"valid": 0.70, "bad_json": 0.10, "invalid_record": 0.20}

    lines: List[Any] = [gen_valid_record(count, j) for j in range(count)]

    r = random.random()
    if r < probs["valid"]:
        return lines

    if r < probs["valid"] + probs["bad_json"]:
        lines[random.randrange(len(lines))] = gen_invalid_record("bad_json")
        return lines

    invalid_mode = random.choice(
        [
            "missing_field",
            "bad_ts",
            "bad_confidence_type",
            "bad_confidence_range",
            "bad_zip",
            "bad_vin8",
            "bad_camera",
        ]
    )
    lines[random.randrange(len(lines))] = gen_invalid_record(invalid_mode)
    return lines


def write_ndjson(path: Path, lines: List[Any]) -> None:
    with path.open("w", encoding="utf-8") as f:
        for line in lines:
            if isinstance(line, str):
                f.write(line + "\n")
            else:
                f.write(
                    json.dumps(line, separators=(",", ":"), ensure_ascii=False) + "\n"
                )


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default="problem_b_generate.jsonl")
    ap.add_argument("--count", type=int, default=50_000)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument(
        "--mix", choices=["balanced", "nasty", "mostly_valid"], default="balanced"
    )
    args = ap.parse_args()

    random.seed(args.seed)

    out_path = Path(args.out)
    expected_path = out_path.with_suffix(out_path.suffix + ".expected.json")

    lines = gen_dataset(args.count, args.mix)
    ok, reason, records = validate_dataset(lines)

    write_ndjson(out_path, lines)

    expected_payload: Dict[str, Any] = {
        "status_code": 200 if ok else 400,
        "output": aggregate_records(records) if ok else None,
    }
    if not ok:
        expected_payload["error"] = reason

    with expected_path.open("w", encoding="utf-8") as f:
        json.dump(expected_payload, f, separators=(",", ":"), ensure_ascii=False)
        f.write("\n")

    print(f"Wrote {len(lines):,} log lines to {out_path}")
    print(f"Wrote expected result to {expected_path}")
    print(f"Expected status: {expected_payload['status_code']}")


if __name__ == "__main__":
    main()
