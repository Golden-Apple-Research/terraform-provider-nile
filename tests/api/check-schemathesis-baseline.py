#!/usr/bin/env python3
"""Compare a Schemathesis JUnit report against the known-deviations baseline.

usage: check-schemathesis-baseline.py REPORT.xml BASELINE [--update]

Schemathesis fails whenever the mock's responses deviate from the pinned Nile
spec. Some deviations are known and accepted (documented in the baseline file
next to this script); this checker keeps CI green for those and fails only on
NEW deviations.

A deviation is fingerprinted as "METHOD /path :: check heading" using every
"- <heading>" line inside a JUnit failure text. <error> elements (connection
problems and other harness failures) can never be baselined: they always
fail the run.

Exit codes: 0 = no failures or only baselined deviations, 1 = new deviations
or errors. With --update, refresh the baseline from the report instead
(errors still fail and are never written).
"""
import sys
import xml.etree.ElementTree as ET


def parse(report_path):
    """Return (deviation fingerprints, error fingerprints)."""
    root = ET.parse(report_path).getroot()
    deviations, errors = set(), set()
    for case in root.iter("testcase"):
        name = case.attrib.get("name", "<unknown operation>")
        for child in case:
            if child.tag == "failure":
                # Schemathesis puts the numbered failure sections (including
                # the "- <check heading>" lines) into the message attribute.
                detail = child.attrib.get("message") or child.text or ""
                headings = [
                    line.strip()[2:].strip()
                    for line in detail.splitlines()
                    if line.strip().startswith("- ")
                ]
                for heading in headings or ["<unspecified failure>"]:
                    deviations.add(f"{name} :: {heading}")
            elif child.tag == "error":
                errors.add(f"{name} :: <error>")
    return deviations, errors


def main():
    args = [a for a in sys.argv[1:] if a != "--update"]
    update = "--update" in sys.argv[1:]
    if len(args) < 2:
        print(__doc__, file=sys.stderr)
        return 2
    report_path, baseline_path = args[0], args[1]

    deviations, errors = parse(report_path)

    if errors:
        print("schemathesis harness errors (mock not running? spec unreadable?):", file=sys.stderr)
        for fingerprint in sorted(errors):
            print(f"  {fingerprint}", file=sys.stderr)
        return 1

    if update:
        with open(baseline_path, "w", encoding="utf-8") as fh:
            fh.write("# Known Schemathesis deviations, one fingerprint per line.\n")
            fh.write("# Regenerate deliberately: tests/api/run-schemathesis.sh with\n")
            fh.write("# ST_REPORT_DIR set, then:\n")
            fh.write("#   check-schemathesis-baseline.py REPORT.xml BASELINE --update\n")
            for fingerprint in sorted(deviations):
                fh.write(f"{fingerprint}\n")
        print(f"baseline written: {baseline_path} ({len(deviations)} deviations)")
        return 0

    baseline = set()
    try:
        with open(baseline_path, encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if line and not line.startswith("#"):
                    baseline.add(line)
    except FileNotFoundError:
        pass  # no baseline yet: every deviation is new

    new = sorted(deviations - baseline)
    stale = sorted(baseline - deviations)

    if new:
        print(f"schemathesis: {len(new)} NEW deviation(s) not in the baseline:", file=sys.stderr)
        for fingerprint in new:
            print(f"  {fingerprint}", file=sys.stderr)
        return 1

    if stale:
        print("schemathesis: baselined deviations no longer present (candidates to remove):")
        for fingerprint in stale:
            print(f"  {fingerprint}")
    print(
        f"schemathesis: {len(deviations)} deviation(s), all matching the "
        f"known baseline ({len(baseline)} entries)"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
