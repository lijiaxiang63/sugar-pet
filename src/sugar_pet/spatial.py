"""Optional helper: PET-only spatial normalisation to MNI space with SPM.

Intensity normalisation needs images that are already in MNI space.  If you
only have native-space PET (no MRI), this module wraps the classic SPM
"Old Normalise" routine, which warps a PET image onto a PET template.

It needs MATLAB and SPM12/SPM25; nothing else in sugar-pet does.
"""

from __future__ import annotations

import gzip
import shutil
import subprocess
import tempfile
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import nibabel as nib
import numpy as np

# Rough centre of the brain in MNI coordinates (mm). The head's centre of
# mass is moved here so that SPM's affine search starts close to the answer.
_MNI_BRAIN_CENTRE = np.array([0.0, -18.0, 8.0])
# Half-size (mm) of the box kept around the head before registration.
_CROP_HALF_SIZE = np.array([110.0, 130.0, 110.0])

_MATLAB_TEMPLATE = """\
addpath('{spm_path}');
addpath(fullfile('{spm_path}', 'toolbox', 'OldNorm'));
spm('defaults', 'PET');
spm_jobman('initcfg');
spm_get_defaults('cmdline', true);
files = {{
{file_list}
}};
for i = 1:numel(files)
    try
        clear matlabbatch
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.subj.source = files(i);
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.subj.wtsrc = '';
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.subj.resample = files(i);
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.template = {{'{template},1'}};
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.weight = '';
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.smosrc = 8;
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.smoref = {smoref};
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.regtype = 'mni';
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.cutoff = 25;
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.nits = 16;
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.eoptions.reg = 1;
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.roptions.preserve = 0;
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.roptions.bb = [-90 -126 -72; 90 90 108];
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.roptions.vox = [2 2 2];
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.roptions.interp = 1;
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.roptions.wrap = [0 0 0];
        matlabbatch{{1}}.spm.tools.oldnorm.estwrite.roptions.prefix = 'w';
        spm_jobman('run', matlabbatch);
    catch err
        fprintf(2, 'FAILED %s: %s\\n', files{{i}}, err.message);
    end
end
"""


def prepare_native(src: Path, dst: Path) -> None:
    """Crop a native PET around the head and put its origin near the brain centre.

    Scanner images often have their origin far outside the head, which makes
    SPM's registration fail.  Only the header and field of view change; voxel
    values are untouched.
    """
    img = nib.load(src)
    data = np.asanyarray(img.dataobj).astype(np.float32)
    if data.ndim == 4:
        data = data.mean(axis=3)
    data = np.nan_to_num(data, nan=0.0, posinf=0.0, neginf=0.0)
    affine = img.affine.copy()

    # Brain uptake is far higher than anything else in a head scan, so a
    # high threshold isolates it well enough for a centre of mass.
    thr = 0.4 * np.percentile(data, 99.5)
    idx = np.argwhere(data > thr)
    if idx.size == 0:
        raise ValueError(f"{src}: image looks empty")
    centre_vox = idx.mean(axis=0)
    centre_mm = nib.affines.apply_affine(affine, centre_vox)

    voxel_sizes = nib.affines.voxel_sizes(affine)
    half = np.ceil(_CROP_HALF_SIZE / voxel_sizes).astype(int)
    c = np.round(centre_vox).astype(int)
    lo = np.maximum(c - half, 0)
    hi = np.minimum(c + half + 1, data.shape)
    cropped = data[lo[0] : hi[0], lo[1] : hi[1], lo[2] : hi[2]]

    # Shift the affine for the crop, then move the brain centre to MNI space.
    affine[:3, 3] = nib.affines.apply_affine(affine, lo)
    affine[:3, 3] += _MNI_BRAIN_CENTRE - centre_mm

    out = nib.Nifti1Image(cropped, affine)
    out.set_qform(affine, code=1)
    out.set_sform(affine, code=1)
    nib.save(out, dst)


def find_matlab() -> str | None:
    """Locate a MATLAB executable (PATH first, then /Applications on macOS)."""
    exe = shutil.which("matlab")
    if exe:
        return exe
    candidates = sorted(Path("/Applications").glob("MATLAB_R*.app/bin/matlab"))
    return str(candidates[-1]) if candidates else None


