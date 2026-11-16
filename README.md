# sugar-pet

**S**tandardized **U**ptake **G**lobal **A**nd **R**egional-normalization for brain FDG-**PET**.

[中文说明](README_zh.md)

FDG-PET voxel values depend on the injected dose, blood glucose, body weight and
more, so two scans cannot be compared directly. *Intensity normalisation* fixes
this by dividing each image by one number, the **normalisation factor**.
sugar-pet estimates that factor with the methods people actually use, including
the histogram-based methods of
[López-González et al., NeuroImage 2020](https://doi.org/10.1016/j.neuroimage.2020.117229):

```
normalised image = image / factor
```

Everything else is just a different way of finding `factor`.

## Install

```bash
pip install git+https://github.com/lijiaxiang63/sugar-pet
```

or, to work on the code:

```bash
git clone https://github.com/lijiaxiang63/sugar-pet && cd sugar-pet && uv sync
```

Python 3.10+; depends only on numpy, scipy and nibabel.

## Quick start

Your images must already be **spatially normalised to MNI space** (any voxel
size or bounding box). See [Images not in MNI space yet?](#images-not-in-mni-space-yet)
if they are not.

```bash
# 1. Reference-region method: no other data needed
sugar-pet normalize sub-01_pet.nii.gz -m cerebellum -o out/

# 2. Histogram method (recommended): build a healthy-control reference once ...
sugar-pet build-reference controls/*.nii.gz -o hc_reference/

# ... then use it for every patient
sugar-pet normalize patients/*.nii.gz -m hn -r hc_reference/ -o out/
```

`out/` then contains one normalised image per input
(`sub-01_desc-hn_pet.nii.gz`) and `factors.tsv` with the factor of every image.

## Methods

Run `sugar-pet methods` to see this list in the terminal.

| `-m`             | How the factor is found                                                    | Needs controls |
| ---------------- | -------------------------------------------------------------------------- | :------------: |
| `cerebellum`     | Mean uptake of the whole cerebellum (RRC in the paper)                     |       no       |
| `cerebellar-gm`  | Mean uptake of cerebellar gray matter                                      |       no       |
| `pons`           | Mean uptake of the central pons                                            |       no       |
| `brainstem`      | Mean uptake of the whole brainstem (RRBS)                                  |       no       |
| `white-matter`   | Mean uptake of deep cerebral white matter                                  |       no       |
| `sensorimotor`   | Mean uptake of primary sensorimotor cortex                                 |       no       |
| `temporal-left` / `temporal-right` | Mean uptake of one temporal lobe (RRTL: pick the side *opposite* the lesion) | no |
| `region`         | Mean uptake of your own mask (`--mask my_roi.nii.gz`)                      |       no       |
| `ps`             | **Proportional scaling**: mean uptake of the gray-matter mask              |       no       |
| `spm-global`     | SPM's global mean (all voxels above 1/8 of the image mean)                 |       no       |
| `hn`             | **Histogram normalisation**: divide the image by the mean of the controls, take the *most frequent* ratio | yes |
| `ips`            | **Iterative PS**: do PS, find hypometabolic voxels, redo PS without them   |      yes       |
| `ihn`            | **Iterative HN**: do HN, find hypometabolic voxels, redo HN without them   |      yes       |

Several methods at once: `-m cerebellum,ps,hn`, or `-m all`. Add
`--factors-only` to get just the table.

### Which one should I use?

Following the paper's conclusions:

* **`hn` is a good default.** It needs no assumption about which regions are
  affected and was the best data-driven method. `ihn` gives almost the same
  factor (see [notes](#how-it-works-and-where-it-differs-from-the-paper)).
* **`ps` is biased when hypometabolism is large.** The lesion lowers the global
  mean, the factor comes out too small, and healthy tissue then looks
  *hyper*metabolic. `ips` removes most of that bias.
* **A reference region is fine only if it is large and truly spared by the
  disease.** The cerebellum works well in many dementias but is wrong for
  diseases that affect it (e.g. MSA, cerebellar ataxias). Small regions such
  as the brainstem or pons are noisy.

## The healthy-control reference

`hn`, `ips` and `ihn` compare each image with a group of healthy controls.
`sugar-pet build-reference` summarises the controls into a small folder:

| File               | Content                                                    |
| ------------------ | ---------------------------------------------------------- |
| `template.nii.gz`  | Mean of the controls after proportional scaling (the HN "normality template") |
| `ps_std.nii.gz`    | Standard deviation of the controls after PS                |
| `hn_mean.nii.gz`, `hn_std.nii.gz` | Mean and SD of the controls after HN        |
| `mask.nii.gz`      | Voxels used for the estimate (gray-matter mask)            |
| `reference.json`   | Number of controls, smoothing, factors of each control     |

Use controls from the same scanner and reconstruction as your patients, and
processed the same way. Twenty or more is a sensible minimum.

By default the controls, and later each patient, are smoothed with an 8 mm
Gaussian kernel **only for estimating the factor**; the image that is written
is your input divided by the factor, never smoothed. If your images are already
smoothed, pass `--fwhm 0` to both commands.

## Options for the iterative methods

| Option            | Default | Meaning                                                      |
| ----------------- | ------- | ------------------------------------------------------------ |
| `--p-threshold`   | `0.01`  | Voxels different from controls at this (uncorrected) p are excluded |
| `--exclude`       | `hypo`  | `hypo`: exclude only hypometabolic voxels (paper). `both`: also hypermetabolic ones |
| `--max-iter`      | `1`     | Number of re-estimations (paper: 1)                          |
| `--save-masks`    | off     | Also write the mask each factor was estimated from           |

## Python

```python
from sugar_pet import Reference, normalize

ref = Reference.build(control_files, fwhm=8)     # or Reference.load("hc_reference")
ref.save("hc_reference")

res = normalize("sub-01_pet.nii.gz", method="ihn", reference=ref)
print(res.factor)                 # the normalisation factor
print(res.info)                   # voxels used, fraction excluded, ...
res.save("sub-01_desc-ihn_pet.nii.gz")
res.mask_image()                  # the voxels the factor was estimated from
```

Any reference-region method accepts your own mask, and any set of labels of the
bundled atlas can be turned into one:

```python
from sugar_pet import atlas_labels, labels_mask

atlas_labels()                                   # {21: 'Putamen (R)', 72: 'Putamen (L)', ...}
res = normalize(img, "region", mask=labels_mask([40, 91]))   # both thalami
```

## Built-in masks

`sugar-pet masks -o masks/` writes them as NIfTI files so you can look at them.
They come from two freely redistributable MNI resources, resampled to 2 mm:
the ICBM 2009c tissue probability maps and the
[CerebrA atlas](https://doi.org/10.1038/s41597-020-0557-9). Masks are resampled
to the grid of each image automatically.

The pons mask is the part of the CerebrA brainstem between z = −44 and −26 mm,
eroded in-plane; treat it as a pragmatic central-pons region, not an anatomical
gold standard. For other MNI variants or atlases, pass your own mask.

## Images not in MNI space yet?

Any spatial normalisation works (SPM, ANTs, ...). If you have PET only and SPM
installed, there is a small wrapper around SPM's "Old Normalise" with a PET
template (needs MATLAB):

```bash
sugar-pet spatial-norm raw/*_pet.nii.gz -o mni/ --spm-path /path/to/spm12 --jobs 4
```

It crops each scan around the head, moves the origin to the brain centre and
writes 2 mm MNI-space images. Always check the result visually.

## How it works, and where it differs from the paper

* **HN.** The ratio image (subject / template) is computed inside the
  gray-matter mask and the peak of its histogram is the factor. A raw histogram
  maximum depends on the bin width, so the histogram is smoothed with a
  Gaussian kernel (a kernel density estimate) before the peak is located.
* **Finding abnormal voxels.** The paper runs SPM's two-sample t-test (one
  patient against the controls). sugar-pet computes the equivalent equal-variance
  t statistic directly, `t = (x − mean) / (sd · √(1 + 1/n))` with n − 1 degrees
  of freedom, so no SPM is needed.
* **iPS.** After voxels are removed, the remaining ones no longer average 1 in
  the controls either. sugar-pet therefore divides the subject's mean by the
  control template's mean over the *same* voxels. `--literal-ips` gives the
  plain mean as written in the paper.
* **iHN ≈ HN.** Hypometabolic voxels sit in the tail of the ratio histogram,
  far from its peak, so removing them barely moves the peak. Expect `hn` and
  `ihn` factors to agree within a fraction of a percent; that robustness is
  exactly why HN works well.
* **One scale.** `ps`, `ips`, `hn` and `ihn` are all scaled so that the
  gray-matter mask of a healthy brain averages about 1.

In a simulated image with a large 40 % cortical lesion (see `tests/`), the
factor is off by −3.7 % with `ps`, −0.5 % with `ips` and less than 0.1 % with
`hn` and `ihn`, the same ranking the paper reports.

## Development

```bash
uv sync
uv run pytest
uv run ruff check .
```

`scripts/build_atlas.py` regenerates the bundled atlas files from the original
MNI downloads.

## References

* López-González FJ, Silva-Rodríguez J, Paredes-Pacheco J, et al. Intensity
  normalization methods in brain FDG-PET quantification. *NeuroImage* 2020;222:117229.
* Manera AL, Dadar M, Fonov V, Collins DL. CerebrA, registration and manual
  label correction of Mindboggle-101 atlas for MNI-ICBM152 template.
  *Sci Data* 2020;7:237.
* Fonov V, Evans AC, Botteron K, et al. Unbiased average age-appropriate
  atlases for pediatric studies. *NeuroImage* 2011;54:313–327.

## License

MIT for the code. The bundled atlas files are covered by the MNI license in
[`src/sugar_pet/data/LICENSE-MNI`](src/sugar_pet/data/LICENSE-MNI).
For research use only; not a medical device.
