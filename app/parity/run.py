"""Run the same commands with the Python package and the Go port, then compare every output.

    go build -o <work>/sugar-pet-cli ./cmd/sugar-pet-cli
    python parity/make_data.py <work>/data
    python parity/run.py <work> [command ...]

Needs the Python package installed (with GrouVox, for compare) beside this
script's Python.  Images should agree to about 1e-6 of their range, tables
and reports exactly.
"""
import csv
import json
import subprocess
import sys
import time
from pathlib import Path

import nibabel as nib
import numpy as np

S = Path(sys.argv[1]).resolve()
D = S / "data"
PY = [sys.executable, "-m", "sugar_pet.cli"]
GO = [str(S / "sugar-pet-cli")]
hc = [str(p) for p in sorted(D.glob("sub-hc*"))]
pt = [str(p) for p in sorted(D.glob("sub-pt*"))]
tsv = str(D / "participants.tsv")

def commands(o):
    ref, ref0 = f"{o}/ref", f"{o}/ref0"
    return {
        "build-ref": ["build-reference", *hc, "-o", ref, "--stats-for", "all", "--covariates", tsv],
        "build-ref0": ["build-reference", *hc[:8], "-o", ref0, "--fwhm", "0", "--mask", "parenchyma"],
        "norm-all": ["normalize", *pt[:3], "-m", "all", "-r", ref, "-o", f"{o}/norm", "--save-masks"],
        "norm-noref": ["normalize", pt[0], "-m", "all", "-o", f"{o}/norm_noref", "--fwhm", "6"],
        "norm-median": ["normalize", pt[1], "-m", "region", "--mask", "pons", "--statistic", "median", "--target", "50", "-o", f"{o}/norm_median"],
        "norm-labels": ["normalize", pt[1], "-m", "region", "--labels", "34,85", "-o", f"{o}/norm_labels", "--save-masks"],
        "norm-iter": ["normalize", *pt[:2], "-m", "ips,ihn", "-r", ref, "--max-iter", "4", "--exclude", "both", "--p-threshold", "0.05", "-o", f"{o}/norm_iter", "--save-masks"],
        "norm-literal": ["normalize", pt[2], "-m", "ips", "-r", ref0, "--literal-ips", "--factors-only", "-o", f"{o}/norm_literal"],
        "norm-moved": ["normalize", str(D / "sub-moved_pet.nii.gz"), str(D / "sub-fine_pet.nii.gz"), str(D / "sub-int16_pet.nii"), "-m", "all", "-r", ref, "-o", f"{o}/norm_moved", "--save-masks"],
        "zmap": ["zmap", *pt[:2], "-m", "hn,ips,ps,ihn,cerebellum,spm-global,pons", "-r", ref, "-o", f"{o}/z"],
        "wmap": ["zmap", *pt[:2], str(D / "sub-moved_pet.nii.gz"), "-m", "hn,ips,occipital", "-r", ref, "-o", f"{o}/w", "--covariates", tsv],
        "cluster": ["reference-cluster", *pt, "-r", ref, "-o", f"{o}/cluster.nii.gz", "--p-threshold", "0.01", "--min-cluster", "0.5"],
        "norm-cluster": ["normalize", pt[0], "-m", "region", "--mask", f"{o}/cluster.nii.gz", "-o", f"{o}/norm_cluster"],
        "compare-grf": ["compare", "--group1", *pt, "--group2", *hc, "-m", "hn", "-r", ref, "-o", f"{o}/cmp", "--covariates", tsv, "--keep-images"],
        "compare-fdr": ["compare", "--group1", *pt, "--group2", *hc, "-m", "cerebellum", "-o", f"{o}/cmp_fdr", "--correction", "fdr", "--name", "pt_vs_hc"],
        "compare-loose": ["compare", "--group1", *pt[:5], "--group2", *hc[:6], "-m", "ps", "-o", f"{o}/cmp_loose", "--voxel-p", "0.01", "--fwhm", "10", "--analysis-mask", "parenchyma"],
        "masks": ["masks", "-o", f"{o}/masks"],
    }

