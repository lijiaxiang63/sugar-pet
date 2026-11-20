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
| When to run | Only for images in native space, before step 1 | Every analysis: step 1 |

::: tip You may not need MATLAB at all
If your images are already in MNI space (from SPM, ANTs, or any other tool),
say so on the Data page and the Spatial Normalisation page drops out. Nothing
else in the app requires MATLAB, SPM, or Python.
:::

## Overview of the app

The app works on one **project** at a time: a folder that holds the patients
and the healthy controls of a study, and everything the analyses write. The
sidebar lists its pages in the order of an analysis:

| Page | Purpose |
| --- | --- |
| [Data](../tutorial/project) | The images of the project: patients, at least 3 healthy controls, and their ages and sexes |
| [Spatial Normalisation](../tutorial/spatial) | Warps native-space images into MNI space using SPM (only for images in native space) |
| [Intensity Normalisation](../tutorial/normalize) | Step 1: summarises the controls into the *control statistics*, then divides each image by its factor |
| [Z-score Maps](../tutorial/zmap) | Step 2: maps how far each voxel of a patient deviates from the controls |
| [Classify Parkinsonism](../tutorial/classify) | Step 3: matches each patient's map against typical metabolic patterns and writes a report |
| [Compare Groups](../tutorial/compare) | Voxel-wise two-sample t-test of patients against controls, with multiple-comparison correction |
| [Reference Cluster](../tutorial/reference-cluster) | Derives a data-driven reference region from the patients |
| [Quick Classification](../tutorial/quick) | Classifies one more patient, outside the project, against its controls |
| [Masks & Atlas](../tutorial/masks) | Inspects and exports the built-in regions and atlas labels |

The **healthy controls** are central: every patient is compared with the
controls of the same project, and with no others. Step 1 summarises them into
the *control statistics*, which the maps, the classification and the group
comparison all read, so they all rest on one set of controls.

## Where to go next

1. [Install the app](./install).
2. Take [a tour of the interface](./interface) to learn the shared controls.
3. [Prepare your data](./prepare-data) (image formats and covariate tables).
4. Follow the [step-by-step tutorial](../tutorial/).

::: warning For research use only
SUGAR-PET is not a medical device. The parkinsonism classification is a
research decision aid, not a clinical diagnosis.
:::
