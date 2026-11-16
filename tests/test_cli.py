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
    assert rows[0].split("\t") == ["image", "method", "factor", "n_voxels", "excluded_fraction"]
    assert [r.split("\t")[1] for r in rows[1:]] == ["cerebellum", "hn", "ihn"]
    factor = float(rows[2].split("\t")[2])
    normed = nib.load(out / "sub-01_desc-hn_pet.nii.gz").get_fdata()
    assert np.allclose(normed * factor, patient.get_fdata(), rtol=1e-3)
    assert (out / "sub-01_desc-ihn_pet_mask.nii.gz").exists()


def test_all_methods_and_errors(tmp_path, healthy, capsys):
    path = str(tmp_path / "x.nii.gz")
    nib.save(healthy, path)
    out = tmp_path / "out"
    assert main(["normalize", path, "-m", "all", "-o", str(out), "--factors-only"]) == 0
    assert not list(out.glob("*.nii.gz"))
    assert len((out / "factors.tsv").read_text().strip().splitlines()) == 1 + 10
    # a reference-based method without a reference fails cleanly
    assert main(["normalize", path, "-m", "hn", "-o", str(out)]) == 1
    assert main(["normalize", path, "-m", "nope", "-o", str(out)]) == 2
    assert main(["methods"]) == 0
    assert main(["masks", "-o", str(tmp_path / "masks")]) == 0
    assert (tmp_path / "masks" / "pons.nii.gz").exists()
