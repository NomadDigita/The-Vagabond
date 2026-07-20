#!/usr/bin/env python3
"""Render one deterministic TGS frame with local lottie-web and Chrome.

Requires lottie-web installed under work/toolcache/tgs-node and Google Chrome.
The output PNG is only a local review artifact; Telegram client review remains
the final compatibility check.
"""

from __future__ import annotations

import argparse
import gzip
import html
import json
import subprocess
import tempfile
import time
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
# The repository is work/The-Vagabond, so toolcache is its direct sibling.
WORK = ROOT.parent
DEFAULT_LOTTIE = WORK / "toolcache" / "tgs-node" / "node_modules" / "lottie-web" / "build" / "player" / "lottie.js"
DEFAULT_CHROME = Path(r"C:\Program Files\Google\Chrome\Application\chrome.exe")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("asset", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--frame", type=int, default=0, help="frame number to render")
    parser.add_argument("--lottie-js", type=Path, default=DEFAULT_LOTTIE)
    parser.add_argument("--chrome", type=Path, default=DEFAULT_CHROME)
    args = parser.parse_args()

    asset = args.asset.resolve()
    output = args.output.resolve()
    if not asset.is_file():
        raise SystemExit(f"Missing asset: {asset}")
    if not args.lottie_js.is_file():
        raise SystemExit(f"Missing lottie-web: {args.lottie_js}")
    if not args.chrome.is_file():
        raise SystemExit(f"Missing Chrome: {args.chrome}")
    with gzip.open(asset, "rt", encoding="utf-8") as handle:
        animation = json.load(handle)
    payload = json.dumps(animation, separators=(",", ":")).replace("</", "<\\/")
    script_uri = args.lottie_js.resolve().as_uri()
    output.parent.mkdir(parents=True, exist_ok=True)
    document = f"""<!doctype html><html><head><meta charset=\"utf-8\"><style>
html,body,#animation{{margin:0;width:512px;height:512px;background:transparent;overflow:hidden}}
</style></head><body><div id=\"animation\"></div><script src=\"{html.escape(script_uri)}\"></script><script>
const data={payload};
const animation=lottie.loadAnimation({{container:document.getElementById('animation'),renderer:'svg',loop:false,autoplay:false,animationData:data,rendererSettings:{{preserveAspectRatio:'xMidYMid meet'}}}});
animation.addEventListener('DOMLoaded',()=>{{animation.goToAndStop({args.frame},true);document.title='TGS_READY';}});
</script></body></html>"""
    with tempfile.TemporaryDirectory(prefix="tgs-preview-") as directory:
        page = Path(directory) / "preview.html"
        profile = Path(directory) / "chrome-profile"
        page.write_text(document, encoding="utf-8")
        command = [
            str(args.chrome), "--headless=new", "--disable-gpu", "--hide-scrollbars",
            "--allow-file-access-from-files", "--no-first-run", "--no-default-browser-check",
            "--disable-background-networking", f"--user-data-dir={profile}",
            "--default-background-color=00000000",
            "--window-size=512,512", "--virtual-time-budget=1500",
            f"--screenshot={output}", page.as_uri(),
        ]
        # Chrome's Windows launcher can return before its screenshot child has
        # written the file, so wait on the output rather than assuming the
        # launcher process lifetime matches the renderer lifetime.
        result = subprocess.run(command, capture_output=True, text=True, timeout=30)
        deadline = time.monotonic() + 20
        while not output.is_file() and time.monotonic() < deadline:
            time.sleep(0.2)
    if not output.is_file():
        raise SystemExit("Chrome preview failed: " + (result.stderr or result.stdout).strip())
    print(f"Rendered frame {args.frame}: {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
