# Step 4: Z-score and W-score maps

**Page: Z-score Maps** · needs the reference

A Z-score map shows, voxel by voxel, how far a single scan deviates from
healthy controls: negative values indicate lower uptake (hypometabolism), and
positive values indicate higher uptake.

## Z-scores or W-scores?

| | Z-score | W-score |
| --- | --- | --- |
| Compares the patient against | all controls | expected uptake for a control **of the same age and sex** |
| Formula | `(image − control mean) / control SD` | `(image − predicted) / residual SD of the controls` |
| Extra input needed | none | covariate table with the patient's age and sex |
| Reference requirement | any reference | built with a covariate table ([Step 2](./build-reference), item 5) |
| Filename suffix | `_zmap.nii.gz` | `_wmap.nii.gz` |

Use W-scores whenever patients and controls differ in age; otherwise, plain
Z-scores conflate normal aging effects with disease.

## Fill in the page

<Shot name="zmap" alt="The Z-score Maps page: images, reference and methods" :marks="[[1, 180], [2, 499], [3, 563], [4, 723]]" />

1. **Add the patient images** (in MNI space).
2. **Reference:** pre-filled from Step 2. The line below shows the control
   count and smoothing kernel.
3. **Covariate table** (optional): select `participants.tsv` to compute
   W-scores. The table must include these patients and the same covariate
   columns used when building the reference (`age` and `sex` here). Leave it
   empty for plain Z-scores.
4. **Tick the method.** Keep **Histogram (HN)**. Grayed-out reference regions
   require a reference built with those regions ticked (Step 2, item 4).
   Custom regions are not supported for Z-score maps.

Scroll down.

<Shot name="zmap-results" alt="The output folder and the results of the Z-score maps" :marks="[[5, 379], [6, 'run'], [7, 551]]" />

5. **Output folder:** Ada selects `study/zmaps/`.
6. Click **Make Maps**.
7. **Results:** one row per image and method:
   * *Lowest*: the most negative voxel value in the map.
   * *Below −2*: fraction of brain voxels below −2. Roughly 2.3% is expected
     by chance; patients with widespread hypometabolism show much more.

## Output files

One map per image and method in the output folder:
`sub-pt01_space-MNI_desc-hn_wmap.nii.gz` (or `_zmap.nii.gz` without a
covariate table).

Before computing the map, each image is normalised and smoothed to match the
controls, so the resulting map is smooth even if your input scans are not.

## Reading a map

Open the map in a viewer (MRIcroGL, FSLeyes, or ITK-SNAP) overlaid on an MNI
template:

* Display values **below −2** in a cool colourmap to highlight hypometabolic
  regions (−3 gives a stricter threshold).
* Positive values after HN or PS often represent spared regions rather than
  true hypermetabolism (see [Choosing a method](../reference/methods)).
* If a warning below the results notes that a patient's age falls outside the
  control range, their W-scores are extrapolated and should be interpreted
  cautiously.

::: details The same from the command line
```bash
sugar-pet zmap patients/*.nii.gz -m hn -r hc_reference/ -o zmaps/ --covariates participants.tsv
```
:::

## Next

[Step 5: Classify parkinsonism](./classify).
