# Step 3: Normalise the intensity

**Page: Intensity Normalisation** · step 1 of the individual analysis

This page computes the normalisation factor for each scan and writes out the
normalised images alongside a factor table. Use it when you need the
normalised NIfTI files themselves—for instance, to run statistics in external
software.

::: tip Not always needed
[Z-score maps](./zmap), [parkinsonism classification](./classify), and
[group comparisons](./compare) normalise images internally, so you do not need
to run this step beforehand.
:::

## Choose the images and methods

<Shot name="normalize" alt="The Intensity Normalisation page: images and methods" :marks="[[1, 214], [2, 579], [3, 742]]" />

1. **Add the images** (in MNI space). Ada adds six patient scans from
   `study/patients/`.
2. **Select the method(s).** **Histogram (HN)** is recommended and uses the
   reference from [Step 2](./build-reference). Methods are grouped into three
   categories:
   * *Data-driven*: HN, iHN, and iPS, which compare each scan against the
     control reference.
   * *Global*: PS, SPM global mean, and whole brain, which average across
     (nearly) the entire brain or gray matter.
   * *Reference regions*: the mean uptake within a region expected to be
     spared by the disease.
3. **Select multiple methods to compare them.** Each ticked method writes a
   separate normalised image per input scan. Ada also ticks **Cerebellum**.
   The reference regions are folded away at first: click **Reference
   regions: 7 more methods** under the *Global* row to show them, and
   **Fewer methods** to fold them again. They always stay in view while one
   of them is ticked.

::: tip One method is enough
Each method is a complete normalisation on its own. HN does not need a
reference region or the whole brain on top of it: ticking **Histogram (HN)**
alone is the usual choice. Ada ticks **Cerebellum** as well only to compare
the two. The results are independent: the cerebellum image is the original
scan divided by its cerebellar mean, not the HN image divided a second time.
With several methods ticked, **Primary method** says which one Classify
Parkinsonism and Compare Groups use (HN unless you choose another).
:::

Not sure which to choose? See [Choosing a method](../reference/methods).

### A region of your own

Tick **Custom region** to reveal a configuration card with three ways to
define your region:

| Region source | Input required |
| --- | --- |
| Built-in mask | One of the [built-in masks](./masks), e.g. *Brainstem* |
| Mask file | Your own MNI-space NIfTI file; voxels > 0 define the region |
| Atlas labels | Comma-separated CerebrA atlas IDs, e.g. `34,85` for bilateral lateral occipital cortex |

## Choose the output and run

Scroll down.

<Shot name="normalize-output" alt="The reference, the output folder, the options and the chart of factors" :marks="[[4, 406], [5, 462], [6, 521], [7, 707, 845], [8, 614]]" />

4. **Reference:** pre-filled if you completed Step 2. Only required for HN,
   iPS, and iHN.
5. **Output folder:** Ada selects `study/normalised/`.
6. **Options in effect:** the defaults (*Mean of the region · target 1 ·
   smoothing automatic*) work well for most studies. **Options…** opens
   **Settings › Intensity Normalisation** to change them (see
   [below](#options)).
7. Click **Normalise Intensity**. Once it is done, **Next: Z-score Maps**
   appears beside it: the two later steps of the individual analysis take the
   images and methods from here and do not ask for them again. If you change
   the images, the methods or the reference on this page afterwards, run it
   again before going on.
8. **Chart of factors:** one line per method, one point per image, numbered
   in the order of the table below; hover over a point to read the image and
   its factor. An image whose factor stands apart from the others, or two
   methods whose lines do not run in parallel, are worth a second look.

<Shot name="normalize-results" alt="The table of factors" :marks="[[9, 520], [10, 784]]" />

9. **Table:** one row per image and method:
   * *Factor*: the divisor applied to the image.
   * *Voxels*: number of voxels used to estimate the factor.
   * *Excluded*: for iPS and iHN, the fraction of voxels excluded as abnormal.
10. **Open factors.tsv** opens the saved table directly; **Show in Finder**
    opens the output folder.

## Output files

Inside the output folder:

| File | Description |
| --- | --- |
| `sub-pt01_space-MNI_desc-hn_pet.nii.gz` | Input image divided by its HN factor. Written once per image and method (`desc-cerebellum`, `desc-ps`, …) |
| `factors.tsv` | Summary table with one row per image and method (factor, voxel count, excluded fraction) |

The normalised images are **not smoothed**—each is simply the input divided
by a single scalar. With HN, PS, iPS, and iHN, healthy gray matter averages
around 1 in the output.

::: warning Using these images in SPM or other software
Disable global scaling in your downstream statistics package (in SPM, turn off
both proportional scaling and ANCOVA). Because these images are already
normalised, scaling them again would overwrite your chosen method.
:::

## Options

The options of this page are in **Settings › Intensity Normalisation**
(**Options…** on the page opens it). They are remembered between launches,
and **Restore Defaults** at the bottom of the section puts them back.

<Shot name="settings-normalize" alt="Settings › Intensity Normalisation" />

| Option | Default | Description |
| --- | --- | --- |
| **Factors** | | |
| Region statistic | Mean | Mean or median of the region, used by reference regions and PS |
| Target value | 1 | Target value for the reference in the output (e.g. `50` for "global mean = 50") |
| Smoothing before estimating | Automatic | Matches the reference for HN, iPS, and iHN; none otherwise (used only for factor estimation) |
| **Output** | | |
| Save masks | off | Also save the voxel mask used to estimate each factor |
| **Iterative methods** | | |
| Exclusion p-value | 0.01 | iPS and iHN: voxels differing from controls below this uncorrected p-value are excluded |
| Exclude | Hypometabolic only | iPS and iHN: exclude only hypometabolic voxels (as in the paper), or hypermetabolic ones too |
| Re-estimations | 1 | iPS and iHN: number of iterative re-estimation passes |
| Plain mean for iPS | off | Uses the literal mean formula from the paper instead of dividing by the control template mean over the remaining voxels |

::: warning Change a setting after the run, and the step is to be run again
[Z-score Maps](./zmap) and [Classify Parkinsonism](./classify) read the
normalised images this step wrote, and do not normalise a second time. So
when an option here is changed after Intensity Normalisation ran, a note at
the top of the section says so, the app tells you once more when you leave
the settings window, and the two later steps wait until the step is run
again with the new settings. (The *Iterative methods* count only when the step used iPS or iHN.)

*Region statistic* and *Smoothing before estimating* are also used when
[a reference is built](./build-reference#options): the factors of the
controls are estimated with the two as they are then, and the reference
records them. For Z-score maps, step 1 and the reference must agree in the
two, or the images cannot be compared with the controls; the Z-score Maps
page then offers both ways out: make the settings agree with the reference
and run step 1 again, or build the reference again with the settings as
they are. The *Target value* is free.
:::

::: details The same from the command line
```bash
sugar-pet normalize patients/*.nii.gz -m hn,cerebellum -r hc_reference/ -o normalised/
```
:::

## Next

[Step 4: Z-score and W-score maps](./zmap).
