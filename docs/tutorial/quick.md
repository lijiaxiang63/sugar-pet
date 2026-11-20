# Quick classification

**Page: Quick Classification** · under *Outside the Project* · needs step 1

A new patient comes to the clinic after the project's analysis is done. This
page classifies that one scan against the healthy controls of the project,
the way the project's own patients were classified, **without adding it to
the project**: it is not one of the patients, no step of the project changes,
and the project file records nothing of it.

The scan goes through the same steps as the patients:

1. warped into MNI space with SPM and the settings of
   [Spatial Normalisation](./spatial), unless it is in MNI space already;
2. intensity-normalised with the **primary method** of [step 1](./normalize)
   and the settings of the project;
3. mapped against the project's **control statistics**, as in
   [step 2](./zmap): W-scores when you give its age and sex;
4. classified with the settings of [Classify Parkinsonism](./classify).

Classifying one of the project's own patients here comes to exactly what step
3 found for it.

::: warning Decision support, not a diagnosis
The same caveats hold as for [Classify Parkinsonism](./classify): read the
result with the clinical findings, MRI and dopamine-transporter imaging.
:::

## Before you start

* Step 1 of the project is done, with the settings as they are now: the page
  takes its method, settings and control statistics. When the control
  statistics are to be summarised again (after the covariates changed, say),
  the page says which step to run first.
* For a scan in native space, SPM and MATLAB are
  [set up](./spatial#set-up-spm-and-matlab-once).

## Run it

<Shot name="quick" alt="The Quick Classification page" :marks="[[1, 178], [2, 234], [3, 307], [4, 379], [5, 545], [6, 586], [7, 627], [8, 'run']]" />

1. **PET image:** the patient's scan, a NIfTI file. Ada chooses
   `sub-new01_pet.nii.gz` from her clinic's folder.
2. **The image is in:** *Native Space* (the default) warps it first; *MNI
   Space* takes it as it is, and it must then be on the grid of the
   project's images.
3. **age** and 4. **sex:** the covariates the control statistics model, shown
   only when the project has a covariate table. Give all of them for a
   W-score map, or none for a plain Z-score map.
5. **Method:** the primary method of step 1, and the normalisations of the
   cross-check when it is on. To change it, **Open Step 1**.
6. **Healthy controls:** how many the patient is compared with.
7. **Written to:** `quick/<name of the image>` in the project folder, which
   is not part of the project.
8. Click **Classify**. The bar shows the warp, then the classification.

## Check the result

<Shot name="quick-results" alt="The result of the quick classification" :marks="[[9, 288], [10, 940, 306], [11, 381], [12, 790]]" />

9. **The conclusion**, read as on
   [Classify Parkinsonism](./classify#reading-the-conclusion), with its
   confidence and the bars of the six patterns.
10. **Open Report** shows the report in the browser.
11. **Lowest W** and **Brain voxels with W < −2**, as in the table of
    [step 2](./zmap#check-the-results), and the method.
12. **The map** on axial slices of the warped scan: blue to cyan where uptake
    is lower than in the controls, red to yellow where it is higher.

Scroll down.

<Shot name="quick-warp" alt="The warp of the scan, to check" :marks="[[13, 350], [14, 742], [15, 268, 782]]" />

13. For a scan that was warped here, **look at the warp before trusting the
    map**: a warp from PET alone can go wrong without an error.
14. **The warped scan** with the outline of the template's brain over it: the
    brain should fill the outline.
15. **Show in Finder** opens the scan's folder.

Choosing another image, or changing the space or the covariates, clears the
result; the files of a run stay in their folder. A scan classified again
under the same name writes over its folder.

## Output files

In `quick/sub-new01/`:

| File | What it holds |
| --- | --- |
| `sub-new01_space-MNI_pet.nii.gz` | The scan warped into MNI space (native scans only) |
| `sub-new01_space-MNI_desc-hn_pet.nii.gz` | The scan normalised with the primary method |
| `sub-new01_space-MNI_desc-hn_wmap.nii.gz` | The W-score map (`_zmap` without covariates) |
| `sub-new01_report.html` | The report |
| `sub-new01_classification.json` | Every number of the report |

::: details The same from the command line
```bash
sugar-pet spatial-norm sub-new01_pet.nii.gz -o quick/ --spm-path /path/to/spm12
sugar-pet classify quick/sub-new01_space-MNI_pet.nii.gz -r hc_reference/ -o quick/ --lang en
```
:::

## Next

[Compare two groups](./compare).
