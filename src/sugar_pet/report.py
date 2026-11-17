"""The clinical report of a classification: one HTML page that prints well."""

from __future__ import annotations

import base64
import json
import math
import struct
import zlib
from datetime import date
from functools import cache
from importlib import resources

import nibabel as nib
import numpy as np

from .masks import builtin_mask, resample_to

LANGUAGES = ("en", "zh")

# Axial slices of the figure, as indices into the 2 mm atlas grid: cerebellum
# and pons, midbrain, basal ganglia and thalamus, up to the central region.
_SLICES = (20, 26, 30, 36, 40, 44, 50, 56, 62, 68)
_COLUMNS = 5
_ZOOM = 2  # screen pixels per voxel
_GAP = 6  # pixels between slices
_WHITE = 1.5  # normalised uptake shown as white
_FROM, _FULL = 2.0, 5.0  # scores at which the colour starts and saturates

# 5 x 7 letters marking the sides of each slice.
_GLYPHS = {
    "R": ("11110", "10001", "10001", "11110", "10100", "10010", "10001"),
    "L": ("10000", "10000", "10000", "10000", "10000", "10000", "11111"),
}
_LETTER = 200  # gray level of the letters


@cache
def _texts() -> dict:
    return json.loads((resources.files("sugar_pet") / "data" / "classify_text.json").read_text(encoding="utf-8"))


@cache
def _css() -> str:
    return (resources.files("sugar_pet") / "data" / "report.css").read_text(encoding="utf-8")


def text(key: str, lang: str, *args) -> str:
    """A sentence of the report in the given language, with ``{0}``, ``{1}``, ... filled in."""
    out = _texts()[key][lang]
    for i, arg in enumerate(args):
        out = out.replace("{" + str(i) + "}", str(arg))
    return out


def escape(value: str) -> str:
    return value.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace('"', "&quot;")


def number(value: float | None, digits: int = 1) -> str:
    """A score as the report prints it: one decimal and a real minus sign."""
    if value is None:
        return "–"
    out = f"{value:.{digits}f}"
    if float(out) == 0:
        out = out.lstrip("-")
    return out.replace("-", "−")


def percent(value: float) -> int:
    return math.floor(100.0 * value + 0.5)


def png(rgb: np.ndarray) -> bytes:
    """Encode an (rows, columns, 3) array of bytes as a PNG file."""
    rows, columns = rgb.shape[:2]
    raw = b"".join(b"\x00" + rgb[r].tobytes() for r in range(rows))

    def chunk(tag: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)

    header = struct.pack(">IIBBBBB", columns, rows, 8, 2, 0, 0, 0)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", header) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")


def montage(image: nib.Nifti1Image, zmap: nib.Nifti1Image) -> tuple[np.ndarray, list[float]]:
    """Axial slices of the image in gray with the abnormal scores in colour.

    Returns the picture (rows, columns, 3) and the MNI z of each slice.
    Slices are in radiological convention: the patient's left is on the
    right.
    """
    grid = builtin_mask("gray-matter")
    uptake = np.nan_to_num(resample_to(image, grid, order=1))
    score = np.nan_to_num(resample_to(zmap, grid, order=1))
    nx, ny, _ = uptake.shape
    width, height = nx * _ZOOM, ny * _ZOOM
    n_rows = (len(_SLICES) + _COLUMNS - 1) // _COLUMNS
    picture = np.zeros((n_rows * height + (n_rows - 1) * _GAP, _COLUMNS * width + (_COLUMNS - 1) * _GAP, 3), np.uint8)
    for n, k in enumerate(_SLICES):
        # Rows from anterior to posterior, columns from the patient's right to left.
        u = uptake[::-1, ::-1, k].T.astype(np.float64)
        s = score[::-1, ::-1, k].T.astype(np.float64)
        gray = np.floor(np.clip(u / _WHITE, 0.0, 1.0) * 255.0 + 0.5)
        tile = np.stack([gray, gray, gray], axis=-1)
        depth = np.floor(np.clip((np.abs(s) - _FROM) / (_FULL - _FROM), 0.0, 1.0) * 255.0 + 0.5)
        low, high = s <= -_FROM, s >= _FROM
        tile[low] = np.stack([np.zeros_like(depth), depth, np.full_like(depth, 255.0)], axis=-1)[low]
        tile[high] = np.stack([np.full_like(depth, 255.0), depth, np.zeros_like(depth)], axis=-1)[high]
        tile = np.repeat(np.repeat(tile.astype(np.uint8), _ZOOM, axis=0), _ZOOM, axis=1)
        for letter, left in (("R", 4), ("L", width - 4 - 5 * _ZOOM)):
            top = height // 2 - 7 * _ZOOM // 2
            for r, line in enumerate(_GLYPHS[letter]):
                for c, bit in enumerate(line):
                    if bit == "1":
                        tile[top + r * _ZOOM : top + (r + 1) * _ZOOM, left + c * _ZOOM : left + (c + 1) * _ZOOM] = (
                            _LETTER
                        )
        row, column = divmod(n, _COLUMNS)
        y0, x0 = row * (height + _GAP), column * (width + _GAP)
        picture[y0 : y0 + height, x0 : x0 + width] = tile
    levels = [float(grid.affine[2, 2] * k + grid.affine[2, 3]) for k in _SLICES]
    return picture, levels


