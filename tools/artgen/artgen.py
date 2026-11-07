# /// script
# requires-python = ">=3.11"
# dependencies = ["pillow>=11"]
# ///
"""Tiên Đạo 2D art pipeline: image API (Meta Muse Image or OpenAI gpt-image) -> post-processing -> game art slots.

Usage (run from the repo root):
  uv run tools/artgen/artgen.py slots                         # list slots + whether a file exists
  uv run tools/artgen/artgen.py gen   --slot items/tu_linh_phu --prompt "single game item icon, ..." [--ref art-work/refs/x.png] [--n 2] [--quality medium]
  uv run tools/artgen/artgen.py place --slot items/tu_linh_phu --src art-work/raw/<file>.png [--keyout-green]

`gen` prepends the shared STYLE and appends the "avoid" list from docs/design/art-prompts.md §0, asks for a
transparent background when the slot needs alpha, saves raw PNGs to art-work/raw/ (git-ignored) and appends every
request to tools/artgen/ledger.jsonl (prompt, model, params, outputs) as the provenance record.
`place` crops to the slot's aspect, cleans the alpha (or keys out a flat green background), resizes and writes
apps/web/src/lib/assets/art/<slot>.webp (+ @2x when the source is large enough).

Providers (keys from the environment or the repo .env — never commit them):
  meta    MODEL_API_KEY   https://api.meta.ai/v1   muse-image-1.0  $0.01/image, opaque only -> green screen + key-out
  openai  OPENAI_API_KEY  https://api.openai.com/v1 gpt-image-1    true transparent PNG, quality low|medium|high
ART_PROVIDER picks one (default: meta if MODEL_API_KEY is set, else openai). ART_MODEL overrides the model id.
"""

from __future__ import annotations

import argparse
import base64
import datetime as dt
import json
import mimetypes
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
ART = ROOT / "apps/web/src/lib/assets/art"
WORK = ROOT / "art-work"
LEDGER = Path(__file__).with_name("ledger.jsonl")
DOC = ROOT / "docs/design/art-prompts.md"

# slot pattern -> (aspect w:h, 1x output size, needs alpha, generation size)
# gpt-image sizes: 1024x1024, 1536x1024, 1024x1536. Crops to the target aspect happen in `place`.
SLOTS: list[tuple[str, tuple[int, int], tuple[int, int], bool, str]] = [
    (r"world/sky", (16, 9), (1920, 1080), False, "1536x1024"),
    (r"world/(far|mid|ground)", (16, 9), (1920, 1080), True, "1536x1024"),
    (r"world/map", (16, 9), (1920, 1080), False, "1536x1024"),
    (r"landmarks/[a-z0-9-]+", (1, 1), (512, 512), True, "1024x1024"),
    (r"characters/avatar", (1, 1), (256, 256), False, "1024x1024"),
    (r"characters/meditate", (1, 1), (1024, 1024), True, "1024x1024"),
    (r"characters/body-[a-z0-9_]+", (2, 3), (1024, 1536), True, "1024x1536"),
    (r"items/[a-z0-9_]+", (1, 1), (128, 128), True, "1024x1024"),
    (r"factions/[a-z0-9_]+", (1, 1), (512, 512), True, "1024x1024"),
    (r"ui/splash", (9, 16), (1080, 1920), False, "1024x1536"),
    (r"scenes/[a-z0-9-]+", (16, 9), (1920, 1080), False, "1536x1024"),
    (r"characters/hong-quan", (1, 1), (768, 768), True, "1024x1024"),
    (r"characters/listener", (1, 1), (256, 256), True, "1024x1024"),
    # reference images only (gen, never `place`): e.g. refs/character-sheet -> art-work/raw, copy to art-work/refs
    (r"refs/[a-z0-9-]+", (3, 2), (1536, 1024), False, "1536x1024"),
]
KNOWN = [
    "world/sky", "world/far", "world/mid", "world/ground", "world/map",
    "landmarks/tu-tieu-cung", "landmarks/dong-phu", "landmarks/the-luc", "landmarks/dao-hoa-dinh", "landmarks/bi-canh",
    "landmarks/chien-truong", "characters/avatar", "characters/meditate", "characters/body-luyen_khi", "items/linh_thach",
    "items/tu_linh_phu", "items/truc_co_dan", "items/ket_dan_qua", "items/linh_thao", "factions/huyen_khung_mon",
    "factions/kim_sa_mon", "factions/thuong_lan_dinh", "factions/tinh_ha_yeu_dinh", "factions/vu_toc", "ui/splash",
    "scenes/tu-tieu-cung", "characters/hong-quan", "characters/listener",
]


def spec(slot: str):
    for pat, aspect, size, alpha, gen in SLOTS:
        if re.fullmatch(pat, slot):
            return aspect, size, alpha, gen
    sys.exit(f"unknown slot {slot!r} — see apps/web/src/lib/assets/art/README.md")


