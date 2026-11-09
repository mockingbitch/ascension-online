# /// script
# requires-python = ">=3.11"
# dependencies = ["pillow>=11"]
# ///
"""Tiên Đạo 2D art pipeline: image API (Meta Muse Image or OpenAI gpt-image) -> post-processing -> game art slots.

Usage (run from the repo root):
  uv run tools/artgen/artgen.py slots                         # list slots + whether a file exists
  uv run tools/artgen/artgen.py gen   --slot items/tu_linh_phu --prompt "single game item icon, ..." [--ref art-work/refs/x.png] [--n 2] [--quality medium]
  uv run tools/artgen/artgen.py place --slot items/tu_linh_phu --src art-work/raw/<file>.png [--key green|magenta|black]
  uv run tools/artgen/artgen.py gen ... --key magenta --dry-run   # print the request only
  uv run tools/artgen/artgen.py selftest                          # offline checks: slot specs + keyers

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
    # per-body art (appearances.yaml ids, lazy-loaded by charArt.svelte.ts) and the guide Vân Khê
    (r"characters/guide/[a-z_]+", (3, 4), (768, 1024), True, "1024x1536"),
    (r"characters/acc/[a-z0-9_]+", (1, 1), (128, 128), True, "1024x1024"),
    # alternate painted outfits: characters/<body_id>/<outfit_id>/<same names> (appearances.yaml outfits[])
    (r"characters/[a-z0-9_]+/[a-z0-9_]+/avatar", (1, 1), (256, 256), False, "1024x1024"),
    (r"characters/[a-z0-9_]+/[a-z0-9_]+/avatar-robe", (1, 1), (256, 256), True, "1024x1024"),
    (r"characters/[a-z0-9_]+/[a-z0-9_]+/meditate(-[a-z0-9_]+)?", (1, 1), (1024, 1024), True, "1024x1024"),
    (r"characters/[a-z0-9_]+/[a-z0-9_]+/listener", (1, 1), (256, 256), True, "1024x1024"),
    (r"characters/[a-z0-9_]+/[a-z0-9_]+/body-[a-z0-9_]+", (2, 3), (1024, 1536), True, "1024x1536"),
    (r"characters/[a-z0-9_]+/avatar", (1, 1), (256, 256), False, "1024x1024"),
    (r"characters/[a-z0-9_]+/avatar-robe", (1, 1), (256, 256), True, "1024x1024"),
    (r"characters/[a-z0-9_]+/meditate(-[a-z0-9_]+)?", (1, 1), (1024, 1024), True, "1024x1024"),
    (r"characters/[a-z0-9_]+/listener", (1, 1), (256, 256), True, "1024x1024"),
    (r"characters/[a-z0-9_]+/body-[a-z0-9_]+", (2, 3), (1024, 1536), True, "1024x1536"),
    # persistent realm aura overlay per effect tier (RealmAura.svelte) — glow art, use --key black
    (r"fx/aura-[1-6]", (1, 1), (512, 512), True, "1024x1024"),
    # reference images only (gen, never `place`): e.g. refs/character-sheet -> art-work/raw, copy to art-work/refs
    (r"refs/[a-z0-9_-]+", (3, 2), (1536, 1024), False, "1536x1024"),
]
GUIDE_EXPR = ["neutral", "smile", "point", "worried", "proud", "surprised", "blink"]
BODY_SLOTS = ["avatar", "avatar-robe", "meditate", "listener"] + [f"body-b{i}" for i in range(1, 7)]
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
# Background keys for providers without real transparency (`--key`):
#   green    default; fine for red / bronze / gold / earth subjects
#   magenta  for jade, teal, green or ink subjects and teal qi (green keying eats them); smoke despilled to ink grey
#   black    for glow / light overlays (fx/aura-*): alpha = luminance, colour un-premultiplied (additive-style light)
KEYS = {
    "green": GREEN,
    "magenta": ("isolated on a perfectly flat pure magenta (#FF00FF) chroma-key background filling the whole frame, "
                "no magenta or pink on the subject, no shadow, no floor, no gradient on the background"),
    "black": ("glowing light and energy only, on a pure flat black (#000000) background filling the whole frame, "
              "no ground, no frame, nothing but light on black"),
}


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
    name, prov, apikey = provider()
    green = alpha and not prov["alpha"]  # provider can't do transparency -> keyed background, removed in `place`
    key = (a.key or env("ART_KEY") or "green") if green else None
    subject = a.prompt.strip().rstrip(",")
    if green:
        subject = re.sub(r",?\s*isolated on plain flat green background[^,]*", "", subject) + ", " + KEYS[key]
    prompt = f"{style}, {subject}. Avoid: {neg}."
    if a.dry_run:
        print(json.dumps({"slot": a.slot, "provider": name, "size": size, "alpha": alpha, "key": key,
                          "refs": a.ref or [], "prompt": prompt}, ensure_ascii=False, indent=1))
        return
    model = env("ART_MODEL") or prov["model"]
    base = prov["base"]
    entry = {"at": dt.datetime.now().isoformat(timespec="seconds"), "slot": a.slot, "provider": name, "model": model,
             "size": size, "quality": a.quality if name == "openai" else None, "transparent": alpha and not green,
             "green_screen": green, "key": key, "refs": a.ref or [], "prompt": prompt}
    try:
        if name == "openai":
            fields = {"model": model, "prompt": prompt, "n": str(a.n), "size": size, "quality": a.quality,
                      "background": "transparent" if alpha else "opaque", "output_format": "png"}
            if a.ref:
                body, ctype = multipart(fields, [("image[]", Path(r)) for r in a.ref])
                res = post(f"{base}/images/edits", apikey, body, ctype)
            else:
                res = post(f"{base}/images/generations", apikey, json.dumps({**fields, "n": a.n}).encode(), "application/json")
        else:
            # Meta Muse Image: size sets the aspect only; no quality/seed/transparency. Search tools off so the model
            # never pulls real artworks as references (originality rule).
            payload = {"model": model, "prompt": prompt, "n": a.n, "size": size, "output_format": "png",
                       "response_format": "b64_json",
                       "tool_enablement": {"enable_image_search": False, "enable_web_search": False, "enable_shell": False}}
            if a.ref:
                payload["images"] = [{"image_url": "data:image/png;base64," + base64.b64encode(Path(r).read_bytes()).decode()} for r in a.ref]
                res = post(f"{base}/images/edits", apikey, json.dumps(payload).encode(), "application/json")
            else:
                res = post(f"{base}/images/generations", apikey, json.dumps(payload).encode(), "application/json")
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
                px[x, y] = (r, max(r, b), b, max(0, int(al * (1 - (dom - 20) / 40))))
    return img


def key_magenta(img: Image.Image) -> Image.Image:
    """Key a flat magenta background (jade/teal subjects survive). Blends with the background (smoke, qi edges) lose
    alpha with the magenta excess and are despilled towards the green level, i.e. ink grey — never violet."""
    img = img.convert("RGBA")
    px = img.load()
    for y in range(img.height):
        for x in range(img.width):
            r, g, b, al = px[x, y]
            m = min(r, b) - g
            if m > 60:
                px[x, y] = (r, g, b, 0)
            elif m > 15:  # small excesses (lips, warm skin) stay untouched
                px[x, y] = (min(r, g + 6), g, min(b, g + 6), max(0, min(255, int(al * (1 - (m - 15) / 45)))))
    return img


def key_black(img: Image.Image) -> Image.Image:
    """Glow on black -> straight alpha: alpha = brightest channel, colour un-premultiplied. Near-black noise dropped."""
    img = img.convert("RGBA")
    px = img.load()
    for y in range(img.height):
        for x in range(img.width):
            r, g, b, _ = px[x, y]
            a = max(r, g, b)
            a = 0 if a < 12 else min(255, int((a - 12) * 255 / 243))
            if a == 0:
                px[x, y] = (0, 0, 0, 0)
            else:
                k = 255 / max(r, g, b)
                px[x, y] = (min(255, int(r * k)), min(255, int(g * k)), min(255, int(b * k)), a)
    return img


KEYERS = {"green": key_green, "magenta": key_magenta, "black": key_black}


def detect_key(img: Image.Image) -> str | None:
    """Which flat background the four corners show (green / magenta / black), else None."""
    rgb = img.convert("RGB")
    cs = [rgb.getpixel(p) for p in ((2, 2), (img.width - 3, 2), (2, img.height - 3), (img.width - 3, img.height - 3))]
    if all(g - max(r, b) > 60 for r, g, b in cs):
        return "green"
    if all(min(r, b) - g > 60 for r, g, b in cs):
        return "magenta"
    if all(max(r, g, b) < 24 for r, g, b in cs):
        return "black"
    return None


def cmd_place(a):
    if a.slot.startswith("refs/"):
        sys.exit("refs/* are reference images: copy the accepted raw PNG to art-work/refs/ instead of placing it")
    aspect, (tw, th), alpha, _ = spec(a.slot)
    img = Image.open(ROOT / a.src)
    # Alpha slot but an opaque source (Meta, or a keyed render): key the background out (green / magenta / black).
    opaque = img.mode != "RGBA" or img.getchannel("A").getextrema()[0] == 255
    key = "green" if a.keyout_green else a.key
    if key or (alpha and opaque):
        found = detect_key(img)
        key = key or found or "green"
        if found != key:
            print(f"warning: corners are not a flat {key} background — key-out may leave a background; regenerate", file=sys.stderr)
        img = KEYERS[key](img)
    else:
        img = img.convert("RGBA" if alpha else "RGB")
    img = crop_to(img, aspect)
    if alpha and img.mode == "RGBA" and img.getchannel("A").getextrema()[0] == 255:
        print("warning: slot needs alpha but the image is fully opaque (use --key or regenerate)", file=sys.stderr)
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


def body_ids() -> list[str]:
    """Body ids of content/catalog/appearances.yaml (no YAML dependency: the `bodies:` block's `- id:` lines)."""
    text = (ROOT / "content/catalog/appearances.yaml").read_text(encoding="utf-8")
    block = text.split("\nbodies:", 1)[1].split("\nhair_accessories:", 1)[0]
    return re.findall(r"^  - id: ([a-z0-9_]+)", block, re.M)


def alt_outfits() -> list[tuple[str, str, str, str]]:
    """(body_id, outfit_id, path, default_pose) of every non-default outfit in appearances.yaml (line scan, no YAML dependency)."""
    text = (ROOT / "content/catalog/appearances.yaml").read_text(encoding="utf-8")
    block = text.split("\nbodies:", 1)[1].split("\nhair_accessories:", 1)[0]
    out = []
    for chunk in re.split(r"\n  - id: ", block)[1:]:
        body = chunk.split("\n", 1)[0].strip()
        path = re.search(r"^    path: (\w+)", chunk, re.M).group(1)
        default = re.search(r"^    default_outfit: (\w+)", chunk, re.M).group(1)
        pose = re.search(r"^    default_pose: (\w+)", chunk, re.M).group(1)
        out += [(body, o, path, pose) for o in re.findall(r"^      - id: (\w+)", chunk, re.M) if o != default]
    return out


POSES = {"tien": ["kiet_gia", "ban_gia", "kiem_ngang"], "vu": ["trung_binh_tan", "toa_thach"]}


def all_slots() -> list[str]:
    per_body = [f"characters/{b}/{s}" for b in body_ids() for s in BODY_SLOTS]
    per_body += [f"characters/{b}/{o}/{s}" for b, o, path, pose in alt_outfits()
                 for s in ["avatar", "meditate", "listener"] + [f"meditate-{p}" for p in POSES[path] if p != pose]
                 + [f"body-b{i}" for i in range(1, 7)]]
    return KNOWN + per_body + [f"characters/guide/{e}" for e in GUIDE_EXPR] + [f"fx/aura-{i}" for i in range(1, 7)]


def cmd_slots(_):
    for s in all_slots():
        have = any((ART / f"{s}.{e}").exists() for e in ("avif", "webp", "png"))
        aspect, size, alpha, _ = spec(s)
        print(f"{'✓' if have else '·'} {s:44} {aspect[0]}:{aspect[1]}  {size[0]}x{size[1]}{'  alpha' if alpha else ''}")


def cmd_selftest(_):
    """Offline checks (no API call): every listed slot resolves, and each keyer clears its background but keeps the
    subject (jade on magenta, red on green, teal glow on black)."""
    bad = [s for s in all_slots() if not any(re.fullmatch(p, s) for p, *_ in SLOTS)]
    assert not bad, f"slots without a spec: {bad}"
    assert spec("characters/tien_nu_y_su/meditate-ban_gia")[2] and not spec("characters/tien_nu_y_su/avatar")[2]
    assert spec("characters/tien_nu_y_su/y_su_tu_yen/meditate-ban_gia")[2] and not spec("characters/tien_nu_y_su/y_su_tu_yen/avatar")[2]
    assert len(alt_outfits()) == 2 * len(body_ids()), "every body has exactly 2 alternate outfits"
    assert spec("characters/guide/smile")[1] == (768, 1024) and spec("fx/aura-6")[1] == (512, 512)
    assert spec("characters/meditate")[1] == (1024, 1024) and spec("characters/body-luyen_khi")[0] == (2, 3)
    cases = [("magenta", (255, 0, 255), (40, 110, 85)), ("green", (0, 255, 0), (150, 30, 30)),
             ("black", (0, 0, 0), (90, 245, 225))]
    for key, bg, fg in cases:
        im = Image.new("RGB", (32, 32), bg)
        im.paste(fg, (8, 8, 24, 24))
        assert detect_key(im) == key, key
        out = KEYERS[key](im)
        assert out.getpixel((1, 1))[3] == 0, f"{key}: background kept"
        assert out.getpixel((16, 16))[3] > 200, f"{key}: subject lost"
    smoke = KEYERS["magenta"](Image.new("RGB", (1, 1), (150, 110, 190))).getpixel((0, 0))
    assert abs(smoke[0] - smoke[2]) <= 6 and smoke[3] < 255, f"magenta smoke not despilled: {smoke}"
    print(f"selftest ok: {len(all_slots())} slots, keyers green/magenta/black")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    g = sub.add_parser("gen")
    g.add_argument("--slot", required=True)
    g.add_argument("--prompt", required=True, help="slot-specific part; STYLE/avoid list are added automatically")
    g.add_argument("--ref", action="append", help="reference image(s) for consistency (uses the edits endpoint)")
    g.add_argument("--n", type=int, default=1)
    g.add_argument("--quality", default="medium", choices=["low", "medium", "high"], help="openai only")
    g.add_argument("--key", choices=list(KEYS), help="background to key out for alpha slots (default green, or ART_KEY)")
    g.add_argument("--dry-run", action="store_true", help="print the request (prompt, size, key) without calling the API")
    p = sub.add_parser("place")
    p.add_argument("--slot", required=True)
    p.add_argument("--src", required=True)
    p.add_argument("--keyout-green", action="store_true", help="same as --key green")
    p.add_argument("--key", choices=list(KEYS), help="force a keyer (default: detected from the corners)")
    sub.add_parser("slots")
    sub.add_parser("selftest")
    a = ap.parse_args()
    {"gen": cmd_gen, "place": cmd_place, "slots": cmd_slots, "selftest": cmd_selftest}[a.cmd](a)


if __name__ == "__main__":
    main()
