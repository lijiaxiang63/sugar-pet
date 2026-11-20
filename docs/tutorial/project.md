# Make a project

**Pages: the opening page, then Data**

SUGAR-PET works on one **project** at a time. A project is a folder: the
images, everything the analyses write, and a file,
`sugar-pet-project.json`, that records the images, what you chose on every
page, the options of the analyses, and what each step last did. The file is
written as you work: there is nothing to save, and a project opened again is
where you left it, results included.

## Make or open a project

With no project open, the app shows its opening page.

<Shot name="welcome" alt="The opening page" :marks="[[1, 353, 321], [2, 508, 321], [3, 268, 412]]" />

1. **New Project…** makes a project (below).
2. **Open Project…** opens one: choose its folder. A project folder dropped
   on the window is opened too.
3. **Recent projects**: click one to open it. One that cannot be reached
   now, on a drive that is not plugged in for instance, says so; **×**
   takes a project off the list and leaves its folder alone.

A study folder of an earlier version of the app, made with
`sugar-pet init` or the old Study Folder page, is listed here as well, and
becomes a project when you open it: its images in `patients/` and
`controls/` and a filled-in `participants.tsv` are taken in.

<Shot name="new-project" alt="The form of a new project" :marks="[[4, 282, 323], [5, 282, 395], [6, 282, 451], [7, 715, 598]]" />

4. **Name** of the project, also the name of its folder.
5. **Location:** the folder the project folder is created in.
6. **Images from outside the project:** *Keep in Place* only notes where an
   image is, so nothing is duplicated but the image must stay there. *Copy
   into Project* copies it into the project folder, which then holds
   everything and can be moved or shared as a whole. You can change this
   later on the Data page; it applies to images added afterwards.
7. **Create Project** makes the folder, with the folders listed
   [below](#what-the-project-folder-holds) and a `participants.tsv` that
   holds only its header line, and opens the project on its Data page.

## The Data page

The Data page is the first of a project and the only one where images are
added: every other page takes them from here.

<Shot name="data" alt="The Data page" :marks="[[8, 147], [9, 191], [10, 372], [11, 444], [12, 544], [13, 649, 544], [14, 'run']]" />

8. **Name:** the name the project goes by in the app. It need not be that of
   its folder.
9. **Project folder:** where it lies. **Show in Finder** opens it, and
   **Close Project** goes back to the opening page.
10. **The images are in:** *MNI Space* if they were warped already (by SPM,
    ANTs or any other tool), or *Native Space* if they come straight from the
    scanner. Native images go through [Spatial Normalisation](./spatial)
    before any other page reads them; that needs MATLAB and SPM. Ada's are in
    native space.
11. **Images from outside the project:** as when the project was made.
12. **Patients:** one FDG-PET scan per patient. Drag files or whole folders
    onto the list, or use **Add Files…** and **Add Folder…**. When the
    project's own `patients/` folder holds images, **Load patients/** adds
    them in one click.
13. **Healthy controls (at least 3):** in the same way. Every patient is
    compared with these controls, and no others: the maps, the
    classification and the group comparison all rest on them. Below 3
    controls nothing can run, and below about 10 the voxel statistics get
    unreliable (see [Prepare your data](../guide/prepare-data#the-healthy-controls)).
    An image can be in one group only.
14. The bar at the bottom says what the project holds, or what it still
    lacks; **Next** leads to the next page.

Lists of more than 8 images get a filter box; **×** on a row removes an
image, and **Clear** empties the list. Removing an image from a list never
deletes its file.

## Age and sex

Scroll down.

<Shot name="data-covariates" alt="The covariate table of the project" :marks="[[15, 679], [16, 734]]" />

15. **Covariate table** (optional, recommended): a TSV or CSV file with the
    age and sex of every image of the project, patients and controls, such
    as the project's own `participants.tsv` once it is filled in (**Use
    participants.tsv** chooses it). Rows are matched to the images by an
    `image` or `participant_id` column; see
    [Prepare your data](../guide/prepare-data#the-table-of-ages-and-sexes).
16. **Adjust for:** the columns used, *age* and *sex* by default. With the
    table, the control statistics model what age and sex do to the uptake
    (W-scores, [step 2](./zmap#z-scores-or-w-scores)) and the group
    comparison adjusts for them.

The app reads the table again when it changes on disk, so you can fill it
in with another program while the project is open.

## What the project folder holds

| Folder or file | Written by |
| --- | --- |
| `sugar-pet-project.json` | The app, as you work |
| `raw/` | Nobody: a place for native-space images, if you want one |
| `patients/`, `controls/` | You, or [Spatial Normalisation](./spatial), which writes the warped images there |
| `participants.tsv` | You |
| `hc_reference/` | [Intensity Normalisation](./normalize): the control statistics |
| `normalised/` | [Intensity Normalisation](./normalize) |
| `zmaps/` | [Z-score Maps](./zmap) |
| `reports/` | [Classify Parkinsonism](./classify) |
| `quick/` | [Quick Classification](./quick), a folder for each patient |
| `compare/` | [Compare Groups](./compare) |
| `reference_cluster.nii.gz` | [Reference Cluster](./reference-cluster) |

What lies in the project folder is recorded by where it is in it, so a
project that holds its images can be moved, or opened on another computer.

::: details The same from the command line
The command line has no project file, but lays out the same folders:

```bash
sugar-pet init study/
```
:::

## Next

[Warp to MNI space](./spatial) if the images are in native space, otherwise
[Normalise the intensity](./normalize).