def style_blocks() -> tuple[str, str]:
    """STYLE and the negative list: the first code blocks under '## 0.' and '### Negative prompt' in the doc."""
    text = DOC.read_text(encoding="utf-8")
    sec = text.split("## 0. Style chung", 1)[1]
    style = re.search(r"```\n(.+?)\n```", sec, re.S).group(1).strip()
    neg = re.search(r"### Negative prompt.*?```\n(.+?)\n```", sec, re.S).group(1).strip()
    return style, neg


PROVIDERS = {
    "meta": {"key": "MODEL_API_KEY", "base": "https://api.meta.ai/v1", "model": "muse-image-1.0", "alpha": False},
    "openai": {"key": "OPENAI_API_KEY", "base": "https://api.openai.com/v1", "model": "gpt-image-1", "alpha": True},
}
GREEN = ("isolated on a perfectly flat pure chroma-key green (#00FF00) background filling the whole frame, "
         "no green on the subject, no shadow or gradient on the background")


def env(name: str) -> str | None:
    v = os.environ.get(name)
    f = ROOT / ".env"
    if not v and f.exists():
        for line in f.read_text().splitlines():
            if line.startswith(f"{name}="):
                v = line.split("=", 1)[1].strip().strip('"').strip("'")
    return v or None


def provider() -> tuple[str, dict, str]:
    name = env("ART_PROVIDER") or ("meta" if env("MODEL_API_KEY") else "openai")
    if name not in PROVIDERS:
        sys.exit(f"ART_PROVIDER must be one of {list(PROVIDERS)}")
    p = PROVIDERS[name]
    key = env(p["key"])
    if not key:
        sys.exit(f"{p['key']} missing for provider {name} (add it to the repo .env — never commit or paste it)")
    return name, p, key


class ApiError(Exception):
    pass


def post(url: str, key: str, body: bytes, ctype: str) -> dict:
    req = urllib.request.Request(url, data=body, headers={"Authorization": f"Bearer {key}", "Content-Type": ctype})
    try:
        with urllib.request.urlopen(req, timeout=300) as r:
            return json.loads(r.read())
    except urllib.error.HTTPError as e:
        host = urllib.parse.urlparse(url).hostname
        raise ApiError(f"{host} {e.code}: {e.read().decode(errors='replace')[:800]}") from None


def multipart(fields: dict[str, str], files: list[tuple[str, Path]]) -> tuple[bytes, str]:
    b = uuid.uuid4().hex
    out = bytearray()
    for k, v in fields.items():
        out += f"--{b}\r\nContent-Disposition: form-data; name=\"{k}\"\r\n\r\n{v}\r\n".encode()
    for k, p in files:
        mt = mimetypes.guess_type(p.name)[0] or "image/png"
        out += f"--{b}\r\nContent-Disposition: form-data; name=\"{k}\"; filename=\"{p.name}\"\r\nContent-Type: {mt}\r\n\r\n".encode()
        out += p.read_bytes() + b"\r\n"
    out += f"--{b}--\r\n".encode()
    return bytes(out), f"multipart/form-data; boundary={b}"


def cmd_gen(a):
    _, _, alpha, size = spec(a.slot)
    style, neg = style_blocks()
    name, prov, key = provider()
    green = alpha and not prov["alpha"]  # provider can't do transparency -> green screen, keyed out in `place`
    subject = a.prompt.strip().rstrip(",")
    if green:
        subject = re.sub(r",?\s*isolated on plain flat green background[^,]*", "", subject) + ", " + GREEN
    prompt = f"{style}, {subject}. Avoid: {neg}."
    model = env("ART_MODEL") or prov["model"]
    base = prov["base"]
    entry = {"at": dt.datetime.now().isoformat(timespec="seconds"), "slot": a.slot, "provider": name, "model": model,
             "size": size, "quality": a.quality if name == "openai" else None, "transparent": alpha and not green,
             "green_screen": green, "refs": a.ref or [], "prompt": prompt}
    try:
        if name == "openai":
            fields = {"model": model, "prompt": prompt, "n": str(a.n), "size": size, "quality": a.quality,
                      "background": "transparent" if alpha else "opaque", "output_format": "png"}
            if a.ref:
                body, ctype = multipart(fields, [("image[]", Path(r)) for r in a.ref])
                res = post(f"{base}/images/edits", key, body, ctype)
            else:
                res = post(f"{base}/images/generations", key, json.dumps({**fields, "n": a.n}).encode(), "application/json")
        else:
            # Meta Muse Image: size sets the aspect only; no quality/seed/transparency. Search tools off so the model
            # never pulls real artworks as references (originality rule).
            payload = {"model": model, "prompt": prompt, "n": a.n, "size": size, "output_format": "png",
                       "response_format": "b64_json",
                       "tool_enablement": {"enable_image_search": False, "enable_web_search": False, "enable_shell": False}}
            if a.ref:
                payload["images"] = [{"image_url": "data:image/png;base64," + base64.b64encode(Path(r).read_bytes()).decode()} for r in a.ref]
                res = post(f"{base}/images/edits", key, json.dumps(payload).encode(), "application/json")
            else:
                res = post(f"{base}/images/generations", key, json.dumps(payload).encode(), "application/json")
    except ApiError as e:
        log({**entry, "error": str(e)[:300], "outputs": []})  # failed calls are part of the provenance record
        sys.exit(str(e))
    (WORK / "raw").mkdir(parents=True, exist_ok=True)
    stamp = dt.datetime.now().strftime("%Y%m%d-%H%M%S")
    outs = []
    for i, d in enumerate(res.get("data", [])):
        p = WORK / "raw" / f"{a.slot.replace('/', '__')}-{stamp}-{i}.png"
        p.write_bytes(base64.b64decode(d["b64_json"]))
        outs.append(str(p.relative_to(ROOT)))
    log({**entry, "outputs": outs, "usage": res.get("usage")})
    print("\n".join(outs))