def _region(name: str, lang: str) -> str:
    return text("region-" + name, lang)


def _place(name: str, side: str, paired: bool, lang: str) -> str:
    """A region with its side, as it stands inside a sentence."""
    region = _region(name, lang).lower()
    return text("of-" + side, lang, region) if paired else region


def _method(name: str, lang: str) -> str:
    return _texts().get("method-" + name, {}).get(lang, name)


def _items(regions: list[dict], wanted, lang: str) -> str:
    """The regions whose status is wanted, most abnormal first, as one line of text."""
    found = []
    for entry in regions:
        name = _region(entry["id"], lang).lower()
        if "left" not in entry:
            if wanted(entry["both"]):
                star = "" if entry["both"]["robust"] else "*"
                found.append((abs(entry["both"]["z"]), text("item-mid", lang, name, number(entry["both"]["z"])) + star))
            continue
        left, right = entry["left"], entry["right"]
        sides = [side for side in ("left", "right") if wanted(entry[side])]
        star = "" if all(entry[side]["robust"] for side in sides) else "*"
        if len(sides) == 2:
            line = text("item-both", lang, name, number(left["z"]), number(right["z"]))
            found.append((max(abs(left["z"]), abs(right["z"])), line + star))
        elif sides:
            z = entry[sides[0]]["z"]
            found.append((abs(z), text("item-" + sides[0], lang, name, number(z)) + star))
    found.sort(key=lambda item: -item[0])  # stable: equal scores stay in the order of the table
    return text("sep", lang).join(line for _, line in found) if found else text("none", lang)


def _chip(feature: dict, paired: dict, lang: str) -> str:
    z = number(feature["z"])
    if feature["direction"] == "asymmetry":
        label = text("chip-asymmetry", lang, z)
    elif feature["direction"] == "island":
        label = text("chip-island", lang, z)
    elif feature["direction"] == "spared":
        label = text("chip-spared", lang)
    else:
        if feature["side"] == "both":
            place = _region(feature["region"], lang)
        else:
            place = _place(feature["region"], feature["side"], paired[feature["region"]], lang)
        label = f"{place} {'↑' if feature['direction'] == 'hyper' else '↓'} {z}"
    level = "on" if feature["evidence"] >= 0.5 else "part" if feature["evidence"] > 0 else "off"
    return f'<span class="chip {level}">{escape(label)}</span>'


def _finding(entry: dict, lang: str) -> str:
    """The last column of the table of regions."""
    if "left" not in entry:
        status = entry["both"]["status"]
        if status == "normal":
            return ""
        return text("status-" + status, lang) + ("" if entry["both"]["robust"] else " *")
    left, right = entry["left"], entry["right"]
    parts = []
    if left["status"] == right["status"]:
        if left["status"] != "normal":
            star = "" if left["robust"] and right["robust"] else " *"
            parts.append(
                text("finding-side", lang, text("status-" + left["status"], lang), text("side-both", lang)) + star
            )
    else:
        for side in ("left", "right"):
            status = entry[side]["status"]
            if status != "normal":
                star = "" if entry[side]["robust"] else " *"
                parts.append(
                    text("finding-side", lang, text("status-" + status, lang), text("side-" + side, lang)) + star
                )
    difference = entry["difference"]
    if difference is not None and abs(difference) >= 1.5 and parts:
        parts.append(text("finding-asymmetric", lang))
    return "; ".join(parts)


