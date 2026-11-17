"""Spatial normalisation, with shell scripts standing in for MATLAB."""

import sys

import nibabel as nib
import numpy as np
import pytest

from sugar_pet.cli import main
from sugar_pet.spatial import spatial_normalize

pytestmark = pytest.mark.skipif(sys.platform == "win32", reason="the stand-in for MATLAB is a shell script")

# "Warps" the images of the script it is given by copying them.
COPYING = """script=$(printf '%s' "$2" | sed "s/^run('//; s/')$//")
grep "^'.*,1'$" "$script" | sed "s/^'//; s/,1'$//" | while read -r f; do
  cp "$f" "$(dirname "$f")/w$(basename "$f")"
done
"""


@pytest.fixture
def setup(tmp_path, healthy):
    """A folder that passes for SPM, a native image, and a maker of stand-ins for MATLAB."""
    spm = tmp_path / "spm12"
    (spm / "toolbox" / "OldNorm").mkdir(parents=True)
    (spm / "toolbox" / "OldNorm" / "PET.nii").touch()
    image = tmp_path / "sub-01_pet.nii.gz"
    nib.save(healthy, image)

    def matlab(script):
        exe = tmp_path / "matlab"
        exe.write_text("#!/bin/sh\n" + script)
        exe.chmod(0o755)
        return exe

    return spm, image, matlab


def test_written(tmp_path, setup):
    spm, image, matlab = setup
    out = tmp_path / "out" / "sub-01_space-MNI_pet.nii.gz"
    assert spatial_normalize([image], [out], spm_path=spm, matlab=str(matlab(COPYING))) == [out]
    # The stand-in does not warp: this is the prepared image, its brain centre moved to that of MNI space.
    img = nib.load(out)
    data = img.get_fdata()
    centre = nib.affines.apply_affine(img.affine, np.argwhere(data > 0.4 * np.percentile(data, 99.5)).mean(axis=0))
    assert np.allclose(centre, [0, -18, 8], atol=0.01)


def test_stale_work_dir(tmp_path, setup):
    """A warped image left by an earlier run is not taken for the result of a run that failed."""
    spm, image, matlab = setup
    work = tmp_path / "work"
    work.mkdir()
    nib.save(nib.Nifti1Image(np.ones((4, 4, 4), np.float32), np.eye(4)), work / "wimg00000.nii")
    out = tmp_path / "out.nii.gz"
    written = spatial_normalize([image], [out], spm_path=spm, matlab=str(matlab("exit 1\n")), work_dir=work)
    assert written == []
    assert not out.exists()


def test_relative_matlab(tmp_path, setup, monkeypatch):
    """MATLAB given by a path relative to the working folder is still found: it runs in the work folder."""
    spm, image, matlab = setup
    matlab(COPYING)
    monkeypatch.chdir(tmp_path)
    out = tmp_path / "out.nii.gz"
    assert spatial_normalize([image], [out], spm_path=spm, matlab="./matlab") == [out]


def test_same_output(tmp_path, setup, capsys):
    """Two images that would be written to one file are refused before anything runs."""
    spm, image, matlab = setup
    out = tmp_path / "out.nii.gz"
    with pytest.raises(ValueError, match="would both be written"):
        spatial_normalize([image, image], [out, out], spm_path=spm, matlab=str(matlab(COPYING)))
    assert not out.exists()

    # The command line names the outputs after the inputs, whatever their folders.
    args = ["spatial-norm", "a/pet.nii.gz", "b/pet.nii.gz", "-o", str(tmp_path / "mni"), "--spm-path", str(spm)]
    assert main(args) == 2
    assert "a/pet.nii.gz and b/pet.nii.gz would both be written" in capsys.readouterr().err
