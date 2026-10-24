"""Builds every Kinlight logo file in this folder from one set of drawings.

    python3 build.py                      # the lantern marks and app tiles
    python3 build.py --font Fraunces.ttf  # also redraws the wordmark and lockups

The wordmark is drawn from Fraunces' own outlines (variable font from
github.com/google/fonts, ofl/fraunces), so redrawing it needs that file plus
`pip install fonttools uharfbuzz`. Without --font, the wordmark and lockup
files already here are left as they are.

Every drawing is on a 64-unit grid. `themed/` holds copies that take their
colors from CSS custom properties (--mi ink, --ml light, --mg glass core),
for pages that switch theme.
"""

import argparse
import colorsys
import pathlib

HERE = pathlib.Path(__file__).resolve().parent

DAY = {"ink": "#2B211B", "light": "#F3A73B", "core": "#FFE3A6"}
NIGHT = {"ink": "#F4E9DB", "light": "#FFB547", "core": "#FFE8B8"}
THEMED = {"ink": "var(--mi, #2B211B)", "light": "var(--ml, #F3A73B)", "core": "var(--mg, #FFE3A6)"}

TILE = {
    "bg": "#231B16", "ceiling": "#9FD0C9", "bead": "#86BDB5", "trim": "#E9DCCB",
    "brass": "#D9B77A", "light": "#FFB547", "core": "#FFE8B8",
    "floor": "#9A6440", "floor_hi": "#B98159", "seam": "#7A4B2E",
}

# The lantern, on the 64 grid. The cap is a porch roof at a 3:4 pitch, hung
# from a ring; the glass tapers down and is split by one mullion.
GLASS = [(23.5, 24.5), (40.5, 24.5), (38, 45), (26, 45)]
GLOW_CENTER = (32, 34.5)
RINGS = [(22, 0.12), (14.5, 0.24)]


def fill(c):
    # Themed colors are var() expressions, which only work inside style.
    return f'style="fill:{c}"' if c.startswith("var(") else f'fill="{c}"'


def stroke(c, w, extra=""):
    if c.startswith("var("):
        return f'style="fill:none;stroke:{c};stroke-width:{w}{extra}"'
    css = extra.replace(";stroke-linejoin:", '" stroke-linejoin="').replace(";stroke-linecap:", '" stroke-linecap="')
    return f'fill="none" stroke="{c}" stroke-width="{w}{css}"'


def poly(points):
    return "M" + " L".join(f"{x:g} {y:g}" for x, y in points) + " Z"


def lantern(p, rings=True, flame=True):
    out = []
    if rings:
        cx, cy = GLOW_CENTER
        for r, o in RINGS:
            out.append(f'<circle cx="{cx}" cy="{cy}" r="{r}" {fill(p["light"])} opacity="{o}"/>')
    out += [
        f'<circle cx="32" cy="6.5" r="2.6" {stroke(p["ink"], 2)}/>',
        f'<rect x="31" y="8.8" width="2" height="2.6" {fill(p["ink"])}/>',
        f'<path d="M32 10.5 L18 21 L46 21 Z" {fill(p["ink"])}/>',
        f'<path d="M32 10.5 L18 21 L46 21 Z" {stroke(p["ink"], 2, ";stroke-linejoin:round")}/>',
        f'<rect x="19.5" y="20.5" width="25" height="4" rx="1.6" {fill(p["ink"])}/>',
        f'<path d="{poly(GLASS)}" {fill(p["light"])}/>',
    ]
    if flame:
        out.append(f'<ellipse cx="32" cy="37" rx="3.4" ry="5.4" {fill(p["core"])}/>')
    out += [
        f'<path d="{poly(GLASS)} M32 24.5 V45" {stroke(p["ink"], 3, ";stroke-linejoin:round")}/>',
        f'<rect x="24.5" y="44.5" width="15" height="4" rx="1.6" {fill(p["ink"])}/>',
        f'<rect x="28.5" y="48.5" width="7" height="3" rx="1.2" {fill(p["ink"])}/>',
    ]
    return out


