# Prepare your data

Spending ten minutes organising your data here can save an afternoon of
troubleshooting later. The app expects three things: MNI-space images, a
healthy control group, and (optionally) a table of ages and sexes.

## The images

* **Format:** NIfTI (`.nii` or `.nii.gz`), one static FDG-PET scan per subject.
* **Space:** MNI space for every page except Spatial Normalisation. Any voxel
  size and bounding box are supported. If your images are still in native
  scanner space, [Step 1](../tutorial/spatial) of the tutorial warps them.
* **Consistent grid within a group:** all control images in a reference must
  share the same voxel size and bounding box—which happens automatically if
  they went through the same spatial normalisation pipeline.
* **Smoothing:** do not pre-smooth your images. The app applies smoothing
  internally where a method needs it (8 mm by default), while the normalised
  images it writes remain unsmoothed. If your images are already smoothed, set
  the smoothing kernel to `0` wherever prompted.

::: warning Inspect every warped image
Spatial normalisation can fail silently without raising an error. Open each
warped image in a viewer (MRIcroGL, FSLeyes, ITK-SNAP, or SPM's Check Reg)
before using it.
:::

## The healthy controls

Histogram normalisation, Z-score maps, classification, and reference clusters
all compare patients against healthy controls. Controls should come from the
**same scanner**, use the **same reconstruction parameters**, and undergo the
same preprocessing as the patients.

| Number of controls | Recommended use |
| --- | --- |
| Fewer than 3 | Cannot build a reference (the app will refuse to run) |
| 3 to 9 | Sufficient for HN, iPS, and iHN factors; voxel-wise statistics are unreliable |
| 20 or more | Recommended for Z-score maps and group comparisons |
| 30 or more | Recommended for single-patient classification |

## The table of ages and sexes

Brain FDG uptake varies with age and differs slightly by sex. Providing a
covariate table allows the app to adjust for these effects (see
[W-scores](../tutorial/zmap#z-scores-or-w-scores)). Use a TSV or CSV file, such
as a BIDS `participants.tsv`:

```
participant_id	age	sex
sub-hc01	63	F
sub-hc02	58	M
sub-pt01	71	M
```

* Rows are matched to images by `participant_id` (`sub-pt01` matches
  `sub-pt01_space-MNI_pet.nii.gz`), or via an `image` column containing
  filenames directly.
* Only the columns you tick in the app are read. Each selected column must be
  numeric (e.g. `age`) or binary categorical (e.g. `F` and `M`).
* A single table can include both patients and controls, as shown above.

## File naming

Output files are named after the input files following BIDS conventions:

| Input | Output |
| --- | --- |
| `sub-pt01_space-MNI_pet.nii.gz` | `sub-pt01_space-MNI_desc-hn_pet.nii.gz` (normalised with HN) |
| | `sub-pt01_space-MNI_desc-hn_zmap.nii.gz` (Z-score map) |
| | `sub-pt01_space-MNI_report.html` (classification report) |

Make sure every input image has a unique filename. Two images with the same
name in different folders (`a/pet.nii.gz` and `b/pet.nii.gz`) would map to
the same output path: Spatial Normalisation and Classify Parkinsonism refuse
to run before writing anything, while other pages will overwrite the first
output with the second.

## A folder layout that works {#a-folder-layout-that-works}

The tutorial uses the layout below. It is not mandatory, but it keeps inputs
and outputs cleanly separated:

```
study/
├── raw/                native-space images        (only if not in MNI space yet)
├── patients/           sub-pt01_space-MNI_pet.nii.gz, ...
├── controls/           sub-hc01_space-MNI_pet.nii.gz, ...
├── participants.tsv    age and sex of all subjects
│
├── hc_reference/       created in Step 2
├── normalised/         created in Step 3
├── zmaps/              created in Step 4
├── reports/            created in Step 5
└── compare/            created in Step 6
```

## Next

Start the [tutorial](../tutorial/).
