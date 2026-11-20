# Step 6: Compare two groups

**Page: Compare Groups**

This page answers "where do patients differ from controls?". In a single run,
it normalises all scans from both groups with the same method, smooths them,
fits a voxel-wise two-sample t-test, applies multiple-comparison correction,
and outputs the surviving clusters with anatomical labels.

## Fill in the page

<Shot name="compare" alt="The Compare Groups page: groups, normalisation and covariates" :marks="[[1, 177], [2, 1027, 177], [3, 584], [4, 640], [5, 770]]" />

1. **Group 1:** add the patient scans (12 images).
2. **Group 2:** add the control scans (16 images). The contrast is *group 1
   minus group 2*, so with patients in Group 1, **negative T-values indicate
   hypometabolism**. Make sure each subject appears only once. **Swap
   Groups**, under the two lists, exchanges them.
3. **Method:** the primary method of step 1 of the individual analysis
   ([Intensity Normalisation](./normalize)); to use another, make it
   primary there. **Histogram (HN)** is recommended for the primary
   analysis.
4. **Reference:** pre-filled automatically; required for HN, iPS, and iHN.
   Both groups share the same reference.
5. **Covariate table** (recommended): the table chosen when the reference
   was built is filled in for you (Ada's `participants.tsv`); check that
   **age** and **sex** are ticked, or remove the table with the × beside its
   name to leave the covariates out. If none is filled in, choose one. The table must cover
   every scan in both groups. If patients and controls live in separate
   tables, a **Second table** field appears for the other file.

Scroll down.

<Shot name="compare-output" alt="The output, the options and the start of the results" :marks="[[6, 457], [7, 506], [8, 562], [9, 'run']]" />

6. **Output folder:** Ada selects `study/compare/`.
7. **Name of the result files:** filename prefix for all outputs (here
   `patients_vs_controls`).
8. **Options in effect:** the multiple-comparison correction and its
   thresholds, the voxels that are tested, and the smoothing. Leave the
   defaults (*GRF correction: voxel p < 0.001, cluster p < 0.05 · within Gray
   matter · smoothing automatic*) unless you have a specific reason to change
   them; **Options…** opens **Settings › Compare Groups** (see
   [Options](#options)).
9. Click **Compare**.

## Check the results

<Shot name="compare-results" alt="The results of the comparison" :marks="[[10, 158], [11, 208], [12, 430], [13, 712]]" />

10. **The line under the Results heading** summarises the exact correction
    applied, ready to adapt for your methods section.
11. **Summary:** degrees of freedom, estimated data smoothness used by the
    correction, and the number of significant clusters.
12. **The picture** draws the T-map of the clusters that survive the
    correction on axial slices of the template: blue to cyan where Group 1
    is lower, red to yellow where it is higher. Without a correction, it
    shows the uncorrected T-map where |T| > 3.
13. **Clusters:** one row per surviving cluster, listing voxel count, peak
    T-value and MNI coordinates, peak region, and covered brain regions. Blue
    peaks are negative (Group 1 lower); red peaks are positive.

## Output files

| File in the output folder | Description |
| --- | --- |
| `patients_vs_controls_T.nii.gz` | Unthresholded T-map |
| `ClusterThresholded_patients_vs_controls_T.nii.gz` | T-map masked to surviving clusters only |
| `ClusterReport_patients_vs_controls_T.csv` | One row per cluster: size, peak, AAL and Harvard-Oxford regions |
| `patients_vs_controls_factors.tsv` | Normalisation factor for every scan |
| `patients_vs_controls_mask.nii.gz` | Voxels included in the test |
| `patients_vs_controls_beta.nii.gz`, `…_cohen_f2.nii.gz` | Model coefficients and effect size |

## Options

The statistics of the comparison are set in **Settings › Compare Groups**
(**Options…** on the page opens it).

<Shot name="settings-compare" alt="Settings › Compare Groups" />

| Option | Default | Description |
| --- | --- | --- |
| Multiple-comparison correction | GRF | **GRF**: cluster-level correction via Gaussian random field theory. **FDR**: voxel-level false discovery rate. **None**: outputs only the unthresholded T-map. All tests are two-tailed |
| Voxel-level p | 0.001 | GRF: the two-tailed threshold that forms the clusters |
| Cluster-level p | 0.05 | GRF: the significance level of a cluster |
| q value | 0.05 | FDR: the false discovery rate (shown in place of the two p-values when FDR is chosen) |
| Voxels to test | Gray matter | A built-in mask or your own file; voxels missing coverage in any scan are excluded |
| Smoothing before the test | Automatic | Matches the reference (8 mm if none); set to `0` if scans are already smoothed |
| Keep the images that were tested | off | Also write the normalised, smoothed scans for both groups |

## Check that the result is robust

A finding that depends heavily on the normalisation choice is not robust.
Tick the methods below as well in Intensity Normalisation, then re-run the
comparison twice more, changing only the **Primary method** there and the
**Name of the result files** here:

1. with **Occipital cortex**: an anatomical reference usually spared in MSA,
   PSP, and CBS;
2. with **White matter**: does not assume most gray matter is normal, so it
   can reveal diffuse decreases that HN misses, though with higher noise.

If the main findings hold, report the HN results and note that you ran this
sensitivity check. If they change, report that transparently.

::: tip In your paper
Report the method (`HN`), the number, age range, and sex of the controls, the
smoothing kernel (8 mm by default), the multiple-comparison correction (from
the line under the **Results** heading), and the SUGAR-PET version.
:::

::: details The same from the command line
```bash
sugar-pet compare --group1 patients/*.nii.gz --group2 controls/*.nii.gz \
    -m hn -r hc_reference/ -o compare/ --covariates participants.tsv --name patients_vs_controls
```
:::

## Next

[Step 7: Reference cluster](./reference-cluster), or browse the built-in
[masks and atlas labels](./masks).