def _run_matlab(matlab: str, script: Path, log: Path) -> int:
    # MATLAB leaves helper processes that keep pipes open, so its output must
    # go to a file rather than a pipe or the call never returns.
    with open(log, "w") as fh:
        return subprocess.run(
            [matlab, "-batch", f"run('{script}')"],
            check=False,
            stdout=fh,
            stderr=subprocess.STDOUT,
            stdin=subprocess.DEVNULL,
            cwd=script.parent,
        ).returncode


def spatial_normalize(
    inputs: list[str | Path],
    outputs: list[str | Path],
    spm_path: str | Path,
    template: str | Path | None = None,
    matlab: str | None = None,
    n_jobs: int = 1,
    work_dir: str | Path | None = None,
) -> list[Path]:
    """Warp native-space PET images to MNI space (2 mm, 91x109x91).

    Parameters
    ----------
    inputs, outputs
        Matching lists of native PET files and the paths to write.
    spm_path
        Folder of an SPM12 or SPM25 installation.
    template
        PET template in MNI space.  Defaults to SPM's ``toolbox/OldNorm/PET.nii``.
        A study-specific FDG template (for example the mean of your own
        normalised controls) usually gives a better fit.
    n_jobs
        Number of MATLAB processes to run side by side.

    Returns
    -------
    The outputs that were actually written; failed images are left out.
    """
    if len(inputs) != len(outputs):
        raise ValueError("inputs and outputs must have the same length")
    spm_path = Path(spm_path).expanduser().resolve()
    custom_template = template is not None
    template = Path(template).resolve() if custom_template else spm_path / "toolbox" / "OldNorm" / "PET.nii"
    if not template.exists():
        raise FileNotFoundError(f"PET template not found: {template}")
    matlab = matlab or find_matlab()
    if not matlab:
        raise RuntimeError("MATLAB was not found; pass its path with matlab=...")
    # MATLAB runs in the work folder, where a path relative to this one is not found.
    found = shutil.which(matlab)
    if found:
        matlab = str(Path(found).absolute())

    cleanup = work_dir is None
    work = Path(tempfile.mkdtemp(prefix="sugarpet_")) if cleanup else Path(work_dir)
    work = work.resolve()
    work.mkdir(parents=True, exist_ok=True)
    try:
        if template.suffix == ".gz":
            unzipped = work / "template.nii"
            with gzip.open(template, "rb") as fi, open(unzipped, "wb") as fo:
                shutil.copyfileobj(fi, fo)
            template = unzipped

        native = []
        for i, src in enumerate(inputs):
            dst = work / f"img{i:05d}.nii"
            # A work folder that is kept may hold the warped image of an earlier
            # run, of another subject perhaps: it must not pass for this one's.
            dst.with_name("w" + dst.name).unlink(missing_ok=True)
            prepare_native(Path(src), dst)
            native.append(dst)

        n_jobs = max(1, min(n_jobs, len(native)))
        chunks = [native[i::n_jobs] for i in range(n_jobs)]
        scripts = []
        for j, chunk in enumerate(chunks):
            script = work / f"sugarpet_job{j}.m"
            script.write_text(
                _MATLAB_TEMPLATE.format(
                    spm_path=spm_path,
                    template=template,
                    # SPM's own template is already smooth; a custom mean
                    # image is smoothed to match the source.
                    smoref=8 if custom_template else 0,
                    file_list="\n".join(f"'{p},1'" for p in chunk),
                )
            )
            scripts.append(script)
        with ThreadPoolExecutor(n_jobs) as pool:
            list(pool.map(lambda s: _run_matlab(matlab, s, s.with_suffix(".log")), scripts))

        written = []
        for src, out in zip(native, outputs):
            warped = src.with_name("w" + src.name)
            if not warped.exists():
                continue
            out = Path(out)
            out.parent.mkdir(parents=True, exist_ok=True)
            img = nib.load(warped)
            data = np.nan_to_num(np.asanyarray(img.dataobj).astype(np.float32))
            nib.save(nib.Nifti1Image(data, img.affine), out)
            written.append(out)
        return written
    finally:
        if cleanup:
            shutil.rmtree(work, ignore_errors=True)
