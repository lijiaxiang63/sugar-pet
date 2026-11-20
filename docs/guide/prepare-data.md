# Prepare your data

Spending ten minutes organising your data here can save an afternoon of
troubleshooting later. A project needs three things: the patients' images, at
least 3 healthy controls, and (optionally, but recommended) a table of ages
and sexes.

## The images

* **Format:** NIfTI (`.nii` or `.nii.gz`), one static FDG-PET scan per subject.
* **Space:** MNI space, or native scanner space: say which on the Data page.
  Images in native space are warped by
  [Spatial Normalisation](../tutorial/spatial) before any analysis reads
  them. Any voxel size and bounding box are supported.
* **One grid:** all images of a project must share the same voxel size and
  bounding box—which happens automatically if they went through the same
  spatial normalisation pipeline.
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

Every project has its own healthy controls, and every patient is compared
with them and no others: histogram normalisation, the Z-score maps, the
classification, the group comparison and the reference cluster all rest on
the same controls. Statistics built from controls elsewhere cannot be
imported; add those controls' images to the project instead. Controls should come from the
**same scanner**, use the **same reconstruction parameters**, and undergo the
same preprocessing as the patients.

| Number of controls | Recommended use |
| --- | --- |
| Fewer than 3 | Not enough: the app runs nothing |
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

## The project folder

You need not lay out any folders yourself: a
[project](../tutorial/project) is a folder the app makes, with a place for
everything. Images can stay where they are (the project notes where), or be
copied into it, so that it holds everything and can be moved or shared as a
whole.

```
my-study/
├── sugar-pet-project.json   what the app records as you work
├── raw/                     native-space images, if you want a place for them
├── patients/                patient images in MNI space
├── controls/                control images in MNI space
├── participants.tsv         age and sex of all subjects
│
├── hc_reference/            the control statistics          (step 1)
├── normalised/              normalised images, factors.tsv  (step 1)
├── zmaps/                   Z- or W-score maps               (step 2)
├── reports/                 classification reports          (step 3)
├── quick/                   Quick Classification
├── compare/                 Compare Groups
└── reference_cluster.nii.gz Reference Cluster
```

Images in native space that are warped in the project go into `patients/`
and `controls/`. A study folder made with `sugar-pet init`, which lays out
the same folders, opens as a project: its images and a filled-in
`participants.tsv` are taken in.

## Next

[Make a project](../tutorial/project), the first part of the [tutorial](../tutorial/).
