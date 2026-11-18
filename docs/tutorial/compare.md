# Step 6: Compare two groups

**Page: Compare Groups**

This page answers "where do patients differ from controls?". In a single run,
it normalises all scans from both groups with the same method, smooths them,
fits a voxel-wise two-sample t-test, applies multiple-comparison correction,
and outputs the surviving clusters with anatomical labels.

## Fill in the page

<Shot name="compare" alt="The Compare Groups page: groups, normalisation and statistics" :marks="[[1, 181], [2, 1027, 181], [3, 514], [4, 570], [5, 682], [6, 739]]" />

1. **Group 1:** add the patient scans (12 images).
2. **Group 2:** add the control scans (16 images). The contrast is *group 1
   minus group 2*, so with patients in Group 1, **negative T-values indicate
   hypometabolism**. Make sure each subject appears only once.
3. **Method:** keep **Histogram (HN)** for the primary analysis.
4. **Reference:** pre-filled automatically; required for HN, iPS, and iHN.
   Both groups share the same reference.
5. **Multiple-comparison correction:**
   * **GRF**: cluster-level correction via Gaussian random field theory (default).
   * **FDR**: voxel-level false discovery rate.
   * **None**: outputs only the unthresholded T-map.
6. **Thresholds:** for GRF, the *voxel-level p* that forms clusters (0.001)
   and the *cluster-level p* (0.05); for FDR, the *q value* (0.05). All tests
   are two-tailed. Leave the defaults unless you have a specific reason to
   change them.

Scroll down.

<Shot name="compare-results" alt="Covariates, output and the results of the comparison" :marks="[[7, 246], [8, 354], [9, 403], [10, 'run'], [11, 592], [12, 647]]" />

7. **Covariate table** (recommended): select `participants.tsv` and tick
   **age** and **sex** to include them as covariates. The table must cover
   every scan in both groups. If patients and controls live in separate
   tables, a **Second table** field appears for the other file.
8. **Output folder:** Ada selects `study/compare/`.
9. **Name of the result files:** filename prefix for all outputs (here
   `patients_vs_controls`).
10. Click **Compare**.
11. **Summary:** degrees of freedom, estimated data smoothness used by the
    correction, and the number of significant clusters.
12. **Clusters:** one row per surviving cluster, listing voxel count, peak
    T-value and MNI coordinates, peak region, and covered brain regions. Blue
    peaks are negative (Group 1 lower); red peaks are positive.

The line under the **Results** heading summarises the exact correction
applied, ready to adapt for your methods section.

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

| Option | Default | Description |
| --- | --- | --- |
| Voxels to test | Gray matter | A built-in mask or your own file; voxels missing coverage in any scan are excluded |
| Smoothing before the test | Automatic | Matches the reference (8 mm if none); set to `0` if scans are already smoothed |
| Keep the images that were tested | off | Also write the normalised, smoothed scans for both groups |

## Check that the result is robust

A finding that depends heavily on the normalisation choice is not robust.
Re-run the comparison twice more, changing only **Method** and **Name of the
result files**:

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
