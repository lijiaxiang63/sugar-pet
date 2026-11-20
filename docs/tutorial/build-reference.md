# Step 2: Build the reference

**Page: Reference** · once per study

The *reference* is a compact folder summarising your healthy controls: their
mean image, voxel-wise spread, and (optionally) how uptake varies with age
and sex. You build it once; every subsequent patient analysis compares against
this summary without re-reading the raw control scans.

You need a reference for:

* **HN**, **iPS**, and **iHN** normalisation on any page;
* [Z-score maps](./zmap), [parkinsonism classification](./classify), and
  [reference clusters](./reference-cluster).

## Add the controls

Open the **Reference** page and scroll down to *Build a new reference*.

<Shot name="reference-controls" alt="The Reference page: the controls for a new reference" :marks="[[1, 315]]" />

1. **Add the control images.** Ada drops the `study/controls/` folder onto
   the list (16 images). All controls must be in MNI space on the same voxel
   grid. At least 3 controls are required, though voxel-wise statistics are
   unreliable with fewer than about 10.

## Choose the covariates, the folder and the options

Scroll down.

<Shot name="reference-settings" alt="The covariates, the folder and the options of a new reference" :marks="[[2, 529], [3, 717], [4, 777], [5, 'run']]" />

2. **Covariate table:** click **Choose…** and select `participants.tsv`. The
   table's columns appear as checkboxes under *Adjust for*; tick **age** and
   **sex**. This enables age- and sex-adjusted
   [W-scores](./zmap#z-scores-or-w-scores) later. Skip this if you don't have
   a covariate table.
3. **Reference folder:** choose where to save the reference. Ada creates a new
   folder, `study/hc_reference/`.
4. **Options in effect:** the mask, the smoothing, and the extra methods for
   Z-score maps. Click **Options…** to open **Settings › Reference** (below)
   and tick the reference regions you also want Z-score maps for; the line
   then reads *z-score maps also with Occipital cortex, White matter* and
   shows **1 changed**.
5. Click **Build Reference**. The app reads and processes the controls one by
   one.

### The options of a new reference {#options}

<Shot name="settings-reference" alt="Settings › Reference" />

* **Mask:** the voxels used to estimate normalisation factors. Keep **Gray
  matter**, matching the original paper. *Mask file…* at the end of the list
  takes a NIfTI file of your own.
* **Smoothing (FWHM, mm):** keep **8** mm. Set this to `0` only if your
  images are already smoothed.
* **Z-score maps with other methods:** PS, iPS, HN, and iHN are always
  available. Tick a reference region here if you also plan to generate
  Z-score maps using it. Ada ticks **Occipital cortex** and **White matter**
  for the sensitivity analysis in
  [Step 6](./compare#check-that-the-result-is-robust) and as an extra
  cross-check in [Step 5](./classify).

These options apply to the next reference you build; a reference that is
already built keeps the ones it was built with.

## Check the result

Once the reference is built, scroll back to the top of the page.

<Shot name="reference" alt="The reference in use, with the mean of its controls" :marks="[[6, 160], [7, 500]]" />

6. **Reference in use** now describes the new reference: the control count,
   smoothing kernel, covariates, and supported Z-score methods. Every other
   page automatically adopts this reference, and its name appears at the
   bottom of the sidebar.
7. **Mean of the controls:** axial slices of the average of your controls
   after smoothing. It should look like a clean, symmetric brain; a blurred
   edge or a doubled outline means some controls are not well aligned in MNI
   space. The buttons under the picture go straight to the pages that use
   the reference.

::: tip Opening an existing reference
Click **Open Another…** on this page—or click the active reference at the
bottom of the sidebar—and select the folder. References created via the CLI
(`sugar-pet build-reference`) work identically.
:::

## What is in the folder

| File | Contents |
| --- | --- |
| `template.nii.gz` | Mean of the controls after proportional scaling (the HN "normality template") |
| `ps_std.nii.gz` | Standard deviation of the controls after PS |
| `hn_mean.nii.gz`, `hn_std.nii.gz` | Mean and standard deviation of the controls after HN |
| `mask.nii.gz` | Voxels used to estimate the factors |
| `reference.json` | Control count, smoothing, and each control's factor |
| `stats-<method>_mean.nii.gz`, `…_std.nii.gz` | For each method ticked under *Z-score maps with other methods* (item 4): control mean and SD after that method |
| `model-<method>_beta.nii.gz`, `…_sd.nii.gz` | When a covariate table is provided: regression parameters for age and sex |

Leave the files inside this folder untouched. To add or remove controls or
change the options, build a new reference.

::: details The same from the command line
```bash
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ \
    --covariates participants.tsv --stats-for occipital,white-matter
```
:::

## Next

[Step 3: Normalise the intensity](./normalize).
