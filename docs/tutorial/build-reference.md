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

<Shot name="reference" alt="The Reference page: the reference in use, and the controls for a new one" :marks="[[1, 469], [8, 160]]" />

1. **Add the control images** under *Build a new reference*. Ada drops the
   `study/controls/` folder onto the list (16 images). All controls must be in
   MNI space on the same voxel grid. At least 3 controls are required, though
   voxel-wise statistics are unreliable with fewer than about 10.

(Callout 8 at the top shows where the summary appears once built; see below.)

## Choose the settings

Scroll down.

<Shot name="reference-settings" alt="The settings of a new reference" :marks="[[2, 170], [3, 227], [4, 356], [5, 578], [6, 768], [7, 'run']]" />

2. **Mask:** the voxels used to estimate normalisation factors. Keep **Gray
   matter**, matching the original paper.
3. **Smoothing:** keep **8** mm. Set this to `0` only if your images are
   already smoothed.
4. **Z-score maps with other methods:** PS, iPS, HN, and iHN are always
   available. Tick a reference region here if you also plan to generate
   Z-score maps using it. Ada ticks **Occipital cortex** and **White matter**
   for the sensitivity analysis in
   [Step 6](./compare#check-that-the-result-is-robust) and as an extra
   cross-check in [Step 5](./classify).
5. **Covariate table:** click **Choose…** and select `participants.tsv`. The
   table's columns appear as checkboxes under *Adjust for*; tick **age** and
   **sex**. This enables age- and sex-adjusted
   [W-scores](./zmap#z-scores-or-w-scores) later. Skip this if you don't have
   a covariate table.
6. **Reference folder:** choose where to save the reference. Ada creates a new
   folder, `study/hc_reference/`.
7. Click **Build Reference**. The app reads and processes the controls one by
   one.

## Check the result

8. Once finished, the **reference in use** card at the top updates with the
   control count, smoothing kernel, covariates, and supported Z-score
   methods. Every other page automatically adopts this reference, and its
   name appears at the bottom of the sidebar.

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
| `stats-<method>_mean.nii.gz`, `…_std.nii.gz` | For each region ticked in item 4: control mean and SD after that method |
| `model-<method>_beta.nii.gz`, `…_sd.nii.gz` | When a covariate table is provided: regression parameters for age and sex |

Leave the files inside this folder untouched. To add or remove controls or
change settings, build a new reference.

::: details The same from the command line
```bash
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ \
    --covariates participants.tsv --stats-for occipital,white-matter
```
:::

## Next

[Step 3: Normalise the intensity](./normalize).
