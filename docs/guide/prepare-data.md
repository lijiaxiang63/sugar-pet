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
├── hc_reference/       written in Step 2
├── normalised/         written in Step 3
├── zmaps/              written in Step 4
├── reports/            written in Step 5
└── compare/            written in Step 6
```

### Create it in one go {#create-it}

You do not have to make these folders by hand. Open the **Study Folder**
page, the first in the sidebar, choose where the study folder goes and what
it is called, and click **Create Folders**:

<Shot name="study" alt="The Study Folder page, after the folders were created" />

The page lists every folder with what it is for, and marks each as *Created*
or *Already there*. It also makes the study folder the one the app writes
into: the output folder of every page is set to its folder of the study, so
you do not have to choose them one by one. The app remembers the study folder
for the next launch. Only Spatial Normalisation is left for you to set, since
it writes patients and controls into different folders.

A reference the study already holds in `hc_reference/` becomes the reference
in use, if none is. When you create the folders of another study, the pages
go over to it: the images, tables and reference of the study you leave are
taken off them, and so are the results of its runs, so that the images of one
study are never written into another or compared with its controls. The files
of those results stay where they are, in the folder of the study you left.
Images and a reference you chose from elsewhere are kept.

From the command line, the folders are created with:

```bash
sugar-pet init study/
```

Both create the folders above and a `participants.tsv` that holds only the
header line `participant_id`, `age`, `sex`, for you to fill in. Nothing that
already exists is changed, so it is safe to run on a study folder you have
started: only what is missing is added.

### What each folder is for {#what-each-folder-is-for}

You fill the first four before running the analyses:

| Folder or file | What goes in it |
| --- | --- |
| `raw/` | Images still in native scanner space. Only needed if yours are not in MNI space yet: [Step 1](../tutorial/spatial) warps them into `patients/` and `controls/`. |
| `patients/` | Patient images in MNI space, one scan per subject. |
| `controls/` | Healthy-control images in MNI space, from the same scanner and preprocessing as the patients. |
| `participants.tsv` | Age and sex of every subject, patients and controls, one row each (see [above](#the-table-of-ages-and-sexes)). |

The analyses write into the others. The app sets each as the output folder
of its page when it creates the study folder; on the command line, pass it
to `-o`:

| Folder | Written by | What it holds |
| --- | --- | --- |
| `hc_reference/` | [Reference](../tutorial/build-reference) (Step 2) | The reference built from the controls. Most other steps read it. |
| `normalised/` | [Intensity Normalisation](../tutorial/normalize) (Step 3) | Intensity-normalised images and `factors.tsv`. |
| `zmaps/` | [Z-score Maps](../tutorial/zmap) (Step 4) | Z-score or W-score maps of the patients. |
| `reports/` | [Classify Parkinsonism](../tutorial/classify) (Step 5) | Classification reports of the patients. |
| `compare/` | [Compare Groups](../tutorial/compare) (Step 6) | T-maps and cluster tables of patients against controls. |

## Next

Start the [tutorial](../tutorial/).
