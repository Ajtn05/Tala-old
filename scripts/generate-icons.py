#!/usr/bin/env python3
"""Build the Dock and menu bar icons from the dashboard's waveform SVG.

Requires ImageMagick and macOS sips. No runtime dependencies are added.
"""
from pathlib import Path
import shutil
import subprocess
import struct
import tempfile

ROOT = Path(__file__).resolve().parents[1]
mark = (ROOT / "internal/dashboard/assets/logo.svg").read_text()
renderer = shutil.which("magick") or shutil.which("convert")
if not renderer:
    raise SystemExit("ImageMagick is required to regenerate icons.")

# Keep the exact waveform geometry used in the dashboard.
waveform = mark[mark.index("<path"):mark.index("</svg>")].strip()
app_svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">
  <rect x="64" y="64" width="896" height="896" rx="200" fill="#245bce"/>
  <g transform="translate(256 256) scale(16)" fill="none" stroke="#ffffff" stroke-width="3" stroke-linecap="round">
    {waveform}
  </g>
</svg>
'''
(ROOT / "icons/AppIcon.svg").write_text(app_svg)

with tempfile.TemporaryDirectory(prefix="tala-icons-") as temp:
    temp = Path(temp)
    source = ROOT / "icons/AppIcon.svg"
    subprocess.run([renderer, "-background", "none", str(source), "-resize", "1024x1024", "-depth", "8", str(ROOT / "icons/AppIcon.png")], check=True)
    iconset = ROOT / "icons/AppIcon.iconset"
    for size in (16, 32, 64, 128, 256, 512, 1024):
        subprocess.run(["sips", "-z", str(size), str(size), str(ROOT / "icons/AppIcon.png"), "--out", str(iconset / f"icon_{size}x{size}.png")], check=True, stdout=subprocess.DEVNULL)
        if size in (32, 256, 512, 1024):
            shutil.copyfile(iconset / f"icon_{size}x{size}.png", iconset / f"icon_{size//2}x{size//2}@2x.png")
        if size == 64:
            shutil.copyfile(iconset / "icon_64x64.png", iconset / "icon_32x32@2x.png")
    # ICNS supports PNG representations directly on all supported macOS versions.
    # Writing the container avoids iconutil's dependency on a GUI image service.
    chunks = []
    for kind, size in ((b"icp4", 16), (b"icp5", 32), (b"icp6", 64), (b"ic07", 128), (b"ic08", 256), (b"ic09", 512), (b"ic10", 1024), (b"ic11", 32), (b"ic12", 64), (b"ic13", 256), (b"ic14", 512)):
        png = (iconset / f"icon_{size}x{size}.png").read_bytes()
        chunks.append(kind + struct.pack(">I", 8 + len(png)) + png)
    payload = b"".join(chunks)
    (ROOT / "icons/AppIcon.icns").write_bytes(b"icns" + struct.pack(">I", 8 + len(payload)) + payload)
    for name, color in (("icon.png", "#000000"), ("icon_recording.png", "#d73445")):
        svg = temp / "menu.svg"
        svg.write_text(mark.replace('stroke="currentColor"', f'stroke="{color}"'))
        subprocess.run([renderer, "-background", "none", str(svg), "-resize", "44x44", str(ROOT / "internal/ui" / name)], check=True)

print("Generated Dock and menu bar icons from assets/logo.svg.")
