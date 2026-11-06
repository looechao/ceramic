#!/usr/bin/env python3
"""Refresh the bundled Mastodon server domains using only Python's standard library."""

import json
import os
from pathlib import Path
import re
import tempfile
from urllib.request import urlopen

SOURCE = "https://api.joinmastodon.org/servers"
DESTINATION = Path(__file__).resolve().parents[1] / "resources" / "instances.json"


def main():
    with urlopen(SOURCE, timeout=30) as response:
        servers = json.load(response)
    if not isinstance(servers, list) or not servers:
        raise ValueError("Expected a non-empty server directory; existing file unchanged")

    domains = set()
    for server in servers:
        domain = server["domain"].strip().rstrip(".").encode("idna").decode().lower()
        labels = domain.split(".")
        if len(domain) > 253 or len(labels) < 2 or any(
            not re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?", label)
            for label in labels
        ):
            raise ValueError(f"Invalid domain: {domain!r}; existing file unchanged")
        domains.add(domain)

    # Write only after the complete response has been validated.
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=DESTINATION.parent, delete=False) as output:
            temporary = Path(output.name)
            json.dump(sorted(domains), output, ensure_ascii=False, indent=2)
            output.write("\n")
        temporary.chmod(0o644)
        os.replace(temporary, DESTINATION)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    print(f"Updated resources/instances.json: {len(domains)} domains")


if __name__ == "__main__":
    main()