only = sys.argv[2:]
for name in commands("x"):
    if only and name not in only:
        continue
    for label, exe in (("py", PY), ("go", GO)):
        args = commands(str(S / label))[name]
        t = time.time()
        r = subprocess.run(exe + args, capture_output=True, text=True)
        print(f"{name:14s} {label} rc={r.returncode} {time.time() - t:6.1f}s", flush=True)
        (S / f"{label}_{name}.log").write_text(r.stdout + "\n--- stderr\n" + r.stderr)
        if r.returncode not in (0, 1):
            print(r.stderr[-800:])

print("\n=== comparison ===")
worst = {}
pyfiles = sorted(p for p in (S / "py").rglob("*") if p.is_file())
gofiles = {p.relative_to(S / "go") for p in (S / "go").rglob("*") if p.is_file()}
for p in pyfiles:
    rel = p.relative_to(S / "py")
    g = S / "go" / rel
    if not g.exists():
        print("MISSING in go:", rel)
        continue
    gofiles.discard(rel)
    if p.name.endswith((".nii.gz", ".nii")):
        a, b = nib.load(p), nib.load(g)
        x, y = a.get_fdata(), b.get_fdata()
        if x.shape != y.shape or not np.allclose(a.affine, b.affine, atol=1e-4):
            print("SHAPE/AFFINE differ:", rel, x.shape, y.shape)
            continue
        fin = np.isfinite(x) & np.isfinite(y)
        same_nonfinite = np.array_equal(np.isfinite(x), np.isfinite(y)) and np.array_equal(x[~fin], y[~fin])
        diff = np.abs(x[fin] - y[fin])
        scale = max(np.abs(x[fin]).max(), 1e-12)
        nbad = int((diff > 1e-4 * scale).sum())
        da, db = str(a.header["descrip"].astype(str)), str(b.header["descrip"].astype(str))
        flag = "" if (nbad == 0 and same_nonfinite and da == db) else "  <<<<<<"
        print(f"{str(rel):62s} max|d|/max={diff.max() / scale:.2e} n(>1e-4)={nbad:6d} nonfinite_ok={same_nonfinite} descr_same={da == db}{flag}")
        if da != db:
            print("     py:", da, "\n     go:", db)
    else:
        ta, tb = p.read_text(), g.read_text()
        if p.suffix == ".json":
            ja, jb = json.loads(ta), json.loads(tb)
            def close(u, v):
                if isinstance(u, dict):
                    return isinstance(v, dict) and u.keys() == v.keys() and all(close(u[k], v[k]) for k in u)
                if isinstance(u, list):
                    return isinstance(v, list) and len(u) == len(v) and all(close(i, j) for i, j in zip(u, v))
                if isinstance(u, (int, float)) and not isinstance(u, bool):
                    return abs(u - v) <= 1e-6 * max(abs(u), 1e-12)
                return u == v or (isinstance(u, str) and "/py/" in u)
            ja.pop("sugar_pet_version", None); jb.pop("sugar_pet_version", None)
            print(f"{str(rel):62s} json close={close(ja, jb)}")
            if not close(ja, jb):
                for k in ja:
                    if not close(ja[k], jb.get(k)):
                        print("   key", k, str(ja[k])[:300], "\n   vs ", str(jb.get(k))[:300])
        else:
            ta, tb = ta.replace("/py/", "/X/"), tb.replace("/go/", "/X/")
            print(f"{str(rel):62s} text identical={ta == tb}")
            if ta != tb:
                la, lb = ta.splitlines(), tb.splitlines()
                shown = 0
                for i in range(max(len(la), len(lb))):
                    u = la[i] if i < len(la) else "<none>"
                    v = lb[i] if i < len(lb) else "<none>"
                    if u != v and shown < 6:
                        shown += 1
                        print("   py:", u[:230], "\n   go:", v[:230])
for rel in sorted(gofiles):
    print("EXTRA in go:", rel)
