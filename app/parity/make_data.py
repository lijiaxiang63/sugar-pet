"""Synthetic data for comparing the Go port with the Python package.

Run with the Python of the sugar-pet package: python make_data.py <work>/data
"""
import sys
from pathlib import Path

import nibabel as nib
import numpy as np
from nibabel.processing import resample_from_to

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "tests"))
from conftest import make_pet  # noqa: E402

from sugar_pet import builtin_mask  # noqa: E402
from sugar_pet.classify import region_masks

out = Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
rng = np.random.default_rng(0)
rows = ["participant_id\tage\tsex\tgroup"]
for i in range(16):
    img = make_pet(rng, scale=rng.uniform(3000, 9000))
    nib.save(img, out / f"sub-hc{i:02d}_pet.nii.gz")
    rows.append(f"sub-hc{i:02d}\t{50 + 2 * i + (i % 3)}\t{'FM'[i % 2]}\thc")

mask = builtin_mask("temporal-left").get_fdata() + builtin_mask("sensorimotor").get_fdata()
left = np.zeros(mask.shape, bool)
left[: mask.shape[0] // 2] = True
lesion = ((mask > 0) & left).astype(np.float32)
for i in range(10):
    img = make_pet(np.random.default_rng(100 + i), scale=4000.0 + 300 * i, lesion=lesion, drop=0.25 + 0.02 * i)
    nib.save(img, out / f"sub-pt{i:02d}_pet.nii.gz")
    rows.append(f"sub-pt{i:02d}\t{55 + 3 * i}\t{'MF'[i % 2]}\tpt")

# One patient for each pattern the classification knows: {(region, side): share of the full change}.
regions = region_masks()
cases = {
    "msap": {("putamen", "both"): 1, ("pons", "both"): 0.8, ("cerebellum", "both"): 0.5},
    "msac": {("cerebellum", "both"): 1, ("vermis", "both"): 1, ("pons", "both"): 0.8},
    "psp": {("midbrain", "both"): 1, ("medial-frontal", "both"): 1, ("caudate", "both"): 0.7, ("thalamus", "both"): 0.7},
    "cbd": {("sensorimotor", "right"): 1, ("parietal", "right"): 1, ("thalamus", "right"): 0.7},
    "dlb": {("occipital", "both"): 1, ("parietal", "both"): 0.7, ("precuneus", "both"): 0.7},
    "mixed": {("midbrain", "both"): 1, ("medial-frontal", "both"): 1, ("occipital", "both"): 1, ("parietal", "both"): 0.5},
    "mild": {("putamen", "both"): 0.35},
    "pd": {(r, "both"): -0.8 for r in ("putamen", "pallidum", "thalamus", "pons", "cerebellum", "sensorimotor")},
}
for i, (name, parts) in enumerate(cases.items()):
    change = np.zeros(mask.shape, np.float32)
    for key, share in parts.items():
        change = np.where(regions[key], np.float32(share), change)
    img = make_pet(np.random.default_rng(200 + i), scale=5000.0, lesion=change, drop=0.25)
    nib.save(img, out / f"sub-case{name}_pet.nii.gz")
    rows.append(f"sub-case{name}\t{60 + 2 * i}\t{'MF'[i % 2]}\tpt")
(out / "participants.tsv").write_text("\n".join(rows) + "\n")

# The first patient on another grid (3 mm, shifted and slightly rotated) and as scaled int16.
first = nib.load(out / "sub-pt00_pet.nii.gz")
c, s = np.cos(0.05), np.sin(0.05)
aff = np.array([[3 * c, -3 * s, 0, -88.3], [3 * s, 3 * c, 0, -127.1], [0, 0, 3, -70.6], [0, 0, 0, 1]])
moved = resample_from_to(first, ((61, 73, 61), aff), order=1)
nib.save(nib.Nifti1Image(np.asarray(moved.dataobj, dtype=np.float32), aff), out / "sub-moved_pet.nii.gz")
fine = np.diag([1.0, 1.0, 1.0, 1.0])
fine[:3, 3] = first.affine[:3, 3]
up = resample_from_to(first, ((193, 229, 193), fine), order=1)
nib.save(nib.Nifti1Image(np.asarray(up.dataobj, dtype=np.float32), fine), out / "sub-fine_pet.nii.gz")
data = first.get_fdata()
slope = data.max() / 32000
scaled = nib.Nifti1Image(np.round(data / slope).astype(np.int16), first.affine)
scaled.header.set_slope_inter(slope, 12.5)
nib.save(scaled, out / "sub-int16_pet.nii")
# The first patient on a grid that ends inside the cerebellum (nothing below z = -35 mm).
low = int(np.ceil((-35 - first.affine[2, 3]) / first.affine[2, 2]))
nib.save(first.slicer[:, :, low:], out / "sub-cut_pet.nii.gz")
print("ok")
