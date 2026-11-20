# Compare two groups

**Page: Compare Groups** · needs step 1

This page answers "where do the patients differ from the controls?". It reads
the images [step 1](./normalize) normalised, smooths them, fits a voxel-wise
two-sample t-test, corrects for multiple comparisons and lists the clusters
that survive, with their anatomical labels.

## Run it

<Shot name="compare" alt="The Compare Groups page" :marks="[[1, 180], [2, 221], [3, 874, 307], [4, 413], [5, 569], [6, 705], [7, 751], [8, 808], [9, 'run']]" />

1. **Group 1 · Patients** and 2. **Group 2 · Healthy controls:** the two
   groups of the project, as step 1 normalised them. The T-map is *group 1
   minus group 2*, so with the patients as group 1, **negative T-values mean
   hypometabolism**. To change the images, **Change in Data**.
3. **Swap Groups** turns them round.
4. **Method:** the primary method of step 1; nothing is normalised a second
   time here. To use another, make it primary in step 1 (**Change in
   Intensity Normalisation**) and run it again. **Histogram (HN)** is
   recommended for the main analysis.
5. **Adjusted for:** the covariates chosen on the Data page, *age* and *sex*
   from `participants.tsv` here. The table must list every image of both
   groups.
6. **Written to:** the `compare` folder of the project.
7. **Name of the result files:** the prefix of every file, here
   `patients_vs_controls`.
8. **Options in effect:** the correction and its thresholds, the voxels
   tested and the smoothing. Keep the defaults (*GRF correction: voxel p <
   0.001, cluster p < 0.05 · within Gray matter · smoothing automatic*)
   unless you have a reason; **Options…** opens **Settings › Compare
   Groups** ([below](#options)).
9. Click **Compare**.

## Check the results

<Shot name="compare-results" alt="The results of the comparison" :marks="[[10, 154], [11, 192], [12, 602], [13, 645], [14, 268, 782]]" />

10. **The line under Results** states the correction exactly as it was
    applied, ready for the methods section of a paper.
11. **Summary:** the degrees of freedom, the smoothness of the data the
    correction used, and the number of significant clusters.
12. **The picture** draws the T-map of the clusters that survive on axial
    slices of the template: blue to cyan where group 1 is lower, red to
    yellow where it is higher. Without a correction it shows the uncorrected
    T-map where |T| > 3.
13. **Clusters:** a row per cluster: voxels, peak T and its MNI coordinates,
    the region of the peak and the regions covered. Blue peaks are negative
    (group 1 lower), red ones positive.
14. **Show in Finder** opens the `compare` folder.

## Output files

| File in `compare/` | What it holds |
| --- | --- |
| `patients_vs_controls_T.nii.gz` | The unthresholded T-map |
| `ClusterThresholded_patients_vs_controls_T.nii.gz` | The T-map of the surviving clusters only |
| `ClusterReport_patients_vs_controls_T.csv` | A row per cluster: size, peak, AAL and Harvard-Oxford regions |
| `patients_vs_controls_factors.tsv` | The normalisation factor of every scan |
| `patients_vs_controls_mask.nii.gz` | The voxels tested |
| `patients_vs_controls_beta.nii.gz`, `…_cohen_f2.nii.gz` | Model coefficients and effect size |

## Options

<Shot name="settings-compare" alt="Settings › Compare Groups" />

| Option | Default | What it does |
| --- | --- | --- |
| Multiple-comparison correction | GRF | **GRF**: cluster-level, by Gaussian random field theory. **FDR**: voxel-level false discovery rate. **None**: only the unthresholded T-map. All tests are two-tailed |
| Voxel-level p | 0.001 | GRF: the two-tailed threshold that forms the clusters |
| Cluster-level p | 0.05 | GRF: the significance of a cluster |
| q value | 0.05 | FDR: the false discovery rate (in place of the two p-values) |
| Voxels to test | Gray matter | A built-in mask or a file of your own; voxels that any scan does not cover are left out |
| Smoothing before the test | Automatic | That of the control statistics (8 mm before they exist); `0` if the scans are smoothed already |
| Keep the images that were tested | off | Also write the normalised, smoothed scans of both groups |

## Check that the result is robust

A finding that rests on the choice of normalisation is not robust. Tick the
methods below as well in step 1, run it, then run the comparison twice more,
each time with another **Primary method** in step 1 and another **Name of
the result files** here:

1. **Occipital cortex:** an anatomical reference that MSA, PSP and CBS
   usually spare;
2. **White matter:** assumes nothing about gray matter, so it can show
   diffuse decreases that HN misses, with more noise.

If the main findings hold, report the HN results and say that you checked.
If they change, report that.

::: tip In your paper
Report the method (`HN`), the number, age range and sex of the controls, the
smoothing (8 mm by default), the correction (the line under **Results**) and
the version of SUGAR-PET.
:::

::: details The same from the command line
```bash
sugar-pet compare --group1 patients/*.nii.gz --group2 controls/*.nii.gz \
    -m hn -r hc_reference/ -o compare/ --covariates participants.tsv --name patients_vs_controls
```
:::

## Next

[Reference cluster](./reference-cluster), or the built-in
[masks and atlas labels](./masks).
