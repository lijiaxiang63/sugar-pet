"""Atypical parkinsonism: which published FDG pattern does one patient's scan look like?

One image is compared with the healthy controls of a reference (a z-score
or W-score map), the map is summarised over the regions that tell the
parkinsonian syndromes apart, and the regional profile is matched against
the patterns described for MSA, PSP, CBD, DLB and PD.  The result is
decision support for the reader of the scan, not a diagnosis.
"""

from __future__ import annotations

import json
import math
import warnings
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from functools import cache
from pathlib import Path

import nibabel as nib
import numpy as np

from . import __version__
from .imageops import load_image
from .masks import _BRAINSTEM, _VERMIS, _atlas, _builtin, get_mask
from .reference import Reference
from .zscore import _zmap

# Regions in the order of the report.  True: one on each side.
REGIONS: tuple[tuple[str, bool], ...] = (
    ("caudate", True),
    ("putamen", True),
    ("posterior-putamen", True),
    ("pallidum", True),
    ("thalamus", True),
    ("midbrain", False),
    ("pons", False),
    ("cerebellum", True),
    ("vermis", False),
    ("medial-frontal", True),
    ("lateral-frontal", True),
    ("sensorimotor", True),
    ("parietal", True),
    ("precuneus", True),
    ("posterior-cingulate", True),
    ("lateral-temporal", True),
    ("occipital", True),
)

# CerebrA labels of the right hemisphere; the left ones are 51 higher.
_RIGHT: dict[str, tuple[int, ...]] = {
    "caudate": (49,),
    "putamen": (21,),
    "pallidum": (27,),
    "thalamus": (40,),
    "cerebellum": (46, 39),  # gray and white matter
    "sensorimotor": (35, 13, 16),  # pre-, post- and paracentral
    "parietal": (9, 10, 51),  # superior and inferior parietal, supramarginal
    "precuneus": (31,),
    "posterior-cingulate": (47, 33),  # posterior and isthmus cingulate
    "lateral-temporal": (3, 28, 45),  # inferior, middle and superior temporal
    "occipital": (34, 43, 6, 12),  # lateral occipital, cuneus, pericalcarine, lingual
}
_ANTERIOR_CINGULATE = (8, 30)  # rostral and caudal
_SUPERIOR_FRONTAL = 38
_LATERAL_FRONTAL = (1, 42, 32, 22)  # rostral and caudal middle frontal, pars opercularis and triangularis
_MEDIAL_X = 12.0  # the superior frontal gyrus within this distance (mm) of the midline counts as medial
_MIDBRAIN_Z = -26.0  # the brainstem above this MNI z (mm), where the built-in pons ends

PATTERNS = ("msa-p", "msa-c", "psp", "cbd", "dlb", "pd")
_GROUP = {"msa-p": "msa", "msa-c": "msa", "psp": "psp", "cbd": "cbd", "dlb": "dlb"}

# A region counts as abnormal from |score| 2, as mildly abnormal from 1.5.
ABNORMAL, MILD = 2.0, 1.5
# A finding is robust when every other normalisation shows it with at least this |score|.
_ROBUST = 1.0
# A region needs this share of its voxels inside the image to be scored.
_MIN_COVERAGE = 0.5
# A voxel centre this close (in voxels) to the edge of the grid still counts as inside it.
_EDGE = 1e-6
# Evidence of a regional decrease (increase) grows from 0 at |score| 1 to 1 at 2.5.
_EVIDENCE_FROM, _EVIDENCE_SPAN = 1.0, 1.5
# The hemispheres must differ by more than 0.5 for an asymmetric pattern, and by 1.5 for full evidence.
_ASYMMETRY_FROM, _ASYMMETRY_SPAN = 0.5, 1.0
# The posterior cingulate must be this much better preserved than precuneus and occipital cortex.
_ISLAND_FROM, _ISLAND_SPAN = 0.5, 1.5
# A pattern is the sum of its core (60 %) and supportive (40 %) features.
_CORE, _SUPPORT = 0.6, 0.4
# Reading of the scores: see ``_conclude``.
_MATCH, _WEAK, _MARGIN, _HIGH, _HIGH_MARGIN = 0.5, 0.3, 0.15, 0.7, 0.3
# Above this share of the brain below -2, the decrease is called widespread.
_WIDESPREAD = 0.25


