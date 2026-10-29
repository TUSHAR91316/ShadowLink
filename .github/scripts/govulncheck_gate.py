#!/usr/bin/env python3
"""Fail CI only for reachable Go vulnerabilities that have a fix available.

Reads `govulncheck -format json` output on stdin. A finding is "reachable" when
its call trace ends in a concrete function (govulncheck's symbol-level result),
meaning our code actually calls the vulnerable code. Reachable findings with no
upstream fix (e.g. GO-2024-3218 in go-libp2p-kad-dht) are reported but do not
fail the build, since there is nothing to upgrade to.
"""

import json
import sys


def main() -> int:
    decoder = json.JSONDecoder()
    data = sys.stdin.read()
    summaries: dict[str, str] = {}
    reachable: dict[str, str] = {}  # osv id -> fixed version ("" when none)

    pos = 0
    while pos < len(data):
        while pos < len(data) and data[pos].isspace():
            pos += 1
        if pos >= len(data):
            break
        msg, pos = decoder.raw_decode(data, pos)

        if "osv" in msg:
            osv = msg["osv"]
            summaries[osv["id"]] = osv.get("summary", "")
        finding = msg.get("finding")
        if finding and finding.get("trace") and finding["trace"][0].get("function"):
            reachable[finding["osv"]] = finding.get("fixed_version", "")

    fixable = {k: v for k, v in reachable.items() if v}
    unfixable = sorted(k for k, v in reachable.items() if not v)

    for osv_id in unfixable:
        print(f"::warning::{osv_id} is reachable but has no fix yet: {summaries.get(osv_id, '')}")
    for osv_id, fixed in sorted(fixable.items()):
        print(f"::error::{osv_id} is reachable and fixed in {fixed}: {summaries.get(osv_id, '')}")

    print(f"Reachable vulnerabilities: {len(reachable)} ({len(fixable)} fixable, {len(unfixable)} without a fix)")
    return 1 if fixable else 0


if __name__ == "__main__":
    sys.exit(main())
