"""Covariates such as age and sex, for adjusted z-scores (W-scores)."""

from __future__ import annotations

import csv
from collections.abc import Mapping, Sequence
from pathlib import Path

import numpy as np

_MISSING = {"", "n/a", "na", "nan", "none", "null"}


def _number(text: str) -> float | None:
    try:
        value = float(text)
    except ValueError:
        return None
    return value if np.isfinite(value) else None


def build_design(covariates: Mapping[str, Sequence]) -> tuple[np.ndarray, list[dict]]:
    """Turn the covariates of the controls into a regression design matrix.

    A covariate whose values are all numbers is used as it is, centred on
    the control mean.  Any other covariate must have exactly two different
    values (e.g. ``"F"`` and ``"M"``) and becomes a 0/1 column.

    Returns the matrix (controls x terms, first column all ones) and a
    description of each covariate, which is what a reference stores so that
    a new subject can be coded the same way.
    """
    if not covariates:
        raise ValueError("no covariates given")
    columns, terms, n = [], [], None
    for name, values in covariates.items():
        texts = [str(v).strip() for v in values]
        if n is None:
            n = len(texts)
            columns.append(np.ones(n))
        if len(texts) != n:
            raise ValueError(f"covariate '{name}' has {len(texts)} values, expected one per control ({n})")
        if any(t.lower() in _MISSING for t in texts):
            raise ValueError(f"covariate '{name}' is missing for some controls")
        numbers = [_number(t) for t in texts]
        if all(v is not None for v in numbers):
            x = np.array(numbers, dtype=np.float64)
            if np.ptp(x) == 0:
                raise ValueError(f"covariate '{name}' is the same for all controls, so it cannot be adjusted for")
            centre = float(x.mean())
            columns.append(x - centre)
            terms.append(
                {"name": name, "type": "numeric", "center": centre, "min": float(x.min()), "max": float(x.max())}
            )
        else:
            levels = sorted(set(texts))
            if len(levels) != 2:
                raise ValueError(f"covariate '{name}' must be numeric or have exactly two values, found {levels[:6]}")
            columns.append(np.array([t == levels[1] for t in texts], dtype=np.float64))
            terms.append({"name": name, "type": "categorical", "levels": levels})
    design = np.column_stack(columns)
    if n < design.shape[1] + 3:
        raise ValueError(f"{n} controls are too few to adjust for {len(terms)} covariate(s)")
    if np.linalg.matrix_rank(design) < design.shape[1]:
        raise ValueError("the covariates are redundant (one can be predicted exactly from the others)")
    return design, terms


def design_row(terms: list[dict], values: Mapping) -> tuple[np.ndarray, list[str]]:
    """Code one subject like the controls were.  Also returns warnings, if any."""
    row, notes = [1.0], []
    for term in terms:
        name = term["name"]
        if name not in values or str(values[name]).strip().lower() in _MISSING:
            raise ValueError(f"covariate '{name}' is needed but missing")
        text = str(values[name]).strip()
        if term["type"] == "numeric":
            value = _number(text)
            if value is None:
                raise ValueError(f"covariate '{name}' must be a number, got '{text}'")
            if not term["min"] <= value <= term["max"]:
                notes.append(
                    f"{name} = {value:g} is outside the range of the controls "
                    f"({term['min']:g} to {term['max']:g}); the adjustment is an extrapolation"
                )
            row.append(value - term["center"])
        else:
            if text not in term["levels"]:
                raise ValueError(f"covariate '{name}' must be one of {term['levels']}, got '{text}'")
            row.append(float(text == term["levels"][1]))
    return np.array(row, dtype=np.float64), notes


def read_table(path: str | Path, images: Sequence[str | Path], names: Sequence[str]) -> list[dict[str, str]]:
    """Look up the covariates of each image in a TSV or CSV table.

    Rows are matched to images through a column called ``image`` (a file
    name or path) or else ``participant_id`` (BIDS style: ``sub-01`` matches
    ``sub-01_..._pet.nii.gz``).  Without either, the first column is used.
    Only the columns in ``names`` are read.
    """
    path = Path(path)
    with open(path, newline="", encoding="utf-8-sig") as fh:
        head = fh.readline()
        fh.seek(0)
        rows = list(csv.DictReader(fh, delimiter="\t" if "\t" in head else ","))
    if not rows:
        raise ValueError(f"{path} has no rows")
    header = list(rows[0])
    absent = [n for n in names if n not in header]
    if absent:
        raise ValueError(f"{path} has no column(s) {absent}; it has {header}")
    key = "image" if "image" in header else "participant_id" if "participant_id" in header else header[0]

    out = []
    for image in images:
        name = Path(image).name
        exact = [r for r in rows if (r[key] or "").strip() in (str(image), name)]
        prefix = [r for r in rows if (k := (r[key] or "").strip()) and name.startswith((k + "_", "sub-" + k + "_"))]
        found = exact or prefix
        if len(found) != 1:
            problem = "no row" if not found else f"{len(found)} rows"
            raise ValueError(f"{problem} in {path.name} (column '{key}') for {name}")
        out.append({n: (found[0][n] or "").strip() for n in names})
    return out
