# Normalise the intensity

**Page: Intensity Normalisation** · step 1 of the individual analysis

This step does two things. First it summarises the healthy controls of the
project into the **control statistics**: the mean and spread of every voxel
for each method, which the patients are compared with from then on. Then it
divides every image of the project, patients and controls, by its
normalisation factor, and writes the normalised images and a table of the
factors. Z-score Maps, Classify Parkinsonism, Compare Groups and Quick
Classification all read what it wrote.

## Choose the methods

<Shot name="normalize" alt="The Intensity Normalisation page: images and methods" :marks="[[1, 109], [2, 269], [3, 503], [4, 666]]" />

1. **The line of steps:** the three steps of the individual analysis (with
   Spatial Normalisation before them for images in native space). A tick
   means a step is done and still rests on the steps before it; click a step
   to go there.
2. **Images:** every image of the project, in MNI space, from the
   [Data page](./project). Patients and controls are normalised the same way,
   so that they can be compared. To change them, **Change in Data**.
3. **Methods.** **Histogram (HN)** is recommended. The methods come in three
   groups:
   * *Data-driven, against the controls*: HN, iHN and iPS, which compare each
     scan with the control statistics;
   * *Global*: PS, SPM global mean and whole brain, which average over
     (nearly) the whole brain or gray matter;
   * *Reference regions*: the mean uptake of a region the disease is expected
     to spare.
4. **Several methods** are for comparison only. Each writes its own image
   from the original scan; the cerebellum image is the scan divided by its
   cerebellar mean, not the HN image divided again. Ada also ticks
   **Cerebellum**. When the reference regions are folded away, **Reference
   regions: 7 more methods** shows them.

Not sure which to choose? See [Choosing a method](../reference/methods).

### A region of your own

Tick **Custom region** for a card with three ways to give the region:

| Region source | What you give |
| --- | --- |
| Built-in mask | One of the [built-in masks](./masks), e.g. *Brainstem* |
| Mask file | An MNI-space NIfTI file of your own; voxels > 0 are the region |
| Atlas labels | CerebrA label numbers, e.g. `34,85` for the lateral occipital cortex of both sides |

A custom region makes normalised images but no z-score maps: there are no
control statistics for it.

## The control statistics

Scroll down.

<Shot name="normalize-controls" alt="The primary method and the control statistics" :marks="[[5, 279], [6, 366], [7, 554], [8, 610]]" />

5. **Primary method**, shown when several methods are ticked: the one
   Classify Parkinsonism, Compare Groups and Quick Classification use. HN
   unless you choose another.
6. **Control statistics:** how many controls were summarised, the smoothing
   they were summarised with (8 mm by default), and the covariates modelled,
   *age* and *sex* when the Data page has a covariate table. Before the
   first run, this part says that the run summarises the controls first.
7. **Z-score maps available for:** the methods the statistics hold: always
   PS, iPS, HN and iHN, the methods ticked here, and those ticked in the
   settings (below).
8. **Saved in:** the `hc_reference` folder of the project. **Show in
   Finder** opens it.

There is nothing to run for the control statistics. The step summarises the
controls again before it normalises the images whenever the controls, their
images on disk, or the settings of the control statistics have changed since;
if only the covariate table changed, the normalised images still stand and
[Z-score Maps](./zmap) summarises the controls anew instead. A note on this
page says when the statistics are to be summarised again.

<Shot name="normalize-output" alt="The mean of the controls, the options and the output" :marks="[[9, 453], [10, 500], [11, 592], [12, 644], [13, 734, 861]]" />

9. **Mean of the controls:** axial slices of the controls' mean image. It
   should look like a sharp, symmetric brain; a blurred edge or a double
   contour means some controls were not well aligned in MNI space.
