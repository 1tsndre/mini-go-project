"""Prints the k6 summaries in bench/results/ as a Markdown table, one row per implementation and scenario.

Usage: python3 bench/report.py [results dir]
"""

import json
import pathlib
import re
import sys

RESULTS = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else pathlib.Path(__file__).parent / "results")
ORDER = ["go", "java", "python", "express", "nestjs"]
UNITS = {"ms": 0.001, "s": 1, "m": 60, "h": 3600}


def seconds(duration: str) -> float:
    """A k6 duration such as "60s" or "1m30s" in seconds."""
    return sum(float(n) * UNITS[u] for n, u in re.findall(r"(\d+(?:\.\d+)?)(ms|s|m|h)", duration))


rows = []
for path in sorted(RESULTS.glob("*.json")):
    if path.name.endswith(".run.json"):
        continue
    name, scenario = path.stem.rsplit("-", 1)
    run = json.loads(path.with_name(path.stem + ".run.json").read_text())
    metrics = json.loads(path.read_text())["metrics"]
    duration = metrics[f"http_req_duration{{scenario:{scenario}}}"]
    count = metrics[f"http_reqs{{scenario:{scenario}}}"]["count"]
    failed = metrics[f"http_req_failed{{scenario:{scenario}}}"]["value"]
    rows.append((name, scenario, run["vus"], count / seconds(run["duration"]), duration, failed))

rows.sort(key=lambda r: (r[1], ORDER.index(r[0]) if r[0] in ORDER else len(ORDER), r[0]))
print("| Scenario | Implementation | VUs | Requests/s | Median (ms) | p95 (ms) | p99 (ms) | Failed |")
print("|----------|----------------|-----|-----------:|------------:|---------:|---------:|-------:|")
for name, scenario, vus, rate, d, failed in rows:
    print(f"| {scenario} | {name} | {vus} | {rate:,.0f} | {d['med']:.1f} | {d['p(95)']:.1f} | {d['p(99)']:.1f} | {failed:.2%} |")