def log(entry: dict):
    with LEDGER.open("a", encoding="utf-8") as f:
        f.write(json.dumps(entry, ensure_ascii=False) + "\n")


def crop_to(img: Image.Image, aspect: tuple[int, int]) -> Image.Image:
    w, h = img.size
    tw = min(w, h * aspect[0] // aspect[1])
    th = min(h, w * aspect[1] // aspect[0])
    left, top = (w - tw) // 2, (h - th) // 2
    return img.crop((left, top, left + tw, top + th))


def key_green(img: Image.Image) -> Image.Image:
    """Chroma-key a flat green background: transparent where green dominates; despill the fringe."""
    img = img.convert("RGBA")
    px = img.load()
    for y in range(img.height):
        for x in range(img.width):
            r, g, b, al = px[x, y]
            dom = g - max(r, b)
            if dom > 60 and g > 120:
                px[x, y] = (r, g, b, 0)
            elif dom > 20:
                px[x, y] = (r, max(r, b), b, int(al * (1 - (dom - 20) / 40)))
    return img


def cmd_place(a):
    if a.slot.startswith("refs/"):
        sys.exit("refs/* are reference images: copy the accepted raw PNG to art-work/refs/ instead of placing it")
    aspect, (tw, th), alpha, _ = spec(a.slot)
    img = Image.open(ROOT / a.src)
    # Alpha slot but an opaque source (Meta, or a green-screen render): key the green out automatically.
    opaque = img.mode != "RGBA" or img.getchannel("A").getextrema()[0] == 255
    if a.keyout_green or (alpha and opaque):
        corners = [img.convert("RGB").getpixel(p) for p in ((2, 2), (img.width - 3, 2), (2, img.height - 3), (img.width - 3, img.height - 3))]
        if not all(g - max(r, b) > 60 for r, g, b in corners):
            print("warning: corners are not chroma green — key-out may leave a background; regenerate", file=sys.stderr)
        img = key_green(img)
    else:
        img = img.convert("RGBA" if alpha else "RGB")
    img = crop_to(img, aspect)
    if alpha and img.mode == "RGBA" and img.getchannel("A").getextrema()[0] == 255:
        print("warning: slot needs alpha but the image is fully opaque (use --keyout-green or regenerate)", file=sys.stderr)
    dest = ART / f"{a.slot}.webp"
    dest.parent.mkdir(parents=True, exist_ok=True)
    sw, _ = img.size
    one = img if sw <= tw else img.resize((tw, th), Image.LANCZOS)
    one.save(dest, "WEBP", quality=82, method=6)
    written = [dest]
    if sw >= tw * 2:
        two = ART / f"{a.slot}@2x.webp"
        img.resize((tw * 2, th * 2), Image.LANCZOS).save(two, "WEBP", quality=80, method=6)
        written.append(two)
    for w in written:
        print(f"{w.relative_to(ROOT)}  {Image.open(w).size}  {w.stat().st_size // 1024} KB")


def cmd_slots(_):
    for s in KNOWN:
        have = any((ART / f"{s}.{e}").exists() for e in ("avif", "webp", "png"))
        aspect, size, alpha, _ = spec(s)
        print(f"{'✓' if have else '·'} {s:32} {aspect[0]}:{aspect[1]}  {size[0]}x{size[1]}{'  alpha' if alpha else ''}")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    g = sub.add_parser("gen")
    g.add_argument("--slot", required=True)
    g.add_argument("--prompt", required=True, help="slot-specific part; STYLE/avoid list are added automatically")
    g.add_argument("--ref", action="append", help="reference image(s) for consistency (uses the edits endpoint)")
    g.add_argument("--n", type=int, default=1)
    g.add_argument("--quality", default="medium", choices=["low", "medium", "high"], help="openai only")
    p = sub.add_parser("place")
    p.add_argument("--slot", required=True)
    p.add_argument("--src", required=True)
    p.add_argument("--keyout-green", action="store_true")
    sub.add_parser("slots")
    a = ap.parse_args()
    {"gen": cmd_gen, "place": cmd_place, "slots": cmd_slots}[a.cmd](a)


if __name__ == "__main__":
    main()
