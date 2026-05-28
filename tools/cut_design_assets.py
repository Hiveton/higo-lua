#!/usr/bin/env python3
from pathlib import Path
from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "qml/assets/design"
OUT = ROOT / "qml/assets/cuts"

PAGES = ["production", "program", "feeders", "heads", "vision", "conveyor", "motion", "maintenance", "logs"]

# Source design images are 1672 x 941. These regions preserve original pixels.
REGIONS = {
    "topbar": (0, 0, 1672, 72),
    "nav": (0, 72, 148, 900),
    "workspace": (148, 72, 1215, 900),
    "right_panel": (1215, 72, 1672, 900),
    "bottom_status": (0, 900, 1672, 941),
}

def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for page in PAGES:
        src = SOURCE / f"{page}.png"
        image = Image.open(src).convert("RGBA")
        for name, box in REGIONS.items():
            image.crop(box).save(OUT / f"{page}-{name}.png")
    print(f"cut {len(PAGES) * len(REGIONS)} assets into {OUT}")

if __name__ == "__main__":
    main()