def render(result: dict, image: nib.Nifti1Image, zmap: nib.Nifti1Image, lang: str = "en") -> str:
    """The report of one classification as an HTML page."""
    if lang not in LANGUAGES:
        raise ValueError(f"the report is available in {' and '.join(LANGUAGES)}, not '{lang}'")

    def t(key: str, *args) -> str:
        return text(key, lang, *args)

    regions, patterns, conclusion = result["regions"], result["patterns"], result["conclusion"]
    by_id = {p["id"]: p for p in patterns}
    paired = {entry["id"]: "left" in entry for entry in regions}
    sep = t("sep")
    out = []
    add = out.append

    add("<!doctype html>")
    add(f'<html lang="{lang}">')
    add("<head>")
    add('<meta charset="utf-8">')
    add('<meta name="viewport" content="width=device-width, initial-scale=1">')
    add(f"<title>{escape(t('title'))}: {escape(result['image'])}</title>")
    add(f"<style>\n{_css()}</style>")
    add("</head>")
    add("<body>")
    add("<main>")
    add(f"<h1>{escape(t('title'))}</h1>")
    add(f'<p class="subtitle">{escape(t("subtitle"))}</p>')

    # What was analysed, and how.
    reference = result["reference"]
    names = [_method(m, lang) for m in result["checks"]]
    how = t("meta-method-checked", _method(result["method"], lang), sep.join(names)) if names else ""
    how = how or t("meta-method-alone", _method(result["method"], lang))
    if result["score"] == "w":
        scores = t("meta-score-w", sep.join(reference["covariates"]))
    else:
        scores = t("meta-score-z")
    rows = [(t("meta-image"), result["image"]), (t("meta-date"), date.today().isoformat())]
    if result["covariates"]:
        rows.append((t("meta-patient"), sep.join(f"{k} {v}" for k, v in result["covariates"].items())))
    rows.append((t("meta-reference"), t("meta-reference-value", reference["n_controls"], f"{reference['fwhm_mm']:g}")))
    rows += [(t("meta-method"), how), (t("meta-score"), scores)]
    rows.append((t("meta-software"), "sugar-pet " + result["sugar_pet_version"]))
    add('<table class="meta">')
    for label, value in rows:
        add(f"<tr><th>{escape(label)}</th><td>{escape(str(value))}</td></tr>")
    add("</table>")

    # Impression.
    add(f"<h2>{escape(t('section-impression'))}</h2>")
    category, chosen = conclusion["category"], conclusion["pattern"]
    if category == "none":
        sentence = t("concl-none")
    else:
        top = by_id[chosen]
        name, match = escape(t("name-" + chosen)), percent(top["score"])
        if category == "indeterminate":
            other = by_id[conclusion["alternative"]]
            sentence = t("concl-indeterminate", name, match, escape(t("name-" + other["id"])), percent(other["score"]))
        elif category == "possible":
            sentence = t("concl-possible", name, match)
        else:
            sentence = t("concl-" + category, name, match, t("confidence-" + conclusion["confidence"]))
    add(f'<p class="conclusion">{sentence}</p>')
    add('<ul class="findings">')
    add(f"<li>{escape(t('find-hypo'))}: {escape(_items(regions, lambda v: v['status'] == 'hypo', lang))}</li>")
    add(f"<li>{escape(t('find-mild'))}: {escape(_items(regions, lambda v: v['status'] == 'mild-hypo', lang))}</li>")
    hyper = _items(regions, lambda v: v["status"] == "hyper" and v["robust"], lang)
    add(f"<li>{escape(t('find-hyper'))}: {escape(hyper)}</li>")
    brain = result["brain"]
    add(f"<li>{escape(t('find-brain', percent(brain['below']), percent(brain['above'])))}</li>")
    add("</ul>")
    if category in ("pattern", "possible") and conclusion["alternative"]:
        other = by_id[conclusion["alternative"]]
        add(f"<p>{escape(t('concl-alternative', t('name-' + other['id']), percent(other['score'])))}</p>")
    if conclusion["side"]:
        add(f"<p>{escape(t('concl-side', t('side-' + conclusion['side'])))}</p>")
    for name in (chosen, conclusion["alternative"] if category == "indeterminate" else None):
        if name:
            add(f'<p class="caveat">{escape(t("caveat-" + name))}</p>')

    # Pattern match.
    add(f"<h2>{escape(t('section-patterns'))}</h2>")
    add(f'<p class="lead">{escape(t("patterns-lead"))}</p>')
    for p in sorted(patterns, key=lambda p: -p["score"]):
        match = percent(p["score"])
        add(f'<div class="pattern{" chosen" if p["id"] == chosen else ""}">')
        add(f'<div class="name">{escape(t("name-" + p["id"]))}</div>')
        add(f'<div class="bar"><span style="width:{match}%"></span></div>')
        add(f'<div class="value">{match}</div>')
        add(f'<div class="expect">{escape(t("expect-" + p["id"]))}</div>')
        add(f'<div class="chips">{"".join(_chip(f, paired, lang) for f in p["features"])}</div>')
        add("</div>")

    # The map.
    picture, levels = montage(image, zmap)
    add(f"<h2>{escape(t('section-map'))}</h2>")
    add("<figure>")
    add(f'<img alt="" src="data:image/png;base64,{base64.b64encode(png(picture)).decode("ascii")}">')
    low, high = number(-_FULL, 0), number(-_FROM, 0)
    add(
        f'<div class="legend"><span>{low}</span><i class="low"></i><span>{high}</span>'
        f"<span>{escape(t('map-lower'))}</span>"
        f'<i class="high"></i><span>{number(_FROM, 0)} … {number(_FULL, 0)}</span>'
        f"<span>{escape(t('map-higher'))}</span></div>"
    )
    where = sep.join(number(level, 0) for level in levels)
    add(f'<figcaption class="caption">{escape(t("map-caption", where))}</figcaption>')
    add("</figure>")

    # The regions.
    add(f"<h2>{escape(t('section-regions'))}</h2>")
    add('<table class="regions">')
    head = [t("col-region"), t("col-left"), t("col-right"), t("col-difference"), *result["checks"], t("col-finding")]
    add("<tr>" + "".join(f"<th>{escape(h)}</th>" for h in head) + "</tr>")
    for entry in regions:
        cells = [f"<td>{escape(_region(entry['id'], lang))}</td>"]
        if "left" in entry:
            for side in ("left", "right"):
                cells.append(f'<td class="{entry[side]["status"]}">{number(entry[side]["z"])}</td>')
            cells.append(f"<td>{number(entry['difference'])}</td>")
        else:
            cells.append(f'<td class="span {entry["both"]["status"]}" colspan="2">{number(entry["both"]["z"])}</td>')
            cells.append("<td></td>")
        cells += [f"<td>{number(entry['both']['checks'][m])}</td>" for m in result["checks"]]
        cells.append(f"<td>{escape(_finding(entry, lang))}</td>")
        add("<tr>" + "".join(cells) + "</tr>")
    add("</table>")
    add(f'<p class="tablenote">{escape(t("regions-note"))}</p>')

    # Notes and limitations.
    add(f"<h2>{escape(t('section-notes'))}</h2>")
    add('<ul class="notes">')
    for note in result["notes"]:
        if "regions" in note:
            places = sep.join(_place(name, side, paired[name], lang) for name, side in note["regions"])
            line = t("note-" + note["key"], places)
        else:
            line = t("note-" + note["key"], *note["values"])
        add(f'<li class="dynamic">{escape(line)}</li>')
    if result["warnings"]:
        add(f'<li class="dynamic">{escape(t("note-warnings") + " " + "; ".join(result["warnings"]))}</li>')
    for n in range(1, 6):
        add(f"<li>{escape(t('limit-' + str(n)))}</li>")
    add("</ul>")
    add(f'<div class="sign"><div>{escape(t("sign-physician"))}</div><div>{escape(t("sign-date"))}</div></div>')
    add(f"<footer>{escape(t('footer', result['sugar_pet_version']))}</footer>")
    add("</main>")
    add("</body>")
    add("</html>")
    return "\n".join(out) + "\n"