@cache
def region_masks() -> dict[tuple[str, str], np.ndarray]:
    """The regions as ``{(name, side): mask}`` on the 2 mm atlas grid; side is left, right or both."""
    labels, affine = _atlas()
    x = (affine[0, 0] * np.arange(labels.shape[0]) + affine[0, 3])[:, None, None]
    y = (affine[1, 1] * np.arange(labels.shape[1]) + affine[1, 3])[None, :, None]
    z = (affine[2, 2] * np.arange(labels.shape[2]) + affine[2, 3])[None, None, :]
    medial = np.abs(x) <= _MEDIAL_X
    out: dict[tuple[str, str], np.ndarray] = {}
    for side, shift in (("right", 0), ("left", 51)):
        for name, ids in _RIGHT.items():
            out[name, side] = np.isin(labels, [i + shift for i in ids])
        out["posterior-putamen", side] = out["putamen", side] & (y < 0)
        superior = labels == _SUPERIOR_FRONTAL + shift
        out["medial-frontal", side] = np.isin(labels, [i + shift for i in _ANTERIOR_CINGULATE]) | (superior & medial)
        out["lateral-frontal", side] = np.isin(labels, [i + shift for i in _LATERAL_FRONTAL]) | (superior & ~medial)
    out["midbrain", "both"] = np.isin(labels, _BRAINSTEM) & (z > _MIDBRAIN_Z)
    out["pons", "both"] = _builtin("pons")
    out["vermis", "both"] = np.isin(labels, _VERMIS)
    for name, paired in REGIONS:
        if paired:
            out[name, "both"] = out[name, "left"] | out[name, "right"]
    return out


def default_checks(reference: Reference, method: str = "hn") -> list[str]:
    """Normalisations a finding is checked against: global scaling, and white matter if the reference has it."""
    checks = ["ps"] + (["white-matter"] if "white-matter" in reference.stats else [])
    return [m for m in checks if m != method]


def _clamp(value: float) -> float:
    return min(max(value, 0.0), 1.0)


def _low(z: float | None) -> float:
    """Evidence (0 to 1) that a region's uptake is decreased."""
    return 0.0 if z is None else _clamp((-z - _EVIDENCE_FROM) / _EVIDENCE_SPAN)


def _high(z: float | None) -> float:
    """Evidence (0 to 1) that a region's uptake is increased."""
    return 0.0 if z is None else _clamp((z - _EVIDENCE_FROM) / _EVIDENCE_SPAN)


def _mean(values: Sequence[float]) -> float:
    return sum(values) / len(values)


def _status(z: float | None) -> str:
    if z is None:
        return "none"
    if z <= -ABNORMAL:
        return "hypo"
    if z <= -MILD:
        return "mild-hypo"
    if z >= ABNORMAL:
        return "hyper"
    if z >= MILD:
        return "mild-hyper"
    return "normal"


def _confirmed(value: float, others: Sequence[float | None]) -> bool:
    """Whether every other normalisation shows the finding too: same direction, |score| of 1 or more."""
    return all(v is not None and v * value > 0 and abs(v) >= _ROBUST for v in others)


def _steady(value: float | None, others: Sequence[float | None]) -> float | None:
    """The score as it counts for the robust match: 0 where it is evidence the other normalisations do not confirm.

    Evidence begins at a |score| of 1, below the 1.5 from which a region is
    called abnormal, so this goes by the score and not by that status.  A
    score that is no evidence is left as it is.
    """
    if value is None or abs(value) <= _EVIDENCE_FROM or _confirmed(value, others):
        return value
    return 0.0


def _lowest(value: float | None, others: Sequence[float | None]) -> float | None:
    """What an increase must show: the lowest score under any normalisation.

    None if one of them could not score the region: an increase that was
    not checked is not confirmed.
    """
    if value is None or any(v is None for v in others):
        return None
    return min([value, *others])


