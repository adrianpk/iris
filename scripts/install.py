#!/usr/bin/env python3
"""Install or remove Iris user files without changing service state or data."""

import argparse
import os
from pathlib import Path
import tempfile


def absolute_path(value):
    path = Path(value)
    if not path.is_absolute() or any(ord(char) < 32 for char in value):
        raise argparse.ArgumentTypeError("paths must be absolute and contain no control characters")
    return path


def unit_quote(value):
    value = str(value).replace("\\", "\\\\").replace('"', '\\"').replace("%", "%%")
    return '"' + value + '"'


def check_target(path):
    if path.is_symlink() or (path.exists() and not path.is_file()):
        raise ValueError(f"refuse to replace non-regular file: {path}")
    if path.exists() and path.stat().st_uid != os.geteuid():
        raise ValueError(f"file is not owned by this user: {path}")


def atomic_file(destination, content, mode):
    destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    check_target(destination)
    descriptor, temporary = tempfile.mkstemp(prefix=".iris-", dir=destination.parent)
    try:
        with os.fdopen(descriptor, "wb") as output:
            os.fchmod(output.fileno(), mode)
            output.write(content)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, destination)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def render_units(source, binary, data, configuration, runtime):
    service = (source / "iris.service").read_text()
    service = service.replace('"%h/.local/bin/iris"', unit_quote(binary))
    environment = " ".join(unit_quote(f"{name}={value}") for name, value in (
        ("XDG_DATA_HOME", data), ("XDG_CONFIG_HOME", configuration), ("XDG_RUNTIME_DIR", runtime)))
    service = service.replace("[Service]\n", "[Service]\nEnvironment=" + environment + "\n")
    socket = (source / "iris.socket").read_text()
    socket = socket.replace("%t/iris/iris.sock", str(runtime / "iris" / "iris.sock").replace("%", "%%"))
    return {"iris.service": service.encode(), "iris.socket": socket.encode()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("install", "uninstall"))
    parser.add_argument("--prefix", type=absolute_path, default=Path.home() / ".local")
    parser.add_argument("--unit-dir", type=absolute_path)
    parser.add_argument("--binary", type=absolute_path)
    args = parser.parse_args()
    configuration = absolute_path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config")))
    units = args.unit_dir or configuration / "systemd/user"
    target = args.prefix / "bin/iris"
    destinations = [target, units / "iris.service", units / "iris.socket"]
    for destination in destinations:
        check_target(destination)
    if args.action == "uninstall":
        for destination in destinations:
            destination.unlink(missing_ok=True)
        print("Removed Iris binary and units; data and configuration preserved. Reload the user manager.")
        return
    data = absolute_path(os.environ.get("XDG_DATA_HOME", str(Path.home() / ".local/share")))
    runtime = absolute_path(os.environ.get("XDG_RUNTIME_DIR", ""))
    source = Path(__file__).resolve().parent.parent
    binary = args.binary or source / "_workspace/bin/iris"
    if not binary.is_file() or not os.access(binary, os.X_OK):
        raise ValueError("build Iris first with make build or provide --binary")
    content = render_units(source / "assets/systemd/user", target, data, configuration, runtime)
    atomic_file(target, binary.read_bytes(), 0o755)
    for name, body in content.items():
        atomic_file(units / name, body, 0o600)
    print("Installed Iris binary and units; activation is explicit. Run systemctl --user daemon-reload.")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError) as error:
        raise SystemExit(f"iris installation: {error}") from error
