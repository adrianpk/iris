#!/usr/bin/env python3
"""Supply the bundled Iris turn policy to Codex UserPromptSubmit."""

import json
from pathlib import Path
import sys


def main():
    event = json.load(sys.stdin)
    if event.get("hook_event_name") != "UserPromptSubmit":
        print("{}")
        return
    policy = Path(__file__).resolve().parent.parent / "references/turn-policy.md"
    print(json.dumps({"hookSpecificOutput": {
        "hookEventName": "UserPromptSubmit",
        "additionalContext": policy.read_text(encoding="utf-8"),
    }}))


if __name__ == "__main__":
    main()