def _round(value: float | None) -> float | None:
    return None if value is None else round(float(value), 4)


@cache
def _region_voxels() -> dict[tuple[str, str], np.ndarray]:
    """The voxels of each region on the grid of the atlas, as indices."""
    return {key: np.argwhere(mask).astype(np.int16) for key, mask in region_masks().items()}


def _in_view(target: nib.Nifti1Image) -> dict[tuple[str, str], float]:
    """The share of each region that lies inside the grid of ``target``.

    On a grid that ends inside a region only a part of the region is left:
    what is scored there must be counted against the whole region.
    """
    to_target = np.linalg.inv(target.affine) @ _atlas()[1]
    upper = np.asarray(target.shape[:3], dtype=np.float64) - 0.5 + _EDGE
    out = {}
    for key, voxels in _region_voxels().items():
        ijk = voxels.astype(np.float64) @ to_target[:3, :3].T + to_target[:3, 3]
        out[key] = float(((ijk > -0.5 - _EDGE) & (ijk < upper)).all(axis=1).mean()) if len(voxels) else 0.0
    return out


def _summarise(z: np.ndarray, brain: np.ndarray, mask: np.ndarray, in_view: float = 1.0) -> dict:
    """Mean score of a region, and how much of it is abnormal.

    ``mask`` is the region on the grid of the image, ``in_view`` the share of
    the whole region that this grid holds.
    """
    total = int(mask.sum())
    inside = mask & brain
    n = int(inside.sum())
    coverage = n / total * in_view if total else 0.0
    if coverage < _MIN_COVERAGE:
        return {"z": None, "coverage": coverage, "below": None, "above": None}
    values = z[inside].astype(np.float64)
    return {
        "z": float(values.mean()),
        "coverage": coverage,
        "below": float((values <= -ABNORMAL).mean()),
        "above": float((values >= ABNORMAL).mean()),
    }


def _feature(role: str, region: str, side: str, direction: str, z: float | None, evidence: float) -> dict:
    return {"role": role, "region": region, "side": side, "direction": direction, "z": z, "evidence": evidence}