def lantern_32(p):
    # Half size, redrawn: a thinner ring, one glow ring, no flame.
    g = [(11.8, 12.5), (20.2, 12.5), (19, 22.5), (13, 22.5)]
    return [
        f'<circle cx="16" cy="17.5" r="10.5" {fill(p["light"])} opacity="0.2"/>',
        f'<circle cx="16" cy="3.2" r="1.8" {stroke(p["ink"], 1.2)}/>',
        f'<path d="M16 5 L8.6 10.6 L23.4 10.6 Z" {fill(p["ink"])}/>',
        f'<path d="M16 5 L8.6 10.6 L23.4 10.6 Z" {stroke(p["ink"], 1, ";stroke-linejoin:round")}/>',
        f'<rect x="9.6" y="10.2" width="12.8" height="2.3" rx="0.8" {fill(p["ink"])}/>',
        f'<path d="{poly(g)}" {fill(p["light"])}/>',
        f'<path d="{poly(g)} M16 12.5 V22.5" {stroke(p["ink"], 1.6, ";stroke-linejoin:round")}/>',
        f'<rect x="12.2" y="22.3" width="7.6" height="2.4" rx="0.8" {fill(p["ink"])}/>',
        f'<rect x="14.2" y="24.6" width="3.6" height="1.6" rx="0.6" {fill(p["ink"])}/>',
    ]


def lantern_16(p):
    # 16 px on a 1x screen, on whole pixels so it stays crisp. Every part of
    # the full drawing survives: the hanging ring (a rounded 4x3 loop), the roof cap
    # and its band, a tapered frame around a 4x4 pane of light, the base and
    # the foot.
    return [
        f'<path d="M7 0 H9 V1 H10 V2 H9 V3 H7 V2 H6 V1 H7 Z M7 1 V2 H9 V1 Z" fill-rule="evenodd" {fill(p["ink"])}/>',
        f'<path d="M8 2 L2 7 H14 Z" {fill(p["ink"])}/>',
        f'<rect x="3" y="7" width="10" height="1" {fill(p["ink"])}/>',
        f'<path d="{poly([(4, 8), (12, 8), (11, 13), (5, 13)])}" {fill(p["ink"])}/>',
        f'<rect x="6" y="8" width="4" height="4" {fill(p["light"])}/>',
        f'<rect x="4" y="12" width="8" height="2" {fill(p["ink"])}/>',
        f'<rect x="6" y="14" width="4" height="1" {fill(p["ink"])}/>',
    ]


def lantern_mono(c="currentColor"):
    # One color: the glass is solid (it's the brightest part), held off the
    # frame and the mullion by a gap, so the lantern still reads as lit.
    return [
        f'<circle cx="32" cy="6.5" r="2.6" fill="none" stroke="{c}" stroke-width="2"/>',
        f'<rect x="31" y="8.8" width="2" height="2.6" fill="{c}"/>',
        f'<path d="M32 10.5 L18 21 L46 21 Z" fill="{c}" stroke="{c}" stroke-width="2" stroke-linejoin="round"/>',
        f'<rect x="19.5" y="20.5" width="25" height="4" rx="1.6" fill="{c}"/>',
        f'<path d="{poly(GLASS)}" fill="none" stroke="{c}" stroke-width="3" stroke-linejoin="round"/>',
        f'<path d="{poly([(26.98, 27.6), (30.4, 27.6), (30.4, 41.9), (28.72, 41.9)])}" fill="{c}"/>',
        f'<path d="{poly([(33.6, 27.6), (37.02, 27.6), (35.28, 41.9), (33.6, 41.9)])}" fill="{c}"/>',
        f'<rect x="24.5" y="44.5" width="15" height="4" rx="1.6" fill="{c}"/>',
        f'<rect x="28.5" y="48.5" width="7" height="3" rx="1.2" fill="{c}"/>',
    ]


def lantern_16_mono(c="currentColor"):
    return [
        f'<path d="M7 0 H9 V1 H10 V2 H9 V3 H7 V2 H6 V1 H7 Z M7 1 V2 H9 V1 Z" fill-rule="evenodd" fill="{c}"/>',
        f'<path d="M8 2 L2 7 H14 Z" fill="{c}"/>',
        f'<rect x="3" y="7" width="10" height="1" fill="{c}"/>',
        f'<path d="{poly([(4, 8), (12, 8), (11, 13), (5, 13)])} M6 8 V12 H10 V8 Z" fill-rule="evenodd" fill="{c}"/>',
        f'<rect x="7" y="9" width="2" height="2" fill="{c}"/>',
        f'<rect x="4" y="12" width="8" height="2" fill="{c}"/>',
        f'<rect x="6" y="14" width="4" height="1" fill="{c}"/>',
    ]


