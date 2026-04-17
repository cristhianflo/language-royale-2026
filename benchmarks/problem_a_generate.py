#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import random
import string
from typing import Any, Dict, Optional, Tuple

# ---- Shared validation rules (THIS defines expected 200 vs 400) ----
LIMITS = {
    "lpr_hits_24h": (0, 10_000),
    "distance_miles": (0.0, 10_000.0),
    "days_since_last_seen": (0, 36_500),
}


def is_nonempty_str(x: Any) -> bool:
    return isinstance(x, str) and x.strip() != ""


def is_int(x: Any) -> bool:
    # bool is a subclass of int in Python, exclude it
    return isinstance(x, int) and not isinstance(x, bool)


def is_number(x: Any) -> bool:
    return isinstance(x, (int, float)) and not isinstance(x, bool)


def validate_body(body: Any) -> Tuple[bool, str]:
    if not isinstance(body, dict):
        return False, "body_not_object"

    case_id = body.get("case_id", None)
    if not is_nonempty_str(case_id):
        return False, "case_id_invalid"

    signals = body.get("signals", None)
    if not isinstance(signals, dict):
        return False, "signals_missing_or_not_object"

    # lpr_hits_24h
    lpr = signals.get("lpr_hits_24h", None)
    if not is_int(lpr):
        return False, "lpr_hits_24h_type"
    lo, hi = LIMITS["lpr_hits_24h"]
    if not (lo <= lpr <= hi):
        return False, "lpr_hits_24h_range"

    # address_match
    addr = signals.get("address_match", None)
    if not isinstance(addr, bool):
        return False, "address_match_type"

    # distance_miles
    dist = signals.get("distance_miles", None)
    if not is_number(dist):
        return False, "distance_miles_type"
    lo_f, hi_f = LIMITS["distance_miles"]
    if not (lo_f <= float(dist) <= hi_f):
        return False, "distance_miles_range"

    # days_since_last_seen
    days = signals.get("days_since_last_seen", None)
    if not is_int(days):
        return False, "days_since_last_seen_type"
    lo_d, hi_d = LIMITS["days_since_last_seen"]
    if not (lo_d <= days <= hi_d):
        return False, "days_since_last_seen_range"

    return True, "ok"


def sigmoid(x: float) -> float:
    if x >= 0:
        return 1.0 / (1.0 + math.exp(-x))
    exp_x = math.exp(x)
    return exp_x / (1.0 + exp_x)


def score_case(body: Dict[str, Any]) -> Dict[str, Any]:
    signals = body["signals"]
    raw_score = sigmoid(
        0.9 * signals["lpr_hits_24h"]
        + 1.2 * int(signals["address_match"])
        - 0.35 * float(signals["distance_miles"])
        - 0.6 * signals["days_since_last_seen"]
    )

    score = round(raw_score, 2)
    if score >= 0.80:
        tier = "HOT"
    elif score >= 0.50:
        tier = "WARM"
    else:
        tier = "COLD"

    return {
        "case_id": body["case_id"],
        "score": score,
        "tier": tier,
    }


# ---- Payload generation ----


def maybe(p: float) -> bool:
    return random.random() < p


def rand_case_id(i: int) -> str:
    return f"C{i:06d}"


def random_plate_like() -> str:
    chars = string.ascii_uppercase + string.digits
    return "".join(random.choice(chars) for _ in range(random.randint(5, 8)))


def gen_valid_signals() -> Dict[str, Any]:
    return {
        "lpr_hits_24h": random.choice([0, 1, 2, 3, 5, 10, 25, 50, 100, 250, 9999]),
        "address_match": random.choice([True, False]),
        "distance_miles": float(
            random.choice([0.0, 0.1, 0.5, 1.2, 2.4, 5.0, 12.5, 25.0, 9999.9])
        ),
        "days_since_last_seen": random.choice([0, 1, 2, 7, 14, 30, 365, 1000]),
    }


def gen_out_of_range_signals() -> Dict[str, Any]:
    return {
        "lpr_hits_24h": random.choice([-1, -5, 10001, 1_000_000]),
        "address_match": random.choice([True, False]),
        "distance_miles": random.choice([-0.01, -10.0, 10000.1, 9999999.0]),
        "days_since_last_seen": random.choice([-1, -9, 36501, 999999]),
    }


