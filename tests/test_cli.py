import nibabel as nib
import numpy as np

from sugar_pet.cli import main, output_name


def test_output_name():
    assert output_name("a/sub-01_space-MNI_pet.nii.gz", "hn") == "sub-01_space-MNI_desc-hn_pet.nii.gz"
    assert output_name("wFDG.nii", "cerebellar-gm") == "wFDG_desc-cerebellargm.nii.gz"


def test_end_to_end(tmp_path, controls, patient, capsys):
    paths = []
    for i, img in enumerate(controls[:8]):
        paths.append(str(tmp_path / f"hc{i}_pet.nii.gz"))
        nib.save(img, paths[-1])
    patient_path = str(tmp_path / "sub-01_pet.nii.gz")
    nib.save(patient, patient_path)

    ref = str(tmp_path / "ref")
    assert main(["build-reference", *paths, "-o", ref, "--fwhm", "0"]) == 0
    out = tmp_path / "out"
    assert (
        main(["normalize", patient_path, "-m", "cerebellum,hn", "-m", "ihn", "-r", ref, "-o", str(out), "--save-masks"])
        == 0
    )

    rows = (out / "factors.tsv").read_text().strip().splitlines()
    assert rows[0].split("\t") == ["image", "method", "factor", "n_voxels", "excluded_fraction", "mask"]
    assert [r.split("\t")[1] for r in rows[1:]] == ["cerebellum", "hn", "ihn"]
    factor = float(rows[2].split("\t")[2])
    normed = nib.load(out / "sub-01_desc-hn_pet.nii.gz").get_fdata()
    assert np.allclose(normed * factor, patient.get_fdata(), rtol=1e-3)
    assert (out / "sub-01_desc-ihn_pet_mask.nii.gz").exists()

    # z-score maps: ps/hn work with any reference, a reference region needs --stats-for
    zdir = tmp_path / "z"
    assert main(["zmap", patient_path, "-m", "hn,ips", "-r", ref, "-o", str(zdir)]) == 0
    z = nib.load(zdir / "sub-01_desc-hn_zmap.nii.gz").get_fdata()
    assert z.min() < -4 and (zdir / "sub-01_desc-ips_zmap.nii.gz").exists()
    assert main(["zmap", patient_path, "-m", "pons", "-r", ref, "-o", str(zdir)]) == 2
    assert "--stats-for pons" in capsys.readouterr().err
    ref2 = str(tmp_path / "ref2")
    assert main(["build-reference", *paths, "-o", ref2, "--fwhm", "0", "--stats-for", "pons,occipital"]) == 0
    assert main(["zmap", patient_path, "-m", "pons", "-r", ref2, "-o", str(zdir)]) == 0
    assert (zdir / "sub-01_desc-pons_zmap.nii.gz").exists()

    # reference cluster from a (tiny) patient group, then used as a region
    cluster = str(tmp_path / "cluster.nii.gz")
    args = ["reference-cluster", patient_path, patient_path, "-r", ref, "-o", cluster]
    assert main([*args, "--p-threshold", "0.05", "--min-cluster", "0.1"]) == 0
    assert main(["normalize", patient_path, "-m", "region", "--mask", cluster, "-o", str(out), "--factors-only"]) == 0


def test_region_options(tmp_path, healthy, capsys):
    path = str(tmp_path / "sub-01_pet.nii.gz")
    nib.save(healthy, path)
    out = tmp_path / "out"
    base = ["normalize", path, "-o", str(out)]
    assert main([*base, "-m", "region", "--mask", "brainstem", "--statistic", "median", "--target", "100"]) == 0
    row = (out / "factors.tsv").read_text().strip().splitlines()[1].split("\t")
    assert row[1] == "region" and row[5] == "brainstem"
    data = nib.load(out / "sub-01_desc-brainstem_pet.nii.gz").get_fdata()
    mask = nib.load(tmp_path / "sub-01_pet.nii.gz").get_fdata() > 0
    from sugar_pet import builtin_mask

    assert np.median(data[(builtin_mask("brainstem").get_fdata() > 0) & mask]) == np.float32(100)

    assert main([*base, "-m", "region", "--labels", "34,85", "--factors-only"]) == 0
    assert (out / "factors.tsv").read_text().strip().splitlines()[1].endswith("labels:34,85")
    assert main([*base, "-m", "region", "--labels", "34,999"]) == 2
    assert main([*base, "-m", "region", "--labels", "34", "--mask", "pons"]) == 2
    capsys.readouterr()
    assert main([*base, "-m", "brainstem"]) == 2
    assert "-m region --mask brainstem" in capsys.readouterr().err
    assert main(["labels"]) == 0
    assert "Lateral Occipital (R)" in capsys.readouterr().out


