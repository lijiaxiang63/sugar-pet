"""Rebuild the small atlas files bundled in ``src/sugar_pet/data``.

Sources (both freely redistributable, see ``src/sugar_pet/data/LICENSE-MNI``):

* ICBM 2009c nonlinear symmetric template (tissue probability maps)
  http://packages.bic.mni.mcgill.ca/mni-models/icbm152/mni_icbm152_nlin_sym_09c_nifti.zip
* CerebrA atlas (Manera et al., Sci Data 2020)
  http://packages.bic.mni.mcgill.ca/mni-models/icbm152/mni_icbm152_nlin_sym_09c_CerebrA_nifti.zip

Usage:  python scripts/build_atlas.py <folder with the unzipped downloads>
"""

from __future__ import annotations

import csv
import sys
from pathlib import Path

import nibabel as nib
import numpy as np

OUT = Path(__file__).resolve().parents[1] / "src" / "sugar_pet" / "data"


def block_mean(a: np.ndarray) -> np.ndarray:
    """Average 2x2x2 blocks (1 mm -> 2 mm), padding odd dimensions by edge."""
    pad = [(0, s % 2) for s in a.shape]
    a = np.pad(a, pad, mode="edge")
    s = a.shape
    return a.reshape(s[0] // 2, 2, s[1] // 2, 2, s[2] // 2, 2).mean(axis=(1, 3, 5))


def main(src: Path) -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    tissue_dir = src / "mni_icbm152_nlin_sym_09c"
    ref = nib.load(tissue_dir / "mni_icbm152_gm_tal_nlin_sym_09c.nii")
    # 2 mm grid: each new voxel is the mean of 2x2x2 old ones, so its centre
    # sits half a (1 mm) voxel further along each axis.
    affine = ref.affine @ np.diag([2, 2, 2, 1.0])
    affine[:3, 3] = nib.affines.apply_affine(ref.affine, [0.5, 0.5, 0.5])

    brain = block_mean(nib.load(tissue_dir / "mni_icbm152_t1_tal_nlin_sym_09c_mask.nii").get_fdata())
    for tissue in ("gm", "wm"):
        prob = nib.load(tissue_dir / f"mni_icbm152_{tissue}_tal_nlin_sym_09c.nii").get_fdata()
        prob = np.clip(block_mean(prob), 0, 1) * (brain > 0.5)
        img = nib.Nifti1Image(np.round(prob * 255).astype(np.uint8), affine)
        img.header.set_slope_inter(1 / 255.0, 0)
        img.set_qform(affine, code=4)
        img.set_sform(affine, code=4)
        nib.save(img, OUT / f"tpl-MNI152NLin2009cSym_res-2_label-{tissue.upper()}_probseg.nii.gz")

    # Label atlas: majority vote inside each 2x2x2 block.
    labels = np.asanyarray(nib.load(src / "mni_icbm152_CerebrA_tal_nlin_sym_09c.nii").dataobj)
    labels = np.round(labels).astype(np.int16)
    best_p = block_mean((labels == 0).astype(np.float32))
    best_l = np.zeros(best_p.shape, dtype=np.uint8)
    for lab in range(1, int(labels.max()) + 1):
        p = block_mean((labels == lab).astype(np.float32))
        win = p > best_p
        best_l[win] = lab
        best_p[win] = p[win]
    img = nib.Nifti1Image(best_l, affine)
    img.set_qform(affine, code=4)
    img.set_sform(affine, code=4)
    nib.save(img, OUT / "tpl-MNI152NLin2009cSym_res-2_atlas-CerebrA_dseg.nii.gz")

    with open(src / "CerebrA_LabelDetails.csv", newline="") as fh, open(OUT / "atlas-CerebrA_dseg.tsv", "w") as out:
        out.write("index\tname\themisphere\n")
        rows = []
        for row in csv.DictReader(fh):
            name = row["Label Name"].strip()
            rows.append((int(row["RH Label"]), name, "R"))
            rows.append((int(row["LH Labels"]), name, "L"))
        for index, name, hemi in sorted(rows):
            out.write(f"{index}\t{name}\t{hemi}\n")
    print("written to", OUT)


if __name__ == "__main__":
    main(Path(sys.argv[1]))
