#!/usr/bin/env python3
"""Audit the quality of every enum definition in the repo.

Checks both data/*.json (structured metadata) and definitions/**/*.md
(rich prose) against a set of AI-slop patterns and stub signals.
Outputs a per-file quality rating to stdout and a summary report.

Usage:
    python3 tools/audit-definitions.py                 # human-readable
    python3 tools/audit-definitions.py --json          # machine-readable
    python3 tools/audit-definitions.py --worst-first   # rank by quality

A "good" definition has:
- A non-empty, specific description (not "X attribute." or marketing-speak)
- Either a ≥200-byte markdown file OR a ≥100-char JSON overview
- Concrete mechanical data where applicable (stats, level effects, numbers)

A definition gets flagged if ANY of:
- It's a stub (our auto-generator marker, or body <200 bytes)
- Description matches a known lazy pattern
- Marketing-speak without specifics
- References to nonexistent enums
- Overly generic prose ("Controls X", "Related to X")
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent

# Patterns we've seen get produced by AI assistants that add no information.
SLOP_PATTERNS = [
    (re.compile(r"^[A-Z][A-Za-z0-9 _]*\s+attribute\.$"), "lazy-suffix: 'X attribute.'"),
    (re.compile(r"^[A-Z][A-Za-z0-9 _]*\s+weapon\.$"), "lazy-suffix: 'X weapon.'"),
    (re.compile(r"^[A-Z][A-Za-z0-9 _]*\s+class\.$"), "lazy-suffix: 'X class.'"),
    (re.compile(r"^[A-Z][A-Za-z0-9 _]*\s+flag\.$"), "lazy-suffix: 'X flag.'"),
    (
        re.compile(
            r"^(Controls|Enables|Related to|Regulates|Manages|Handles|Represents|Provides|Grants)\s+[a-z]",
        ),
        "lazy-verb: 'Controls/Enables/...'",
    ),
    (
        re.compile(
            r"\b(Powerful|Versatile|Essential|Must[- ]have|Game[- ]changing|Indispensable)\b",
            re.IGNORECASE,
        ),
        "marketing-speak",
    ),
    (re.compile(r"^Level\s+[0-9]+\.?$"), "'Level N.' as whole content"),
    (re.compile(r"^TODO|TBD|FIXME|\bXXX\b", re.IGNORECASE), "TODO/TBD marker"),
]

STUB_MARKDOWN_MARKER = "*Stub — a human needs to document this.*"
STUB_MIN_BODY_BYTES = 200


def rate(entry_id: str, description: str, overview: str, md_path: Path | None) -> dict:
    """Return a quality record for a single definition."""
    flags: list[str] = []

    # Prose quality checks on the description.
    desc = (description or "").strip()
    ov = (overview or "").strip()
    for pat, label in SLOP_PATTERNS:
        if desc and pat.search(desc):
            flags.append(f"desc:{label}")
        if ov and pat.search(ov):
            flags.append(f"overview:{label}")

    if not desc:
        flags.append("desc:empty")
    if not ov:
        flags.append("overview:empty")
    elif len(ov) < 80:
        flags.append("overview:short(<80c)")

    # Markdown body checks.
    md_state = "missing"
    md_bytes = 0
    if md_path and md_path.exists():
        body = md_path.read_text(encoding="utf-8", errors="ignore")
        md_bytes = len(body)
        if STUB_MARKDOWN_MARKER in body:
            md_state = "stub-marker"
            flags.append("md:auto-stub")
        elif md_bytes < STUB_MIN_BODY_BYTES:
            md_state = "stub-short"
            flags.append(f"md:short({md_bytes}b)")
        elif "|" in body and re.search(r"\|[-]+\|", body):
            md_state = "has-table"  # probably has stats
        elif any(m in body.lower() for m in ("### mechanics", "### stats", "### damage", "**multiplier**")):
            md_state = "has-mechanics"
        else:
            md_state = "prose-only"

    # Aggregate into a single rating.
    if md_state == "has-table" or md_state == "has-mechanics":
        rating = "good"
    elif md_state == "missing" and not ov and not desc:
        rating = "empty"
    elif "md:auto-stub" in flags or "md:short" in " ".join(flags):
        rating = "stub"
    elif any("slop" in f or "lazy" in f or "marketing" in f for f in flags):
        rating = "slop"
    elif not ov and not desc:
        rating = "empty"
    elif len(ov) >= 200 or len(desc) >= 80:
        rating = "prose"
    else:
        rating = "thin"

    return {
        "id": entry_id,
        "rating": rating,
        "flags": flags,
        "md_state": md_state,
        "md_bytes": md_bytes,
    }


RATING_ORDER = {"good": 0, "prose": 1, "thin": 2, "slop": 3, "stub": 4, "empty": 5}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true", help="machine-readable output")
    parser.add_argument(
        "--worst-first",
        action="store_true",
        help="list entries ranked worst-first",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=0,
        help="limit per-entry listings (0 = no limit)",
    )
    args = parser.parse_args(argv)

    # Family -> (json file, markdown subdir).
    families = {
        "attributes": ("data/attributes.json", "definitions/attributes"),
        "weapons": ("data/weapons.json", "definitions/weapons"),
        "classes": ("data/classes.json", "definitions/mb_classes"),
        "class_flags": ("data/class_flags.json", "definitions/class_flags"),
        "saber_styles": ("data/saber_styles.json", "definitions/saber_styles"),
        "glossary": ("data/glossary.json", "definitions/glossary"),
    }

    all_results: dict[str, list[dict]] = {}

    for name, (json_rel, md_rel) in families.items():
        json_path = REPO_ROOT / json_rel
        md_dir = REPO_ROOT / md_rel
        if not json_path.exists():
            continue
        data = json.loads(json_path.read_text())
        if not isinstance(data, list):
            continue

        results: list[dict] = []
        for e in data:
            entry_id = e.get("id") or e.get("name") or "(unnamed)"
            md_path = md_dir / f"{entry_id}.md" if md_dir else None
            results.append(
                rate(
                    entry_id=entry_id,
                    description=e.get("description", ""),
                    overview=e.get("overview", ""),
                    md_path=md_path,
                )
            )
        all_results[name] = results

    if args.json:
        print(json.dumps(all_results, indent=2))
        return 0

    print("═══════════════════════════════════════════════════════════")
    print(" MBII Foundry — Definition Quality Audit")
    print("═══════════════════════════════════════════════════════════")

    total_good = total = 0
    for family, results in all_results.items():
        counts = {"good": 0, "prose": 0, "thin": 0, "slop": 0, "stub": 0, "empty": 0}
        for r in results:
            counts[r["rating"]] += 1
        total += len(results)
        total_good += counts["good"]
        print(f"\n── {family} ({len(results)} entries) ──")
        for rating in ("good", "prose", "thin", "slop", "stub", "empty"):
            pct = 100 * counts[rating] / len(results) if results else 0
            bar = "█" * int(pct / 3)
            print(f"  {rating:6s} {counts[rating]:4d} ({pct:5.1f}%) {bar}")

        if args.worst_first:
            sorted_r = sorted(
                results, key=lambda r: (RATING_ORDER[r["rating"]], r["id"]), reverse=True
            )
            limit = args.limit or len(sorted_r)
            print(f"\n  Worst-first (first {limit}):")
            for r in sorted_r[:limit]:
                print(f"    {r['rating']:6s}  {r['id']:40s}  [{', '.join(r['flags'][:3])}]")

    if total:
        print(f"\n Overall: {total_good}/{total} ({100 * total_good / total:.1f}%) rated 'good'")
    return 0


if __name__ == "__main__":
    sys.exit(main())
