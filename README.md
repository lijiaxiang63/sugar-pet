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
| `whole-brain`    | Mean uptake of the whole brain (gray + white matter)                       |       no       |
| `cerebellum`     | Mean uptake of the whole cerebellum (RRC in the paper)                     |       no       |
| `cerebellar-gm`  | Mean uptake of cerebellar gray matter                                      |       no       |
| `pons`           | Mean uptake of the central pons                                            |       no       |
| `white-matter`   | Mean uptake of deep cerebral white matter                                  |       no       |
| `occipital`      | Mean uptake of the occipital cortex                                        |       no       |
| `sensorimotor`   | Mean uptake of primary sensorimotor cortex                                 |       no       |
| `region`         | Mean uptake of any region you choose (`--mask` or `--labels`, see below)   |       no       |
| `ps`             | **Proportional scaling**: mean uptake of the gray-matter mask              |       no       |
| `spm-global`     | SPM's global mean (all voxels above 1/8 of the image mean)                 |       no       |
| `hn`             | **Histogram normalisation**: divide the image by the mean of the controls, take the *most frequent* ratio | yes |
| `ips`            | **Iterative PS**: do PS, find hypometabolic voxels, redo PS without them   |      yes       |
| `ihn`            | **Iterative HN**: do HN, find hypometabolic voxels, redo HN without them   |      yes       |

Several methods at once: `-m cerebellum,ps,hn`, or `-m all`. Add
`--factors-only` to get just the table.

`ps` and `whole-brain` are plain region means like the others; only the region
differs. `ps` uses gray matter (as in the paper), `whole-brain` gray and white
matter together.

Options for the reference regions:

| Option               | Meaning                                                                   |
| -------------------- | ------------------------------------------------------------------------- |
| `--mask NAME_OR_FILE` | Use another region: a [built-in mask](#built-in-masks) or your own NIfTI file in MNI space |
| `--labels 34,85`     | Build the region from atlas labels (`sugar-pet labels` lists them)        |
| `--statistic median` | Median of the region instead of its mean (also for `ps`)                  |
| `--target 50`        | Scale the output so that the reference is 50 instead of 1 (any method)    |

For example, the paper's two other reference regions are built-in masks:
`-m region --mask brainstem` (RRBS) and `-m region --mask temporal-left`
(RRTL; pick the side *opposite* the lesion).

### Which one should I use?

Following the paper's conclusions:

* **`hn` is a good default.** It needs no assumption about which regions are
  affected and was the best data-driven method. `ihn` gives almost the same
  factor (see [notes](#how-it-works-and-where-it-differs-from-the-paper)).
* **`ps` is biased when hypometabolism is large.** The lesion lowers the global
  mean, the factor comes out too small, and healthy tissue then looks
  *hyper*metabolic. `ips` removes most of that bias.
* **A reference region is fine only if it is large and truly spared by the
  disease.** Small regions such as the pons are noisy.

Which regions are spared depends on the disease. Some examples:

| Disease                          | Avoid as reference                    | Usually spared            |
| -------------------------------- | ------------------------------------- | ------------------------- |
| Alzheimer's disease              | —                                     | cerebellum, pons, sensorimotor |
| Multiple system atrophy          | cerebellum, pons                      | occipital                 |
| Progressive supranuclear palsy   | pons (midbrain atrophy nearby)        | occipital, cerebellum     |
| Corticobasal syndrome            | sensorimotor                          | occipital, cerebellum     |
| Dementia with Lewy bodies, PCA   | occipital                             | cerebellum, sensorimotor  |

These are rules of thumb; check the literature for your disease and stage.

`hn`, `ips` and `ps` assume that *most* of the gray matter is normal. If a
disease lowers uptake almost everywhere they cannot see it; a white-matter
reference or a [reference cluster](#reference-cluster) is a useful
sensitivity check in that case.

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
| `stats-<method>_mean.nii.gz`, `..._std.nii.gz` | Only with `--stats-for`: mean and SD of the controls after another method (for [z-score maps](#z-score-maps)) |
| `model-<method>_beta.nii.gz`, `..._sd.nii.gz` | Only with `--covariates`: how the controls depend on age and sex (for [W-scores](#adjusting-for-age-and-sex-w-scores)) |

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

## Z-score maps

A z-score map shows how far each voxel of a normalised image is from the
controls: `z = (image − control mean) / control SD`. Negative means lower
uptake than the controls.

```bash
sugar-pet zmap patients/*.nii.gz -m hn -r hc_reference/ -o zmaps/
```

writes `sub-01_desc-hn_zmap.nii.gz`. The image is smoothed like the controls
were before it is compared. `ps`, `ips`, `hn` and `ihn` work with any
reference. For a reference region, the controls must have been scaled with the
same region, so name it when building the reference:

```bash
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ --stats-for cerebellum,pons
sugar-pet zmap patients/*.nii.gz -m pons -r hc_reference/ -o zmaps/
```

(`--stats-for all` stores every reference region.)

### Adjusting for age and sex (W-scores)

Uptake changes with age and differs a little between the sexes. If your
patients are older than your controls, a plain z-score mixes that up with the
disease. A W-score compares each patient with what the controls predict *for a
person of the same age and sex*:
`w = (image − predicted) / residual SD of the controls`.

Give `build-reference` a table with the age and sex of the controls, and
`zmap` a table with those of the patients:

```bash
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ --covariates controls.tsv
sugar-pet zmap patients/*.nii.gz -m hn -r hc_reference/ -o wmaps/ --covariates patients.tsv
```

writes `sub-01_desc-hn_wmap.nii.gz`. Without `--covariates`, `zmap` still
writes plain z-scores from the same reference.

The table is a TSV or CSV file such as a BIDS `participants.tsv`:

```
participant_id	age	sex
sub-01	63	F
sub-02	58	M
```

Rows are matched to images by `participant_id` (`sub-01` matches
`sub-01_..._pet.nii.gz`), or by a column called `image` holding file names.
Only the columns you adjust for are read. `--adjust-for age,sex` is the
default; any column works if it is numeric or has exactly two values. Age
enters as a straight line, and you get a warning when a patient is older or
younger than every control.

## Reference cluster

Instead of choosing a reference region from an atlas, you can let a patient
group show which voxels its disease spares
([Yakushev et al., 2009](https://doi.org/10.1016/j.neuroimage.2008.07.015)).
After proportional scaling, spared regions look *hyper*metabolic compared with
controls, because the lesions pulled the global mean down. Those voxels become
the reference region:

```bash
sugar-pet reference-cluster patients/*.nii.gz -r hc_reference/ -o cluster.nii.gz
sugar-pet normalize patients/*.nii.gz -m region --mask cluster.nii.gz -o out/
```

`--p-threshold` (default 0.001, uncorrected) and `--min-cluster` (default
1 mL) control which voxels are kept.

The cluster depends on the group it came from. Comparing that same group with
controls afterwards is circular: derive the cluster from other patients, or
cross-validate.

## Python

```python
from sugar_pet import Reference, normalize, zmap

ref = Reference.build(control_files, fwhm=8)  # or Reference.load("hc_reference")
ref.save("hc_reference")

res = normalize("sub-01_pet.nii.gz", method="ihn", reference=ref)
print(res.factor)  # the normalisation factor
print(res.info)  # voxels used, fraction excluded, ...
res.save("sub-01_desc-ihn_pet.nii.gz")
res.mask_image()  # the voxels the factor was estimated from

z = zmap("sub-01_pet.nii.gz", method="ihn", reference=ref)  # a NIfTI image

# W-scores: build the reference with the controls' covariates, then pass the patient's
ref = Reference.build(control_files, covariates={"age": ages, "sex": sexes})
w = zmap("sub-01_pet.nii.gz", method="ihn", reference=ref, covariates={"age": 63, "sex": "F"})
```

A reference region can be a built-in mask, your own file, or any set of labels
of the bundled atlas:

```python
from sugar_pet import atlas_labels, labels_mask, reference_cluster

res = normalize(img, "region", mask="brainstem", statistic="median", target=100)

atlas_labels()  # {21: 'Putamen (R)', 72: 'Putamen (L)', ...}
res = normalize(img, "region", mask=labels_mask([40, 91]))  # both thalami

cluster = reference_cluster(patient_files, ref)
res = normalize(img, "region", mask=cluster)
```

## Built-in masks

`gray-matter`, `parenchyma`, `cerebellum`, `cerebellar-gm`, `pons`,
`white-matter`, `occipital`, `sensorimotor`, `brainstem`, `temporal-left` and
`temporal-right`. `sugar-pet masks -o masks/` writes them as NIfTI files so
you can look at them, and `sugar-pet labels` lists the atlas labels for
`--labels`.

They come from two freely redistributable MNI resources, resampled to 2 mm:
the ICBM 2009c tissue probability maps and the
[CerebrA atlas](https://doi.org/10.1038/s41597-020-0557-9). Masks are resampled
to the grid of each image automatically.

The pons mask is the part of the CerebrA brainstem between z = −44 and −26 mm,
eroded in-plane; treat it as a pragmatic central-pons region, not an anatomical
gold standard. `occipital` is the whole occipital cortex; for the lateral
occipital cortex alone use `--labels 34,85`. For other MNI variants or atlases,
pass your own mask.

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
* **Reference cluster.** The two-sample t-test (patients > controls, equal
  variances) is computed directly from the patient images and the mean and SD
  stored in the reference, so the control images are not needed again.
* **W-scores.** An ordinary least-squares regression of the normalised
  controls on the covariates is fitted at every voxel. As in
  [La Joie et al., 2012](https://doi.org/10.1523/JNEUROSCI.2170-12.2012), the
  residual is divided by the residual SD of the controls, without the extra
  term for the uncertainty of the prediction.
* **Left out on purpose.** Standardising images to zero mean and unit variance
  (common before deep learning) is a shift as well as a scaling, so uptake
  ratios are lost. The log transform and double centring of SSM/PCA covariance
  analysis belong to that analysis, which gives the same result whatever
  scaling was applied before.

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
* Yakushev I, Hammers A, Fellgiebel A, et al. SPM-based count normalization
  provides excellent discrimination of mild Alzheimer's disease and amnestic
  mild cognitive impairment from healthy aging. *NeuroImage* 2009;44:43–50.
* La Joie R, Perrotin A, Barré L, et al. Region-specific hierarchy between
  atrophy, hypometabolism, and β-amyloid (Aβ) load in Alzheimer's disease
  dementia. *J Neurosci* 2012;32:16265–16273.
* Manera AL, Dadar M, Fonov V, Collins DL. CerebrA, registration and manual
  label correction of Mindboggle-101 atlas for MNI-ICBM152 template.
  *Sci Data* 2020;7:237.
* Fonov V, Evans AC, Botteron K, et al. Unbiased average age-appropriate
  atlases for pediatric studies. *NeuroImage* 2011;54:313–327.

## License

MIT for the code. The bundled atlas files are covered by the MNI license in
[`src/sugar_pet/data/LICENSE-MNI`](src/sugar_pet/data/LICENSE-MNI).
For research use only; not a medical device.