def tile(t, detail=True):
    """The app icon: the lantern hangs under a haint-blue porch ceiling."""
    s, top = 0.74, 16.5
    gy = top + (GLOW_CENTER[1] - 4) * s
    lamp = {"ink": t["brass"], "light": t["light"], "core": t["core"]}
    out = [
        f'<rect width="64" height="64" fill="{t["bg"]}"/>',
        f'<rect width="64" height="12" fill="{t["ceiling"]}"/>',
    ]
    if detail:
        out.append(f'<path d="M0 4 H64 M0 8 H64" stroke="{t["bead"]}" stroke-width="0.8"/>')
    out += [
        f'<rect y="12" width="64" height="2.5" fill="{t["trim"]}"/>',
        f'<circle cx="32" cy="{gy:.2f}" r="{22 if detail else 18}" fill="{t["light"]}" opacity="0.1"/>',
        f'<circle cx="32" cy="{gy:.2f}" r="14.5" fill="{t["light"]}" opacity="0.2"/>',
        f'<path d="M32 14.5 V16.9" stroke="{t["brass"]}" stroke-width="1.2"/>',
        f'<g transform="translate(32 {top}) scale({s}) translate(-32 -4)">',
        *lantern(lamp, rings=False, flame=detail),
        "</g>",
        f'<rect y="54" width="64" height="10" fill="{t["floor"]}"/>',
        f'<path d="M0 54.4 H64" stroke="{t["floor_hi"]}" stroke-width="0.8"/>',
    ]
    if detail:
        out.append(f'<path d="M15 54.8 V64 M47 54.8 V64" stroke="{t["seam"]}" stroke-width="0.8"/>')
    return out


def gray(hexcolor):
    r, g, b = (int(hexcolor[i:i + 2], 16) / 255 for i in (1, 3, 5))
    v = round((0.2126 * r + 0.7152 * g + 0.0722 * b) * 255)
    return f"#{v:02X}{v:02X}{v:02X}"


def svg(view, body, size=None, extra=""):
    w, h = view
    dims = f' width="{size[0]}" height="{size[1]}"' if size else ""
    inner = "\n  ".join(body)
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w:g} {h:g}"{dims}{extra}>\n'
            f'  {inner}\n</svg>\n')


def write(name, text):
    path = HERE / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    print("wrote", path.relative_to(HERE))


def build_rasters():
    """favicon.ico and apple-touch-icon.png, when rsvg-convert and ImageMagick are installed."""
    import shutil
    import subprocess
    import tempfile
    if not (shutil.which("rsvg-convert") and shutil.which("magick")):
        print("skipped favicon.ico and apple-touch-icon.png (needs rsvg-convert and magick)")
        return
    with tempfile.TemporaryDirectory() as tmp:
        frames = []
        for size, src in ((16, "lantern-16.svg"), (32, "lantern-32.svg"), (48, "lantern.svg")):
            out = f"{tmp}/{size}.png"
            subprocess.run(["rsvg-convert", "-w", str(size), "-h", str(size), str(HERE / src), "-o", out], check=True)
            frames.append(out)
        subprocess.run(["magick", *frames, str(HERE / "favicon.ico")], check=True)
        print("wrote favicon.ico")
    subprocess.run(["rsvg-convert", "-w", "180", "-h", "180", str(HERE / "tile.svg"), "-o", str(HERE / "apple-touch-icon.png")], check=True)
    print("wrote apple-touch-icon.png")