10. **Options of the control statistics:** the mask and the smoothing, and
    the statistics of further methods kept. **Options…** opens
    **Settings › Control Statistics** ([below](#control-statistics-options)).
11. **Written to:** the `normalised` folder of the project, a folder for each
    group, and `factors.tsv`.
12. **Options of the normalisation:** the defaults (*Mean of the region ·
    target 1 · smoothing automatic*) suit most studies. **Options…** opens
    **Settings › Intensity Normalisation** ([below](#options)).
13. Click **Normalise Intensity**. Once it is done, **Next: Z-score Maps**
    appears beside it.

## Check the results

<Shot name="normalize-results" alt="The chart and the table of the factors" :marks="[[14, 232], [15, 485], [16, 477, 782]]" />

14. **The chart of factors:** a line for each method, a point for each
    image, the patients first and then the controls. The factors of the two
    groups should lie in the same range. An image whose factor stands apart,
    or two lines that do not run in parallel, are worth a second look; hover
    over a point to read it.
15. **The table:** a row for each image and method, with its group:
    * *Factor*: what the image was divided by;
    * *Voxels*: how many voxels the factor was estimated from;
    * *Excluded*: for iPS and iHN, the share of voxels left out as abnormal.
16. **Open factors.tsv** opens the table; **Show in Finder** the folder.

## Output files

| File | What it holds |
| --- | --- |
| `hc_reference/` | The control statistics (see [Output files](../reference/outputs#control-statistics)) |
| `normalised/patients/sub-pt01_space-MNI_desc-hn_pet.nii.gz` | A patient's image divided by its HN factor: one per image and method (`desc-cerebellum`, `desc-ps`, …) |
| `normalised/controls/…` | The same for the controls |
| `normalised/factors.tsv` | A row per image and method: group, factor, voxels, excluded share |

The normalised images are **not smoothed**: each is the input divided by one
number. With HN, PS, iPS and iHN, healthy gray matter lies around 1.

::: warning Using these images in SPM or other software
Turn global scaling off in the statistics package (in SPM, both proportional
scaling and ANCOVA): the images are normalised already, and scaling them
again would undo the method you chose.
:::

## Options

The options of this step are in **Settings › Intensity Normalisation**
(**Options…** on the page opens it). They are kept in the project, and
**Restore Defaults** at the bottom of the section puts them back.

<Shot name="settings-normalize" alt="Settings › Intensity Normalisation" />

| Option | Default | What it does |
| --- | --- | --- |
| **Factors** | | |
| Region statistic | Mean | Mean or median of the region, for the reference regions and PS |
| Target value | 1 | The value the reference takes in the output, e.g. `50` for "global mean = 50" |
| Smoothing before estimating | Automatic | That of the control statistics for HN, iPS and iHN, none otherwise; only for estimating the factor |
| **Output** | | |
| Save masks | off | Also save the voxels each factor came from |
| **Iterative methods** | | |
| Exclusion p-value | 0.01 | iPS and iHN: voxels that differ from the controls at this uncorrected p are left out |
| Exclude | Hypometabolic only | iPS and iHN: only hypometabolic voxels (as in the paper), or hypermetabolic ones too |
| Re-estimations | 1 | iPS and iHN: how many times the factor is estimated again |
| Plain mean for iPS | off | The formula as the paper writes it, instead of the mean relative to the control template |

The factors of the controls are estimated with *Region statistic* and
*Smoothing before estimating* as well, so changing either summarises the
controls again on the next run.

### Control statistics {#control-statistics-options}

<Shot name="settings-controls" alt="Settings › Control Statistics" />

* **Mask:** the voxels the factors are estimated from; gray matter, as in the
  paper. *Mask file…* at the end of the list takes a NIfTI file of your own.
* **Smoothing (FWHM, mm):** applied before the statistics are computed, 8 mm
  by default. Use `0` if the images are smoothed already.
* **Control statistics of other methods:** the statistics always hold those
  of the methods ticked on the page, so z-score maps need nothing here. Tick
  a method to keep its statistics as well: **White matter** adds a check to
  the classification that does not rest on gray matter, and the reference
  regions allow the [sensitivity analysis](./compare#check-that-the-result-is-robust)
  of a group comparison.

::: warning Change a setting after the run, and the step is to be run again
Z-score Maps and Classify Parkinsonism read the images this step wrote, and
do not normalise a second time. So when a setting here is changed after the
step ran, a note at the top of the section says so, the app tells you once
more when you leave the settings window, and the steps after it wait until
it is run again. (The *Iterative methods* count only when the step used iPS
or iHN.)
:::

::: details The same from the command line
```bash
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ --covariates participants.tsv
sugar-pet normalize patients/*.nii.gz -m hn,cerebellum -r hc_reference/ -o normalised/
```
:::

## Next

[Z-score and W-score maps](./zmap).