def _patterns(score: dict, robust_high: dict) -> tuple[list[dict], str | None]:
    """How well the regional profile matches each published pattern (0 to 1).

    ``score[region][side]`` is the regional score under the main
    normalisation; ``robust_high`` the lowest one under any normalisation,
    which is what an increase has to show.
    """

    def low(role, region, side="both"):
        z = score[region][side]
        return _feature(role, region, side, "hypo", z, _low(z))

    def pattern(name, features, core, support, total=None):
        total = _CORE * core + _SUPPORT * support if total is None else total
        return {"id": name, "score": total, "core": core, "support": support, "features": features}

    out = []

    # MSA-P: putamen (the posterior part first), then pons and cerebellum.
    f = [low("core", "putamen"), low("core", "posterior-putamen"), low("support", "pons"), low("support", "cerebellum")]
    out.append(
        pattern("msa-p", f, max(f[0]["evidence"], f[1]["evidence"]), _mean([f[2]["evidence"], f[3]["evidence"]]))
    )

    # MSA-C: cerebellum, then pons and putamen.
    f = [low("core", "cerebellum"), low("support", "pons"), low("support", "putamen")]
    out.append(pattern("msa-c", f, f[0]["evidence"], _mean([f[1]["evidence"], f[2]["evidence"]])))

    # PSP: midbrain and medial frontal cortex, then caudate, thalamus and lateral frontal cortex.
    f = [low("core", "midbrain"), low("core", "medial-frontal")]
    f += [low("support", "caudate"), low("support", "thalamus"), low("support", "lateral-frontal")]
    out.append(pattern("psp", f, _mean([v["evidence"] for v in f[:2]]), _mean([v["evidence"] for v in f[2:]])))

    # CBD: the peri-central and parietal cortex of one hemisphere, with the thalamus and striatum of that side.
    cortex = {}
    for side in ("left", "right"):
        values = [score[r][side] for r in ("sensorimotor", "parietal") if score[r][side] is not None]
        cortex[side] = _mean(values) if values else None
    worse, difference, asymmetry = None, None, 0.0
    if cortex["left"] is not None and cortex["right"] is not None:
        worse = "left" if cortex["left"] < cortex["right"] else "right"
        difference = abs(cortex["left"] - cortex["right"])
        asymmetry = _clamp((difference - _ASYMMETRY_FROM) / _ASYMMETRY_SPAN)
    side = worse or "left"
    f = [_feature("core", "", side, "asymmetry", difference, asymmetry)]
    f += [low("core", "sensorimotor", side), low("core", "parietal", side)]
    f += [low("support", "thalamus", side), low("support", "putamen", side), low("support", "lateral-frontal", side)]
    core = asymmetry * _mean([v["evidence"] for v in f[1:3]])
    out.append(pattern("cbd", f, core, asymmetry * _mean([v["evidence"] for v in f[3:]])))

    # DLB: occipital cortex, then parietal cortex and precuneus with a preserved posterior cingulate.
    occipital = low("core", "occipital")
    island_z, island = None, 0.0
    cingulate, precuneus = score["posterior-cingulate"]["both"], score["precuneus"]["both"]
    if occipital["z"] is not None and cingulate is not None and precuneus is not None:
        island_z = cingulate - (precuneus + occipital["z"]) / 2.0
        if occipital["evidence"] > 0:
            island = _clamp((island_z - _ISLAND_FROM) / _ISLAND_SPAN)
    f = [occipital, low("support", "parietal"), low("support", "precuneus")]
    f.append(_feature("support", "posterior-cingulate", "both", "island", island_z, island))
    out.append(pattern("dlb", f, occipital["evidence"], _mean([v["evidence"] for v in f[1:]])))

    # PD: no decrease in striatum, brainstem or cerebellum; a relative increase there under every normalisation.
    f = []
    for region in ("putamen", "pallidum", "thalamus", "pons", "cerebellum", "sensorimotor"):
        z = robust_high[region]
        f.append(_feature("core", region, "both", "hyper", z, _high(z)))
    spared = 1.0 - max(
        _low(score[r]["both"]) for r in ("putamen", "caudate", "thalamus", "midbrain", "pons", "cerebellum")
    )
    f.append(_feature("support", "", "both", "spared", None, spared))
    core = _mean([v["evidence"] for v in f[:-1]])
    out.append(pattern("pd", f, core, spared, core * spared))
    return out, worse


def _conclude(patterns: list[dict], worse: str | None) -> dict:
    """Read the pattern scores: one pattern, two that cannot be told apart, or none.

    The confidence in a pattern goes by its ``robust`` score, the match
    without the findings that change with the normalisation: *low* if that
    alone would not assign the pattern.
    """
    by_id = {p["id"]: p for p in patterns}
    top = max((by_id[name] for name in _GROUP), key=lambda p: p["score"])
    rivals = [by_id[name] for name in _GROUP if _GROUP[name] != _GROUP[top["id"]]]
    rival = max(rivals, key=lambda p: p["score"])
    margin = top["score"] - rival["score"]
    out = {"category": "none", "pattern": None, "alternative": None, "confidence": None, "side": None}
    if top["score"] >= _MATCH:
        out["pattern"] = top["id"]
        if margin >= _MARGIN:
            out["category"] = "pattern"
            if top["robust"] < _MATCH:
                out["confidence"] = "low"
            else:
                out["confidence"] = "high" if top["robust"] >= _HIGH and margin >= _HIGH_MARGIN else "moderate"
        else:
            out["category"] = "indeterminate"
            out["confidence"] = "low"
        if rival["score"] >= _WEAK:
            out["alternative"] = rival["id"]
    elif top["score"] >= _WEAK:
        out.update(category="possible", pattern=top["id"], confidence="low")
        if rival["score"] >= _WEAK:
            out["alternative"] = rival["id"]
    elif by_id["pd"]["score"] >= _WEAK:
        out.update(category="pd", pattern="pd")
        out["confidence"] = "moderate" if by_id["pd"]["score"] >= _MATCH else "low"
    if out["pattern"] == "cbd" or out["alternative"] == "cbd":
        out["side"] = worse
    return out


