#!/usr/bin/env python3
"""Render the small allowlisted Envoy deployment template safely."""

from __future__ import annotations

import argparse
import re
from pathlib import Path

HOST_PATTERN = re.compile(
    r"^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$",
    re.IGNORECASE,
)
TOKEN = "__JUDGIFY_PUBLIC_HOST__"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--template", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--host", required=True)
    return parser.parse_args()


def valid_host(value: str) -> bool:
    return bool(HOST_PATTERN.fullmatch(value))


def main() -> int:
    args = parse_args()
    host = args.host.strip().lower()
    if host != args.host or not valid_host(host):
        raise SystemExit("Envoy public hostname must be a canonical DNS name")
    source = args.template.read_text(encoding="utf-8")
    if source.count(TOKEN) != 1:
        raise SystemExit("Envoy template must contain exactly one hostname token")
    rendered = source.replace(TOKEN, host)
    if TOKEN in rendered:
        raise SystemExit("Envoy template retained an unresolved hostname token")
    args.output.parent.mkdir(mode=0o755, parents=True, exist_ok=True)
    args.output.write_text(rendered, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
