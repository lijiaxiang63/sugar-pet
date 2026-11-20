# Z-score and W-score maps

**Page: Z-score Maps** · step 2 of the individual analysis

A Z-score map shows, voxel by voxel, how far a patient's scan is from the
healthy controls of the project: negative values mean lower uptake
(hypometabolism), positive values higher uptake.

## Z-scores or W-scores?

| | Z-score | W-score |
| --- | --- | --- |
| The patient is compared with | all the controls | what the controls predict for someone **of the same age and sex** |
| Formula | `(image − control mean) / control SD` | `(image − predicted) / residual SD of the controls` |
| Needs | nothing more | a covariate table on the [Data page](./project#age-and-sex) |
| File name ends in | `_zmap.nii.gz` | `_wmap.nii.gz` |

Use W-scores whenever the patients and the controls differ in age: plain
Z-scores mix the effects of normal ageing with those of the disease. The page
makes W-score maps by itself when the control statistics model the
covariates and the table gives them for the patients.

## Run it

Z-score Maps takes the patients and the methods of step 1 and does not ask
for them again: it reads the normalised images step 1 wrote, smooths them
like the controls and compares them with the control statistics voxel by
voxel. No map is made of the controls; they are what the patients are
compared with. While step 1 has not run, or something it ran with changed
since, the page says so and **Make Maps** stays disabled.

<Shot name="zmap" alt="The Z-score Maps page" :marks="[[1, 109], [2, 287], [3, 369], [4, 861, 410], [5, 502], [6, 671], [7, 708, 861], [8, 888, 861]]" />

1. **The line of steps:** a tick means a step is done; click one to go there.
2. **Patients** and **Methods:** what step 1 normalised. A method that makes
   no maps (a custom region, or one the control statistics hold nothing for)
   is named in a note under the card and left out.
3. **Healthy controls:** how many controls the patients are compared with,
   and their smoothing.
4. To change any of these, **Change in Step 1**, run it again, and come back.
5. **Covariate table:** whether the maps are W-scores. The table and its
   columns are chosen on the Data page; without one, the maps are plain
   Z-scores.
6. **Written to:** the `zmaps` folder of the project.
7. Click **Make Maps**. If only the covariate table changed since step 1,
   this run first summarises the controls anew with it.
8. Once it is done, **Next: Classify Parkinsonism** leads on.

## Check the results

<Shot name="zmap-results" alt="A map, and the table of the maps" :marks="[[9, 126], [10, 506], [11, 549]]" />

9. **The picture** draws one map on axial slices of the patient's own image.
   The arrows step through the maps, as does a click on a row of the table.
10. **Colours:** blue to cyan from −2 down to −5 and below (lower uptake than
    the controls), red to yellow from +2 up to +5 and above. Voxels between
    −2 and +2 are not coloured. The legend starts with *W* for W-scores and
    *z* for Z-scores.
11. **The table:** a row for each image and method.
    * *Lowest*: the most negative value of the map.
    * *Below −2*: the share of brain voxels below −2. About 2.3% is expected
      by chance; widespread hypometabolism shows far more.

## Output files

One map per patient and method in `zmaps/`:
`sub-pt01_space-MNI_desc-hn_wmap.nii.gz`, or `_zmap.nii.gz` without
covariates. The maps are smoothed like the controls, so they are smooth even
when the scans are not.

## Reading a map

The picture is a first look. To go through a map slice by slice, open it in a
viewer (MRIcroGL, FSLeyes or ITK-SNAP) over an MNI template:

* Show values **below −2** in a cool colour map to see the hypometabolic
  regions (−3 is stricter).
* Positive values after HN or PS are often spared regions rather than true
  hypermetabolism (see [Choosing a method](../reference/methods)).
* When a note under the results says a patient's age lies outside that of
  the controls, the W-scores are extrapolated: read them with care.

::: details The same from the command line
```bash
sugar-pet zmap patients/*.nii.gz -m hn -r hc_reference/ -o zmaps/ --covariates participants.tsv
```
:::

## Next

[Classify parkinsonism](./classify).