@dataclass
class Classification:
    """Outcome of :func:`classify`.

    Attributes
    ----------
    result
        Everything that was measured and concluded, as plain data (what
        ``save`` writes as JSON): regional scores, pattern scores,
        conclusion and notes.
    zmap
        The z-score (or W-score) map under the main normalisation.
    image
        The intensity-normalised image the map was made from.
    """

    result: dict
    zmap: nib.Nifti1Image
    image: nib.Nifti1Image

    @property
    def conclusion(self) -> dict:
        return self.result["conclusion"]

    def report(self, lang: str = "en") -> str:
        """The clinical report as one HTML page (``lang``: ``"en"`` or ``"zh"``)."""
        from .report import render

        return render(self.result, self.image, self.zmap, lang)

    def save(self, folder: str | Path, stem: str, lang: str = "en") -> dict[str, Path]:
        """Write the map, the report and the numbers; returns ``{role: path}``."""
        folder = Path(folder)
        folder.mkdir(parents=True, exist_ok=True)
        suffix = "wmap" if self.result["score"] == "w" else "zmap"
        files = {
            "map": folder / f"{stem}_desc-{self.result['method'].replace('-', '')}_{suffix}.nii.gz",
            "json": folder / f"{stem}_classification.json",
            "report": folder / f"{stem}_report.html",
        }
        nib.save(self.zmap, files["map"])
        files["json"].write_text(json.dumps(self.result, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        files["report"].write_text(self.report(lang), encoding="utf-8")
        return files


def classify(
    image: str | Path | nib.Nifti1Image,
    *,
    reference: Reference | str | Path,
    method: str = "hn",
    checks: Sequence[str] | None = None,
    covariates: Mapping | None = None,
    p_threshold: float = 0.01,
    exclude: str = "hypo",
    max_iter: int = 1,
) -> Classification:
    """Match one patient's FDG-PET image against the patterns of the parkinsonian syndromes.

    The image (in MNI space) is turned into a z-score map against the
    controls of ``reference``, with the normalisation ``method``; with
    ``covariates`` (``{"age": 63, "sex": "F"}``) into a W-score map adjusted
    for them.  The map is averaged over the striatum, thalamus, brainstem,
    cerebellum and the cortical regions that separate the syndromes, each
    side on its own, and the regional profile is scored against the
    patterns of MSA-P, MSA-C, PSP, CBD, DLB and PD.

    ``checks`` are further normalisations the map is made with (default:
    proportional scaling, and white matter if the reference holds its
    statistics).  A finding that only one normalisation shows is reported
    as such, an increase counts for PD only if all of them show it, and the
    confidence in a pattern is lowered if it rests on such findings.

    The rules are taken from the literature, not fitted to data: treat the
    outcome as a structured reading of the map, to be weighed with the
    clinical findings, MRI and dopamine-transporter imaging.
    """
    ref = reference if isinstance(reference, Reference) else Reference.load(reference)
    checks = default_checks(ref, method) if checks is None else [m for m in dict.fromkeys(checks) if m != method]
    for m in (method, *checks):  # fail before doing any work
        ref.control_stats(m)
        if covariates is not None:
            ref.control_model(m)
    img = load_image(image)

    maps, notes = {}, []
    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        for m in (method, *checks):
            maps[m] = _zmap(img, m, ref, covariates, p_threshold, exclude, max_iter)
    for w in caught:
        if str(w.message) not in notes:
            notes.append(str(w.message))
    z, brain, res, row, _ = maps[method]

    target = res.image
    masks = {
        key: get_mask(nib.Nifti1Image(m.astype(np.uint8), _atlas()[1]), target) for key, m in region_masks().items()
    }
    view = _in_view(target)
    regions, score, steady, robust_high = [], {}, {}, {}
    for name, paired in REGIONS:
        entry: dict = {"id": name}
        score[name], steady[name] = {}, {}
        for side in ("left", "right", "both") if paired else ("both",):
            found = _summarise(z, brain, masks[name, side], view[name, side])
            others = {m: _summarise(maps[m][0], maps[m][1], masks[name, side], view[name, side])["z"] for m in checks}
            value = found["z"]
            status = _status(value)
            agree = status in ("none", "normal") or _confirmed(value, others.values())
            found.update(checks=others, status=status, robust=agree)
            entry[side] = found
            score[name][side] = value
            steady[name][side] = _steady(value, others.values())
        robust_high[name] = _lowest(entry["both"]["z"], entry["both"]["checks"].values())
        if paired:
            left, right = score[name]["left"], score[name]["right"]
            entry["difference"] = None if left is None or right is None else left - right
        regions.append(entry)

    patterns, worse = _patterns(score, robust_high)
    # The match that is left without the findings that change with the normalisation.
    for found, kept in zip(patterns, _patterns(steady, robust_high)[0], strict=True):
        found["robust"] = min(found["score"], kept["score"])
    conclusion = _conclude(patterns, worse)
    unchecked = _conclude([{**found, "robust": found["score"]} for found in patterns], worse)
    inside = z[brain].astype(np.float64)
    below = float((inside <= -ABNORMAL).mean()) if inside.size else 0.0
    above = float((inside >= ABNORMAL).mean()) if inside.size else 0.0

    remarks = []
    if conclusion["confidence"] != unchecked["confidence"]:
        match = next(found["robust"] for found in patterns if found["id"] == conclusion["pattern"])
        remarks.append({"key": "lowered", "values": [math.floor(100 * match + 0.5)]})
    if ref.n < 30:
        remarks.append({"key": "few-controls", "values": [ref.n]})
    if covariates is None:
        remarks.append({"key": "unadjusted", "values": []})
    if below > _WIDESPREAD:
        remarks.append({"key": "widespread", "values": [math.floor(100 * below + 0.5)]})
    listed = {"not-covered": [], "dependent": [], "global-only": []}
    for entry in regions:
        for side in ("left", "right") if "left" in entry else ("both",):
            found = entry[side]
            if found["z"] is None:
                listed["not-covered"].append([entry["id"], side])
            elif not found["robust"]:
                listed["dependent"].append([entry["id"], side])
            scaled = found["checks"].get("ps")
            if found["z"] is not None and scaled is not None and scaled >= ABNORMAL and found["z"] < MILD:
                listed["global-only"].append([entry["id"], side])
    remarks += [{"key": key, "regions": found} for key, found in listed.items() if found]

    def tidy(value):
        """Round the numbers, so that the file reads well."""
        if isinstance(value, dict):
            return {k: tidy(v) for k, v in value.items()}
        if isinstance(value, list):
            return [tidy(v) for v in value]
        return _round(value) if isinstance(value, float) else value

    result = {
        "sugar_pet_version": __version__,
        "image": Path(image).name if isinstance(image, (str, Path)) else "image",
        "score": "z" if row is None else "w",
        "covariates": None if covariates is None else {name: str(covariates[name]).strip() for name in ref.covariates},
        "reference": {"n_controls": ref.n, "fwhm_mm": ref.fwhm, "covariates": ref.covariates},
        "method": method,
        "checks": list(checks),
        "factors": {m: float(f"{maps[m][2].factor:.6g}") for m in (method, *checks)},
        "brain": tidy({"below": below, "above": above}),
        "regions": tidy(regions),
        "patterns": tidy(patterns),
        "conclusion": conclusion,
        "notes": remarks,
        "warnings": notes,
    }
    out = nib.Nifti1Image(z, target.affine)
    out.header.set_xyzt_units("mm")
    kind = "z-score" if row is None else "w-score"
    out.header["descrip"] = f"sugar-pet {kind} {method} factor={res.factor:.6g} n={ref.n}"[:79]
    return Classification(result, out, target)