def build_marks():
    write("lantern.svg", svg((64, 64), lantern(DAY), (256, 256)))
    write("lantern-on-dark.svg", svg((64, 64), lantern(NIGHT), (256, 256)))
    write("lantern-32.svg", svg((32, 32), lantern_32(DAY), (32, 32)))
    write("lantern-32-on-dark.svg", svg((32, 32), lantern_32(NIGHT), (32, 32)))
    write("lantern-16.svg", svg((16, 16), lantern_16(DAY), (16, 16)))
    write("lantern-16-on-dark.svg", svg((16, 16), lantern_16(NIGHT), (16, 16)))
    write("lantern-mono.svg", svg((64, 64), lantern_mono(), (256, 256)))
    write("lantern-16-mono.svg", svg((16, 16), lantern_16_mono(), (16, 16)))

    # Favicons. A 16 px tab on a 2x screen is drawn with 32 device pixels, so
    # favicon.svg uses the 32 cut, and follows the browser's theme on its own.
    # favicon.ico carries the pixel cuts (16 and 32) for 1x screens and older
    # browsers, which pick the frame that matches their screen.
    fav = {"ink": "var(--i)", "light": "var(--l)", "core": "var(--l)"}
    style = ("<style>svg{--i:#2B211B;--l:#F3A73B}"
             "@media (prefers-color-scheme:dark){svg{--i:#F4E9DB;--l:#FFB547}}</style>")
    write("favicon.svg", svg((32, 32), [style, *lantern_32(fav)], (32, 32)))

    write("tile.svg", svg((64, 64), tile(TILE), (1024, 1024)))
    clip = '<clipPath id="r"><rect width="64" height="64" rx="14.3"/></clipPath>'
    write("tile-rounded.svg", svg((64, 64), [f"<defs>{clip}</defs>", '<g clip-path="url(#r)">', *tile(TILE), "</g>"], (256, 256)))
    write("tile-small.svg", svg((64, 64), tile(TILE, detail=False), (128, 128)))
    write("tile-tinted.svg", svg((64, 64), tile({k: gray(v) for k, v in TILE.items()}), (1024, 1024)))

    write("themed/lantern.svg", svg((64, 64), lantern(THEMED)))
    write("themed/lantern-32.svg", svg((32, 32), lantern_32(THEMED)))
    write("themed/lantern-16.svg", svg((16, 16), lantern_16(THEMED)))
    build_rasters()


# ---- Wordmark -----------------------------------------------------------

WORD = "kinlight"            # both i's lose their dots to lantern panes; the second is lit
AXES = {"opsz": 96, "wght": 620, "SOFT": 100, "WONK": 1}
TRACK = -24                  # font units (2000 per em) between letters


def wordmark_outline(font_path):
    import uharfbuzz as hb
    from fontTools.pens.boundsPen import BoundsPen
    from fontTools.pens.svgPathPen import SVGPathPen
    from fontTools.pens.transformPen import TransformPen
    from fontTools.ttLib import TTFont
    from fontTools.varLib import instancer

    blob = hb.Blob.from_file_path(str(font_path))
    hbfont = hb.Font(hb.Face(blob))
    hbfont.set_variations(AXES)

    def shape(text):
        buf = hb.Buffer()
        buf.add_str(text)
        buf.guess_segment_properties()
        hb.shape(hbfont, buf, {"kern": True, "liga": False})
        return buf.glyph_infos, buf.glyph_positions

    font = instancer.instantiateVariableFont(TTFont(font_path), AXES)
    order = font.getGlyphOrder()
    glyphs = font.getGlyphSet()

    # Set the word with ordinary i's, so the spacing is the font's own, then
    # draw each i without its dot: the dot is replaced by a lantern pane.
    from fontTools.pens.recordingPen import RecordingPen
    rec = RecordingPen()
    glyphs["i"].draw(rec)
    contours, current = [], []
    for op, args in rec.value:
        current.append((op, args))
        if op in ("closePath", "endPath"):
            contours.append(current)
            current = []

    def top(contour):
        return max(pt[1] for _, args in contour for pt in args)

    def bounds(contour):
        pts = [pt for _, args in contour for pt in args]
        return min(p[0] for p in pts), max(p[0] for p in pts), min(p[1] for p in pts), max(p[1] for p in pts)

    dot = max(contours, key=top)
    dx0, dx1, dy0, dy1 = bounds(dot)
    stem_ops = [op for c in contours if c is not dot for op in c]

    infos, positions = shape(WORD)
    pen = SVGPathPen(glyphs)
    x = 0
    dots = []
    for info, pos in zip(infos, positions):
        name = order[info.codepoint]
        ox, oy = x + pos.x_offset, -pos.y_offset
        tp = TransformPen(pen, (1, 0, 0, -1, ox, oy))
        if name == "i":
            for op, args in stem_ops:
                getattr(tp, op)(*args)
            dots.append((ox + (dx0 + dx1) / 2, oy - (dy0 + dy1) / 2, dx1 - dx0))
        else:
            glyphs[name].draw(tp)
        x += pos.x_advance + TRACK
    width = x - TRACK

    lbounds = BoundsPen(glyphs)
    glyphs["l"].draw(lbounds)
    ascender = lbounds.bounds[3]
    gpen = BoundsPen(glyphs)
    glyphs["g"].draw(gpen)
    descender = gpen.bounds[1]

    return {
        "path": pen.getCommands(), "width": width, "ascender": ascender, "descender": descender,
        "dots": dots, "dot": dots[-1],
    }


