# Warp to MNI space

**Page: Spatial Normalisation** · only for images in native space · needs MATLAB and SPM

This page warps the images of the project from native scanner space into MNI
space. It is there only when the Data page says the images are in *Native
Space*; for images in MNI space already (whatever tool warped them), skip to
[Normalise the intensity](./normalize).

## Before you start

* **MATLAB** is installed (the app looks for it itself).
* **SPM12 or SPM25** is downloaded. You only need to know the folder holding
  `spm.m`; it does not have to be on MATLAB's path.
* The images are FDG-PET scans in native space. No structural MRI is needed.

## Set up SPM and MATLAB (once)

SPM and MATLAB are set up in the settings, not on the page, and the app
remembers them for every project. On the page, click **Set Up…** in the
**SPM and MATLAB** row (it reads **Change…** once they are set), or open
**Settings › Spatial Normalisation** yourself.

<Shot name="settings-spatial" alt="Settings › Spatial Normalisation" />

* **SPM folder:** choose the folder holding `spm.m`. The app tells you if it
  is not SPM12 or SPM25.
* **MATLAB:** when MATLAB was found, its name is there and there is nothing
  to do. Otherwise choose the MATLAB program: the MATLAB app on macOS,
  `bin/matlab` on Windows and Linux.
* **PET template** (rarely needed): SPM's own PET template unless you choose
  one. The mean of your own warped controls often fits better.
* **MATLAB processes** (rarely needed): the images are shared out among this
  many MATLAB processes running side by side. More is faster, and takes more
  memory.

## Run it

<Shot name="spatial" alt="The Spatial Normalisation page" :marks="[[1, 109], [2, 269], [3, 444], [4, 502], [5, 554], [6, 675, 861]]" />

1. **The line of steps.** For a project in native space, Spatial
   Normalisation leads the line of the individual analysis, *before step 1*:
   step 1 reads only what it warped. A tick means a step is done.
2. **Images:** the patients and controls of the [Data page](./project), and
   how many of each are warped already. To change them, **Change in Data**.
3. **SPM and MATLAB:** a green tick and the two paths mean both are set up.
   If the row shows **Set Up…** instead, do the
   [set-up above](#set-up-spm-and-matlab-once) first.
4. **Written to:** the warped images go into the `patients/` and `controls/`
   folders of the project, at 2 mm, named like
   `sub-pt01_space-MNI_pet.nii.gz`. 4D images are averaged over time first.
5. **Options in effect:** the template and the number of MATLAB processes;
   **Options…** opens the settings above.
6. Click **Warp to MNI Space**. The bar counts the images as they finish.

## Check the results

<Shot name="spatial-results" alt="The results of the warp" :marks="[[7, 126], [8, 549], [9, 268, 782], [10, 884, 861]]" />

7. **Look at every warped image before going on.** The picture shows axial
   slices of one image with the outline of the template's brain over it: the
   brain should fill the outline. A warp from PET alone can go wrong without
   an error, and a tilted, shrunken or cropped brain spoils every step after
   it. The arrows step through the images.
8. **The table** lists each image and what it became; click a row to draw
   it. A row that reads **Failed** did not work, and the notes under the
   table say why.
9. **Show in Finder** opens the project folder.
10. **Next: Intensity Normalisation** leads on. The pages after this one read
    the warped images; an image not warped yet is left out of them, and
    step 1 says how many.

## How it works

The app crops each image around the head, moves the origin to the centre of
the brain, and runs SPM's "Old Normalise" onto a PET template in MATLAB, in
the background. Patients and controls go through exactly the same warp,
which is what matters for comparing them.

::: details The same from the command line
```bash
sugar-pet spatial-norm raw/*_pet.nii.gz -o patients/ --spm-path /path/to/spm12 --jobs 4
```
:::

## Next

[Normalise the intensity](./normalize).
