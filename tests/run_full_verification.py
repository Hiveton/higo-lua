#!/usr/bin/env python3
import os
import json
import subprocess
from datetime import datetime, timezone
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
PAGES = ["production", "program", "feeders", "heads", "vision", "conveyor", "motion", "maintenance", "logs"]
CAPTURE_SETS = [
    ("captures-800", 800, 600),
    ("captures-1920", 1920, 1080),
    ("captures-4k", 3840, 2160),
]
DIALOG_CAPTURES = [
    ("feeders", 2, "add"),
    ("vision", 4, "add"),
    ("logs", 8, "add"),
]


def run(cmd, cwd=ROOT, env=None):
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    print("+", " ".join(map(str, cmd)))
    subprocess.run(cmd, cwd=cwd, env=merged_env, check=True)


def verify_capture_dir(directory: Path, width: int, height: int, suffix: str = ""):
    captures = []
    for page in PAGES:
        path = directory / f"{page}{suffix}-{width}x{height}.png"
        if not path.exists():
            raise AssertionError(f"missing capture: {path}")
        with Image.open(path) as image:
            if image.size != (width, height):
                raise AssertionError(f"wrong size for {path}: {image.size}")
        captures.append({"page": page, "path": str(path.relative_to(ROOT)), "width": width, "height": height})
    return captures


def verify_dialog_capture(directory: Path, page: str, action: str, width: int, height: int):
    path = directory / f"{page}-{action}-{width}x{height}.png"
    if not path.exists():
        raise AssertionError(f"missing dialog capture: {path}")
    with Image.open(path) as image:
        if image.size != (width, height):
            raise AssertionError(f"wrong size for {path}: {image.size}")
    return {"page": page, "action": action, "path": str(path.relative_to(ROOT)), "width": width, "height": height}


def main():
    report = {
        "project": "SMT Pick And Place HMI",
        "status": "running",
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "pages": PAGES,
        "commands": [],
        "self_test": "pending",
        "capture_sets": [],
        "dialog_captures": [],
    }

    run(["python3", "tests/validate_project.py"])
    report["commands"].append("python3 tests/validate_project.py")
    run(["cmake", "-S", ".", "-B", "build"])
    report["commands"].append("cmake -S . -B build")
    run(["cmake", "--build", "build", "-j4"])
    report["commands"].append("cmake --build build -j4")

    binary = ROOT / "build/smt_pnp_hmi"
    run([str(binary), "--self-test"], env={"QT_QPA_PLATFORM": "offscreen"})
    report["commands"].append("QT_QPA_PLATFORM=offscreen build/smt_pnp_hmi --self-test")
    report["self_test"] = "passed"

    for name, width, height in CAPTURE_SETS:
        out = ROOT / "artifacts" / name
        out.mkdir(parents=True, exist_ok=True)
        run([
            str(binary),
            "--capture-dir", str(out),
            "--capture-width", str(width),
            "--capture-height", str(height),
        ], env={"QT_QPA_PLATFORM": "offscreen"})
        captures = verify_capture_dir(out, width, height)
        report["capture_sets"].append({
            "name": name,
            "width": width,
            "height": height,
            "count": len(captures),
            "captures": captures,
        })

    dialog_dir = ROOT / "artifacts/captures-dialog"
    dialog_dir.mkdir(parents=True, exist_ok=True)
    for page, page_index, action in DIALOG_CAPTURES:
        run([
            str(binary),
            "--capture-dir", str(dialog_dir),
            "--capture-width", "1920",
            "--capture-height", "1080",
            "--capture-action-page", str(page_index),
            "--capture-action", action,
        ], env={"QT_QPA_PLATFORM": "offscreen"})
        report["dialog_captures"].append(verify_dialog_capture(dialog_dir, page, action, 1920, 1080))

    report["status"] = "passed"
    report_path = ROOT / "artifacts/verification-report.json"
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"verification report written: {report_path}")
    print("full SMT PNP HMI verification passed")


if __name__ == "__main__":
    main()