def pane(cx, cy, w, color):
    # The i's dot, drawn as a lantern pane: wider at the top, tapering down.
    tw, bw, h = w * 0.94, w * 0.66, w * 1.0
    cy += w * 0.08
    pts = [(cx - tw / 2, cy - h / 2), (cx + tw / 2, cy - h / 2), (cx + bw / 2, cy + h / 2), (cx - bw / 2, cy + h / 2)]
    d = poly([(round(a, 1), round(b, 1)) for a, b in pts])
    return f'<path d="{d}" {fill(color)}/><path d="{d}" {stroke(color, round(w * 0.12, 1), ";stroke-linejoin:round")}/>'


def wordmark_body(wm, p, glow=True):
    cx, cy, w = wm["dot"]
    body = []
    if glow:
        for r, o in ((w * 1.8, 0.1), (w * 1.25, 0.22)):
            body.append(f'<circle cx="{cx:.1f}" cy="{cy + w * 0.08:.1f}" r="{r:.1f}" {fill(p["light"])} opacity="{o}"/>')
    body.append(f'<path d="{wm["path"]}" {fill(p["ink"])}/>')
    for dx, dy, dw in wm["dots"][:-1]:
        body.append(pane(dx, dy, dw, p["ink"]))
    body.append(pane(cx, cy, w, p["light"] if glow else p["ink"]))
    return body


def build_wordmark(font_path):
    wm = wordmark_outline(font_path)
    asc, desc, width = wm["ascender"], wm["descender"], wm["width"]
    pad = 40
    top = -asc - wm["dot"][2] * 1.2
    view = f'{-pad} {top - pad:.0f} {width + 2 * pad:.0f} {(-desc) - top + 2 * pad:.0f}'

    def wsvg(body):
        return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{view}">\n  ' + "\n  ".join(body) + "\n</svg>\n"

    mono = {"ink": "currentColor", "light": "currentColor"}
    write("wordmark.svg", wsvg(wordmark_body(wm, DAY)))
    write("wordmark-on-dark.svg", wsvg(wordmark_body(wm, NIGHT)))
    write("wordmark-mono.svg", wsvg(wordmark_body(wm, mono, glow=False)))
    write("themed/wordmark.svg", wsvg(wordmark_body(wm, THEMED)))

    # Lockups. The lantern stands as tall as the l, plus a fifth; its foot
    # sits on the baseline, and the gap is half an x-height.
    lamp_h = 51.5 - 3.9
    scale = asc * 1.2 / lamp_h
    lamp_w = 64 * scale
    gap = asc * 0.28
    lx = -lamp_w - gap
    ly = -asc * 1.2 - 3.9 * scale
    for name, p in (("lockup.svg", DAY), ("lockup-on-dark.svg", NIGHT), ("themed/lockup.svg", THEMED)):
        lamp = f'<g transform="translate({lx:.1f} {ly:.1f}) scale({scale:.3f})">' + "".join(lantern(p)) + "</g>"
        vx, vy = lx - pad, ly - pad
        vw, vh = width - lx + 2 * pad, -desc - ly + 2 * pad
        body = [lamp, *wordmark_body(wm, p)]
        write(name, f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{vx:.0f} {vy:.0f} {vw:.0f} {vh:.0f}">\n  '
              + "\n  ".join(body) + "\n</svg>\n")

    # Stacked: the lantern centered over the word.
    s2 = asc * 1.9 / lamp_h
    sx = width / 2 - 32 * s2
    sy = -asc - wm["dot"][2] * 1.2 - asc * 0.45 - 51.5 * s2
    for name, p in (("lockup-stacked.svg", DAY), ("lockup-stacked-on-dark.svg", NIGHT)):
        lamp = f'<g transform="translate({sx:.1f} {sy:.1f}) scale({s2:.3f})">' + "".join(lantern(p)) + "</g>"
        vy = sy + 3.9 * s2 - 22 * s2 * 0.4 - pad
        body = [lamp, *wordmark_body(wm, p)]
        write(name, f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{-pad - 60} {vy:.0f} {width + 2 * pad + 120:.0f} {-desc - vy + pad:.0f}">\n  '
              + "\n  ".join(body) + "\n</svg>\n")


if __name__ == "__main__":
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--font", type=pathlib.Path, help="Fraunces variable TTF, to redraw the wordmark")
    args = ap.parse_args()
    build_marks()
    if args.font:
        build_wordmark(args.font)
