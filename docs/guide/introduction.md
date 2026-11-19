# What SUGAR-PET does

SUGAR-PET is a desktop app for **intensity normalisation** of brain FDG-PET
images, along with the downstream analyses that build on it: single-subject
deviation maps against healthy controls, parkinsonism pattern classification,
and group-level comparisons.

## Why intensity normalisation?

Raw FDG-PET voxel values depend on injected dose, blood glucose, body weight,
and other scan-specific factors, so two scans cannot be compared directly.
Intensity normalisation addresses this by dividing each image by a single
number, the **normalisation factor**:

```
normalised image = image / factor
```

The methods in SUGAR-PET simply differ in how they estimate that factor: taking
the mean uptake of a spared reference region, the mean of the whole gray
matter, or the mode of the voxel-wise ratio against a healthy control template.
See [Choosing a method](../reference/methods) for guidance on which to pick.

## Two different "normalisations"

In neuroimaging, the same word is used for two completely separate steps. The
app keeps them on distinct pages:

| | Spatial normalisation | Intensity normalisation |
| --- | --- | --- |
| What it does | Warps an image from native scanner space into MNI space so each voxel corresponds to the same anatomical location across subjects | Divides an image by its factor so intensity values are comparable across scans |
| App page | **Spatial Normalisation** | **Intensity Normalisation** and all subsequent pages |
| Requirements | MATLAB and SPM12 or SPM25 | None |
| When to run | Only if your images are not yet in MNI space | Every analysis |

::: tip You may not need MATLAB at all
If your images are already in MNI space (from SPM, ANTs, or any other tool),
skip the first page. Nothing else in the app requires MATLAB, SPM, or Python.
:::

## Overview of the app

The sidebar lists the pages in the order of a typical analysis:

| Page | Purpose | Needs a reference? |
| --- | --- | :---: |
| [Spatial Normalisation](../tutorial/spatial) | Warps native-space PET images into MNI space using SPM | no |
| [Reference](../tutorial/build-reference) | Summarises healthy controls into a single folder (the *reference*) | builds it |
| [Intensity Normalisation](../tutorial/normalize) | Divides each image by its factor and outputs `factors.tsv` | for HN, iPS, iHN |
| [Z-score Maps](../tutorial/zmap) | Maps how far each voxel deviates from healthy controls | yes |
| [Classify Parkinsonism](../tutorial/classify) | Matches a patient's map against typical metabolic patterns and writes a report | yes |
| [Compare Groups](../tutorial/compare) | Voxel-wise two-sample t-test between two groups with multiple-comparison correction | for HN, iPS, iHN |
| [Reference Cluster](../tutorial/reference-cluster) | Derives a data-driven reference region from a patient group | yes |
| [Masks & Atlas](../tutorial/masks) | Inspects and exports the built-in regions and atlas labels | no |

The **reference** is central to the workflow: it is a compact folder
summarising your healthy controls, built once and reused for every patient
afterwards. Because most pages rely on it, the tutorial starts by building
one early.

## Where to go next

1. [Install the app](./install).
2. Take [a tour of the interface](./interface) to learn the shared controls.
3. [Prepare your data](./prepare-data) (image formats and covariate tables).
4. Follow the [step-by-step tutorial](../tutorial/).

::: warning For research use only
SUGAR-PET is not a medical device. The parkinsonism classification is a
research decision aid, not a clinical diagnosis.
:::