def gen_invalid_type_signals() -> Dict[str, Any]:
    return {
        "lpr_hits_24h": random.choice(["3", None, True, [1], {"n": 3}]),
        "address_match": random.choice(["true", 1, 0, None, ["x"], {"b": True}]),
        "distance_miles": random.choice(["2.4", None, False, [2.4], {"m": 2.4}]),
        "days_since_last_seen": random.choice(["1", None, 1.5, [1], {"d": 1}]),
    }


def drop_random_required_keys(signals: Dict[str, Any]) -> Dict[str, Any]:
    required = [
        "lpr_hits_24h",
        "address_match",
        "distance_miles",
        "days_since_last_seen",
    ]
    random.shuffle(required)
    for k in required[: random.randint(1, 3)]:
        signals.pop(k, None)
    return signals


def gen_body(i: int, mix: str) -> Dict[str, Any]:
    """
    mix:
      - balanced: 70% valid
      - nasty: 45% valid
      - mostly_valid: 90% valid
    """
    if mix == "mostly_valid":
        probs = {"valid": 0.90, "missing": 0.04, "oor": 0.03, "badtype": 0.03}
    elif mix == "nasty":
        probs = {"valid": 0.45, "missing": 0.20, "oor": 0.20, "badtype": 0.15}
    else:
        probs = {"valid": 0.70, "missing": 0.10, "oor": 0.10, "badtype": 0.10}

    r = random.random()
    cutoff_valid = probs["valid"]
    cutoff_missing = cutoff_valid + probs["missing"]
    cutoff_oor = cutoff_missing + probs["oor"]

    body: Dict[str, Any] = {
        "case_id": rand_case_id(i),
        "signals": gen_valid_signals(),
    }

    # optional noise fields (should not affect validity)
    if maybe(0.25):
        body["meta"] = {"plate_hint": random_plate_like()}
    if maybe(0.10):
        body["signals"]["extra_noise"] = random.choice(
            [None, "x", 123, {"nested": True}]
        )

    if r < cutoff_valid:
        return body

    if r < cutoff_missing:
        mode = random.choice(
            [
                "missing_signals",
                "signals_null",
                "signals_missing_required",
                "missing_case_id",
                "case_id_null",
                "case_id_empty",
            ]
        )
        if mode == "missing_signals":
            body.pop("signals", None)
        elif mode == "signals_null":
            body["signals"] = None
        elif mode == "signals_missing_required":
            body["signals"] = drop_random_required_keys(gen_valid_signals())
        elif mode == "missing_case_id":
            body.pop("case_id", None)
        elif mode == "case_id_null":
            body["case_id"] = None
        elif mode == "case_id_empty":
            body["case_id"] = "   "
        return body

    if r < cutoff_oor:
        body["signals"] = gen_out_of_range_signals()
        return body

    body["signals"] = gen_invalid_type_signals()
    return body


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default="problem_a_generate.jsonl")
    ap.add_argument("--count", type=int, default=50_000)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument(
        "--mix", choices=["balanced", "nasty", "mostly_valid"], default="balanced"
    )
    args = ap.parse_args()

    random.seed(args.seed)

    good = 0
    bad = 0

    with open(args.out, "w", encoding="utf-8") as f:
        for i in range(args.count):
            body = gen_body(i, args.mix)
            ok, _reason = validate_body(body)
            expected_status = 200 if ok else 400
            if ok:
                good += 1
                expected_output: Optional[Dict[str, Any]] = score_case(body)
            else:
                bad += 1
                expected_output = None

            rec = {
                "input": body,
                "output": expected_output,
                "status_code": expected_status,
            }
            f.write(json.dumps(rec, separators=(",", ":"), ensure_ascii=False) + "\n")

    print(f"Wrote {args.count:,} testcases to {args.out}")
    print(f"Expected 200: {good:,} ({good / args.count:.1%})")
    print(f"Expected 400: {bad:,} ({bad / args.count:.1%})")


if __name__ == "__main__":
    main()
