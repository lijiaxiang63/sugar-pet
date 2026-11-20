# Step 4: Z-score and W-score maps

**Page: Z-score Maps** · step 2 of the individual analysis · needs the reference

A Z-score map shows, voxel by voxel, how far a single scan deviates from
healthy controls: negative values indicate lower uptake (hypometabolism), and
positive values indicate higher uptake.

## Z-scores or W-scores?

| | Z-score | W-score |
| --- | --- | --- |
| Compares the patient against | all controls | expected uptake for a control **of the same age and sex** |
| Formula | `(image − control mean) / control SD` | `(image − predicted) / residual SD of the controls` |
| Extra input needed | none | covariate table with the patient's age and sex |
| Reference requirement | any reference | built with a covariate table ([Step 2](./build-reference), item 2) |
| Filename suffix | `_zmap.nii.gz` | `_wmap.nii.gz` |

Use W-scores whenever patients and controls differ in age; otherwise, plain
Z-scores conflate normal aging effects with disease.

## Fill in the page

Z-score Maps is step 2 of the individual analysis: it takes the images and
the methods chosen on [Intensity Normalisation](./normalize) (its step 1) and
does not ask for them again. It reads the normalised images step 1 wrote and
does not normalise a second time. While Intensity Normalisation has not run,
or its images, methods, reference or [settings](./normalize#options) were
changed after it ran, the page says so and **Make Maps** stays disabled.

<Shot name="zmap" alt="The Z-score Maps page: what it takes from step 1, the covariate table and the output" :marks="[[1, 109], [2, 276], [3, 358], [4, 466], [5, 584], [6, 682, 845], [7, 867, 845]]" />

1. **The line of steps:** the three steps of the individual analysis. A tick
   means the step is done and still rests on the steps before it; click a
   step to go there.
2. **From step 1:** the images, methods and reference that Intensity
   Normalisation ran with, read-only. If one of the methods cannot make maps
   (a custom region, or a method not ticked when the reference was built;
   Step 2, item 4), a note under the card says that it is left out here.
3. To use other images or methods, click **Change in Step 1**, run Intensity
   Normalisation again, and come back.
4. **Covariate table** (optional): select `participants.tsv` to compute
   W-scores. The table must include these patients and the same covariate
   columns used when building the reference (`age` and `sex` here). Leave it
   empty for plain Z-scores.
5. **Output folder:** Ada selects `study/zmaps/`.
6. Click **Make Maps**.
7. Once it is done, **Next: Classify Parkinsonism** appears beside it.

## Check the results

<Shot name="zmap-results" alt="A map on the brain, and the table of the maps" :marks="[[8, 135], [9, 508], [10, 584]]" />

8. **The picture** draws one map on axial slices of the patient's own image.
   The arrows above it step through the maps, and so does a click on a row
   of the table.
9. **Colours:** blue to cyan for values from −2 down to −5 and below (lower
   uptake than the controls), red to yellow for +2 up to +5 and above.
   Voxels between −2 and +2 are left uncoloured. The legend starts with *W*
   for W-scores and *z* for Z-scores.
10. **The table** has one row per image and method:
    * *Lowest*: the most negative voxel value in the map.
    * *Below −2*: fraction of brain voxels below −2. Roughly 2.3% is expected
      by chance; patients with widespread hypometabolism show much more.

## Output files

One map per image and method in the output folder:
`sub-pt01_space-MNI_desc-hn_wmap.nii.gz` (or `_zmap.nii.gz` without a
covariate table).

Each map is made of the normalised image step 1 wrote with the same method:
it is smoothed to match the controls and compared with them voxel by voxel,
so the map is smooth even if your input scans are not.

A normalised image can be compared with the controls only if it was scaled
the way they were. A *Target value* set in step 1 does no harm (it is divided
out first), but *Region statistic* and *Smoothing before estimating* must be
what they were [when the reference was built](./build-reference#options).
If they are not, the page says what each side used and offers both ways
out: **Open Settings…**, to make the settings agree with the reference and
run step 1 again, or **Go to Reference**, to build the reference again with
the settings as they are.

## Reading a map

The picture on the page is a first look. To go through a map slice by slice,
open it in a viewer (MRIcroGL, FSLeyes, or ITK-SNAP) overlaid on an MNI
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