def test_all_methods_and_errors(tmp_path, healthy, capsys):
    path = str(tmp_path / "x.nii.gz")
    nib.save(healthy, path)
    out = tmp_path / "out"
    assert main(["normalize", path, "-m", "all", "-o", str(out), "--factors-only"]) == 0
    assert not list(out.glob("*.nii.gz"))
    assert len((out / "factors.tsv").read_text().strip().splitlines()) == 1 + 9
    # a reference-based method without a reference fails cleanly
    assert main(["normalize", path, "-m", "hn", "-o", str(out)]) == 1
    assert main(["normalize", path, "-m", "nope", "-o", str(out)]) == 2
    assert main(["methods"]) == 0
    assert main(["masks", "-o", str(tmp_path / "masks")]) == 0
    assert (tmp_path / "masks" / "pons.nii.gz").exists()


def test_w_scores_from_a_participants_table(tmp_path, controls, patient, capsys):
    paths = []
    table = ["participant_id\tname\tage\tsex"]
    for i, img in enumerate(controls[:12]):
        paths.append(str(tmp_path / f"sub-hc{i:02d}_ses-1_pet.nii.gz"))
        nib.save(img, paths[-1])
        table.append(f"sub-hc{i:02d}\tsomeone\t{40 + 3 * i}\t{'FM'[i % 2]}")
    patient_path = str(tmp_path / "sub-p1_pet.nii.gz")
    nib.save(patient, patient_path)
    table.append("sub-p1\tsomeone\t90\tF")
    tsv = tmp_path / "participants.tsv"
    tsv.write_text("\n".join(table) + "\n")

    ref = str(tmp_path / "ref")
    build = ["build-reference", *paths, "-o", ref, "--fwhm", "0"]
    assert main([*build, "--covariates", str(tsv), "--stats-for", "pons"]) == 0
    assert "someone" not in (tmp_path / "ref" / "reference.json").read_text()  # only what is asked for is read
    out = tmp_path / "w"
    capsys.readouterr()
    assert main(["zmap", patient_path, "-m", "hn,pons", "-r", ref, "-o", str(out), "--covariates", str(tsv)]) == 0
    assert "outside the range of the controls" in capsys.readouterr().err  # age 90
    w = nib.load(out / "sub-p1_desc-hn_wmap.nii.gz")
    assert "w-score" in str(w.header["descrip"]) and w.get_fdata().min() < -4
    assert (out / "sub-p1_desc-pons_wmap.nii.gz").exists()
    # without the table: plain z-scores
    assert main(["zmap", patient_path, "-m", "hn", "-r", ref, "-o", str(out)]) == 0
    assert (out / "sub-p1_desc-hn_zmap.nii.gz").exists()

    # an image that is not in the table, and a reference without covariates
    other = str(tmp_path / "sub-p2_pet.nii.gz")
    nib.save(patient, other)
    assert main(["zmap", other, "-m", "hn", "-r", ref, "-o", str(out), "--covariates", str(tsv)]) == 1
    assert "no row" in capsys.readouterr().err
    plain = str(tmp_path / "plain")
    assert main(build[:-4] + ["-o", plain, "--fwhm", "0"]) == 0
    assert main(["zmap", patient_path, "-m", "hn", "-r", plain, "-o", str(out), "--covariates", str(tsv)]) == 2
    assert "built without covariates" in capsys.readouterr().err
    assert main([*build, "--covariates", str(tsv), "--adjust-for", "age,height"]) == 2
