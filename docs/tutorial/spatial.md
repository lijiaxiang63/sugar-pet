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

## Set up SPM and MATLAB (once)

SPM and MATLAB are set up in the settings, not on the page, and the app
remembers them. On the Spatial Normalisation page, click **Set Up…** in the
**SPM and MATLAB** row (it reads **Change…** once they are set), or open
**Settings › Spatial Normalisation** yourself.

<Shot name="settings-spatial" alt="Settings › Spatial Normalisation" />

* **SPM folder:** click **Choose…** and select the directory containing
  `spm.m`. The app tells you if the folder is not SPM12 or SPM25.
* **MATLAB:** if MATLAB was detected, the line under the name shows where it
  was found and there is nothing to do. Otherwise click **Choose…** and
  select the MATLAB program yourself, as in the screenshot: the MATLAB app
  on macOS, `bin/matlab` on Windows and Linux.
* **PET template** (rarely needed): SPM's built-in PET template by default.
  The mean of your own warped controls often improves registration accuracy.
* **MATLAB processes** (rarely needed): splits the images across several
  MATLAB workers running in parallel. Higher values run faster but use more
  memory.

You can close the Settings window or leave it open; the changes are saved as
you make them.

## Fill in the page

<Shot name="spatial" alt="The Spatial Normalisation page, filled in" :marks="[[1, 177], [2, 494], [3, 550], [4, 609], [5, 'run']]" />

1. **Add the native-space images.** Drag and drop them onto the list, or use
   **Add Files…** / **Add Folder…**. Ada adds the six scans from `study/raw/`.
   4D dynamic images are automatically averaged over time.
2. **Check SPM and MATLAB.** A green tick and the two paths mean both are
   set up. If the row shows **Set Up…** instead, do the
   [set-up above](#set-up-spm-and-matlab-once) first.
3. **Choose the output folder.** Ada selects `study/patients/`. Warped images
   are written at 2 mm isotropic resolution and named like
   `sub-pt01_space-MNI_pet.nii.gz`.
4. **Options in effect.** The line names the template and the number of
   MATLAB processes; **Options…** opens **Settings › Spatial Normalisation**
   to change them.
5. Click **Warp to MNI Space**. The status bar tracks how many images have
   finished.

## Check the results

<Shot name="spatial-results" alt="The results of the spatial normalisation" :marks="[[6, 135], [7, 584], [8, 784]]" />

6. **Look at every warped image before continuing.** The picture shows axial
   slices of one warped image with the outline of the template's brain drawn
   over it: the brain should fill the outline. PET-only registration can fail
   silently—a tilted, shrunken, or cropped brain will compromise every
   downstream step. The arrows above the picture step through the images.
7. **The table** lists each input image and its output file. Click a row to
   draw that image. A row whose output reads **Failed** in red did not work; the
   reason appears below the table.
8. Hand the images on: **Use for Intensity Normalisation** for patients, or
   **Use to Build a Reference** for controls, jumps to that page with these
   images pre-loaded. **Show in Finder** shows the files.

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
