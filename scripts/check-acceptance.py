#!/usr/bin/env python3
"""Verify the ten-group Iris acceptance map without executing its runtime gates."""

from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent.parent


def main():
    cases = {}
    for path in (ROOT / "internal/iris").glob("*_test.go"):
        source = path.read_text()
        tag = re.search(r"^//go:build (.+)$", source, re.M)
        for name in re.findall(r"^func (Test\w+)\(t \*testing.T\)", source, re.M):
            cases[name] = tag.group(1) if tag else "unit"
    rows = [line.split("|")[1:-1] for line in
            (ROOT / "docs/reference/acceptance.md").read_text().splitlines()
            if re.match(r"^\| \d+\. ", line)]
    assert [int(row[0].split(".")[0]) for row in rows] == list(range(1, 11)), "Ten consecutive groups required"
    makefile = (ROOT / "Makefile").read_text()
    prerequisites = re.search(r"^verify: (.+)$", makefile, re.M).group(1).split()
    assert {"lint-strict", "test", "contracts-check", "acceptance-check", "test-install"} <= set(prerequisites), "Incomplete verification dependencies"
    selectors = {}
    for tag in ("integration", "systemd", "bridge"):
        body = re.search(rf"^test-{tag}:\n((?:\t.*\n)+)", makefile, re.M).group(1)
        assert f"-tags={tag}" in body, f"Missing build tag in {tag} gate"
        selectors[tag] = re.search(r"-run '([^']+)'", body).group(1)
    mapped = set()
    for row in rows:
        names = re.findall(r"`(Test\w+)`", row[2])
        assert names, f"Empty acceptance group: {row[0]}"
        for name in names:
            assert name in cases, f"Missing acceptance case: {name}"
            tag = cases[name]
            gate = {"unit": "make verify", "integration": "make test-integration",
                    "systemd": "make test-systemd", "bridge": "make test-bridge"}[tag]
            assert f"`{gate}`" in row[3], f"Case outside its declared gate: {name}"
            if tag != "unit":
                assert re.search(selectors[tag], name), f"Case outside actual gate selector: {name}"
            mapped.add(name)
    print(f"Acceptance map: ten groups; {len(mapped)} existing cases in their declared gates; runtime results recorded separately")


if __name__ == "__main__":
    main()
