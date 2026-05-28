#!/usr/bin/env python3
import json
import shutil
import zipfile
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ARTIFACTS = ROOT / "artifacts"
RELEASE_ROOT = ARTIFACTS / "release"
PACKAGE_NAME = "smt-pnp-hmi-deliverable"
PACKAGE_DIR = RELEASE_ROOT / PACKAGE_NAME
ZIP_PATH = RELEASE_ROOT / f"{PACKAGE_NAME}.zip"

SOURCE_PATHS = [
    "CMakeLists.txt",
    "README.md",
    "src",
    "qml",
    "docs",
    "tests",
    "tools",
]

ARTIFACT_PATHS = [
    "verification-report.json",
    "captures-800",
    "captures-1920",
    "captures-4k",
    "captures-dialog",
]


def copy_path(source: Path, destination: Path) -> None:
    if not source.exists():
        raise FileNotFoundError(f"required path does not exist: {source}")
    if source.is_dir():
        shutil.copytree(source, destination, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
    else:
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, destination)


def collect_pngs(path: Path) -> list[str]:
    if not path.exists():
        return []
    return sorted(str(item.relative_to(PACKAGE_DIR)) for item in path.rglob("*.png"))


def count_pngs(path: Path) -> int:
    return len(collect_pngs(path))


def load_verification_report() -> dict:
    report_path = ARTIFACTS / "verification-report.json"
    if not report_path.exists():
        raise FileNotFoundError("missing artifacts/verification-report.json; run tests/run_full_verification.py first")
    return json.loads(report_path.read_text(encoding="utf-8"))


def write_manifest(verification_report: dict) -> dict:
    design_dir = PACKAGE_DIR / "source/qml/assets/design"
    cuts_dir = PACKAGE_DIR / "source/qml/assets/cuts"
    artifacts_dir = PACKAGE_DIR / "artifacts"
    binary_path = PACKAGE_DIR / "binary/smt_pnp_hmi"

    manifest = {
        "project": "SMT Pick And Place HMI",
        "package": PACKAGE_NAME,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "source_root": "source",
        "requirements": {
            "machine": "8 heads, front 60 feeders, rear 60 feeders, tray, three-section conveyor",
            "vision": ["flying camera", "Mark camera", "large-component camera"],
            "resolution_support": ["800x600", "1920x1080", "3840x2160"],
            "qt": "Qt 6.5+",
        },
        "design_fidelity": {
            "mode": "original design images with transparent QML interaction hotspots",
            "design_asset_count": count_pngs(design_dir),
            "cut_asset_count": count_pngs(cuts_dir),
        },
        "verification": {
            "report": "artifacts/verification-report.json",
            "status": verification_report.get("status"),
            "self_test": verification_report.get("self_test"),
            "capture_sets": verification_report.get("capture_sets", []),
            "dialog_captures": verification_report.get("dialog_captures", []),
        },
        "deliverables": {
            "main_screenshot_count": (
                count_pngs(artifacts_dir / "captures-800")
                + count_pngs(artifacts_dir / "captures-1920")
                + count_pngs(artifacts_dir / "captures-4k")
            ),
            "dialog_screenshot_count": count_pngs(artifacts_dir / "captures-dialog"),
            "design_assets": collect_pngs(design_dir),
            "cut_assets": collect_pngs(cuts_dir),
            "binary": "binary/smt_pnp_hmi" if binary_path.exists() else None,
            "documents": [
                "source/README.md",
                "source/docs/acceptance-matrix.md",
                "source/docs/device-interface-contract.md",
            ],
        },
        "known_boundaries": [
            "Current backend bridge is a replaceable Qt mock implementation.",
            "Real motion controller, camera SDK, PLC/IO, database, and permission system are not connected yet.",
        ],
    }

    manifest_path = PACKAGE_DIR / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    return manifest


def create_zip() -> None:
    if ZIP_PATH.exists():
        ZIP_PATH.unlink()
    with zipfile.ZipFile(ZIP_PATH, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for path in sorted(PACKAGE_DIR.rglob("*")):
            if path.is_file():
                archive.write(path, path.relative_to(RELEASE_ROOT))


def main() -> None:
    verification_report = load_verification_report()
    RELEASE_ROOT.mkdir(parents=True, exist_ok=True)
    if PACKAGE_DIR.exists():
        shutil.rmtree(PACKAGE_DIR)
    PACKAGE_DIR.mkdir(parents=True)

    source_root = PACKAGE_DIR / "source"
    for relative in SOURCE_PATHS:
        copy_path(ROOT / relative, source_root / relative)

    artifacts_root = PACKAGE_DIR / "artifacts"
    for relative in ARTIFACT_PATHS:
        copy_path(ARTIFACTS / relative, artifacts_root / relative)

    binary = ROOT / "build/smt_pnp_hmi"
    if binary.exists():
        binary_out = PACKAGE_DIR / "binary/smt_pnp_hmi"
        binary_out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(binary, binary_out)

    manifest = write_manifest(verification_report)
    create_zip()

    print(f"release directory: {PACKAGE_DIR}")
    print(f"release zip: {ZIP_PATH}")
    print(
        "manifest: "
        f"{manifest['deliverables']['main_screenshot_count']} main screenshots, "
        f"{manifest['deliverables']['dialog_screenshot_count']} dialog screenshots, "
        f"{manifest['design_fidelity']['design_asset_count']} design assets, "
        f"{manifest['design_fidelity']['cut_asset_count']} cut assets"
    )


if __name__ == "__main__":
    main()
