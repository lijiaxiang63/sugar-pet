# Step 1: Warp to MNI space

**Page: Spatial Normalisation** · optional · needs MATLAB and SPM

This step warps images from native scanner space into MNI space. **Skip this
step if your images are already in MNI space** (regardless of which tool
warped them) and go straight to [Step 2](./build-reference).

## Before you start

* **MATLAB** is installed (the app searches for it automatically).
* **SPM12 or SPM25** is downloaded. You only need to know the folder
  containing `spm.m`; it does not need to be on MATLAB's search path.
* Your images are FDG-PET scans in native space. No structural MRI is needed.

## Fill in the page

<Shot name="spatial" alt="The Spatial Normalisation page, filled in" :marks="[[1, 181], [2, 520], [3, 571], [4, 679], [5, 748], [6, 'run']]" />

1. **Add the native-space images.** Drag and drop them onto the list, or use
   **Add Files…** / **Add Folder…**. Ada adds the six scans from `study/raw/`.
   4D dynamic images are automatically averaged over time.
2. **Choose the SPM folder** (the directory containing `spm.m`).
3. **Check MATLAB.** If detected automatically, its name is shown here.
   Otherwise, click **Choose…** and select the MATLAB executable yourself.
4. **Choose the output folder.** Ada selects `study/patients/`. Warped images
   are written at 2 mm isotropic resolution and named like
   `sub-pt01_space-MNI_pet.nii.gz`.
5. **Options** (rarely needed):
   * *PET template*: uses SPM's built-in PET template by default. Using the
     mean of your own warped controls often improves registration accuracy.
   * *MATLAB processes*: splits images across multiple parallel MATLAB
     workers. Higher values run faster but use more memory.
6. Click **Warp to MNI Space**. The status bar tracks how many images have
   finished.

## Check the results

<Shot name="spatial-results" alt="The results of the spatial normalisation" :marks="[[7, 502], [8, 741], [9, 784]]" />

7. **The table** lists each input image and its output path. A filename shown
   in red indicates a failure; the reason appears below the table.
8. **Inspect every warped image** in a viewer before continuing. PET-only
   registration can fail silently—a tilted, shrunken, or cropped brain will
   compromise every downstream step.
9. Click **Use for Intensity Normalisation** to jump to the next page with
   these images pre-loaded, or **Show in Finder** to inspect the files.

::: tip Warp your controls the same way
Process your healthy controls identically (Ada saves them to
`study/controls/`). Patients and controls must go through the exact same
spatial normalisation pipeline.
:::

## How it works under the hood

The app crops each image around the head, shifts the origin to the brain
centre, and runs SPM's "Old Normalise" with a PET template in the background.
Any other spatial normalisation pipeline (e.g. MRI-guided SPM or ANTs) works
equally well for the remaining pages.

::: details The same from the command line
```bash
sugar-pet spatial-norm raw/*_pet.nii.gz -o patients/ --spm-path /path/to/spm12 --jobs 4
```
:::

## Next

[Step 2: Build the reference](./build-reference).
