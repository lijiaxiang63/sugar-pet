# Step 5: Classify parkinsonism

**Page: Classify Parkinsonism** · step 3 of the individual analysis · needs the reference

For a patient with an uncertain diagnosis, this page identifies brain regions
where uptake is lower or higher than in healthy controls, evaluates how closely
that pattern matches major parkinsonian syndromes, and generates a report.

::: warning Decision aid, not a clinical diagnosis
Current diagnostic criteria (MSA 2022, PSP 2017, DLB 2017) treat FDG-PET as
supportive evidence only. The rules implemented here come from published
literature and have not been fitted to or validated on your patients. Always
interpret the report alongside clinical findings, structural MRI, and
dopamine-transporter imaging.
:::

## Before you start

* [Step 3](./normalize) and [Step 4](./zmap): the classification takes their
  images, methods and covariate table.
* The reference from [Step 2](./build-reference). For single-patient
  classification, 30 or more controls are recommended, acquired under the same
  conditions (eyes open or closed) as the patients.
* Whenever possible, build the reference with a **covariate table** and with
  **White matter** ticked under *Z-score maps with other methods*
  ([Step 2](./build-reference#options)): the former adjusts scores for age
  and sex, and the latter adds a cross-check that does not depend on gray
  matter.

## Fill in the page

Classify Parkinsonism is step 3 of the individual analysis: it reads the
deviation map that [Z-score Maps](./zmap) (its step 2) made of each patient,
and asks for no images, method, reference or covariate table again; nothing
is normalised or mapped a second time. The step computes only the maps its
cross-checks need (PS, and deep white matter when the reference holds it),
and takes those too from step 2 where it made them. While
Z-score Maps has not run, or something before it was changed after it ran,
the page says so and **Classify** stays disabled.

<Shot name="classify" alt="The Classify Parkinsonism page, filled in" :marks="[[1, 109], [2, 296], [3, 399], [4, 507], [5, 580], [6, 642], [7, 'run']]" />

1. **The line of steps:** the three steps of the individual analysis; a tick
   means the step is done.
2. **From steps 1 and 2:** read-only. *Method* names the method the
   classification rests on and those it is checked with (always PS, plus
   white matter if the reference holds its statistics); *Reference* warns if
   fewer than 30 controls are available; *Covariate table* says whether the
   scores are adjusted for age and sex.
3. To change any of these, click **Change in Step 2**.
4. **Output folder:** Ada selects `study/reports/`.
5. The classification rests on the **primary method** of step 1, chosen
   there when it ran with several. Keep **Histogram (HN)**: it requires no
   prior assumption about which regions are spared, making it well suited to
   undiagnosed cases.
6. **Options in effect:** the language of the report. **Options…** opens
   **Settings › Classify Parkinsonism** (below).
7. Click **Classify**.

### The options of the classification {#options}

<Shot name="settings-classify" alt="Settings › Classify Parkinsonism" />

* **Language of the report:** as the app, English, or Chinese.

The normalisation is not set here: the classification uses the method chosen
in the first two steps of the individual analysis.

## Check the results

<Shot name="classify-results" alt="The results of the classification" :marks="[[8, 294], [9, 751, 258], [10, 1012, 294]]" />

8. **Results:** one row per patient, showing the conclusion in bold (with its
   confidence beside it when it names a pattern) and, under it, the image and
   how the conclusion was reached.
9. **Pattern bars:** how well the patient matches each of the six patterns
   (MSA-P, MSA-C, PSP, CBD, DLB, PD), from 0 to 100. The leading pattern is
   in colour; hover over a bar to read its score. Two bars of similar height,
   as for the second patient, mean the two syndromes cannot be told apart.
10. **Open Report** shows that patient's report in your browser.

Scroll down to the end of the page.

<Shot name="classify-figure" alt="The deviation map of one patient" :marks="[[11, 560], [12, 784]]" />

11. **The picture** draws the deviation map of one patient on axial slices of
    their image: blue to cyan where uptake is lower than in the controls
    (−2 down to −5), red to yellow where it is higher. Click a row to draw
    that patient.
12. **Show in Finder** opens the output folder.

## Reading the conclusion

| Conclusion | Meaning |
| --- | --- |
| **A pattern**, e.g. *Multiple system atrophy, parkinsonian type (MSA-P)* | One pattern scores at least 50 out of 100 and leads the next disease by at least 15 points. Confidence is *high* for scores ≥ 70 with a lead ≥ 30, otherwise *moderate*; it drops to *low* if the supporting findings do not hold across all checked normalisations |
| ***A* or *B*** | Two syndromes score closely; both are listed |
| **Partial features of…** | Highest score falls between 30 and 50—suggestive, but not strong enough to assign a pattern |
| **PD pattern** | No atypical parkinsonism pattern is present, but relative increases typical of Parkinson's disease are |
| **No pattern of atypical parkinsonism** | In a patient with parkinsonism, this favours Parkinson's disease, whose FDG-PET scan is often near normal early on |

## The report

The report is a standalone HTML file with no external dependencies. It shows
the deviation map, left and right scores for all 17 regions, the match and
supporting features for every pattern, and the final conclusion with caveats.
Print it to PDF directly from your browser (**File › Print**).

## Output files

In the output folder, named after each input image:

| File | Description |
| --- | --- |
| `sub-pt01_space-MNI_report.html` | Standalone HTML report |
| `sub-pt01_space-MNI_classification.json` | All numerical outputs from the report (regions, patterns, conclusion) |
| `classification.tsv` | Summary table with one row per image: conclusion and pattern scores |

The deviation map under the report is the one in the output folder of step 2
(`sub-pt01_space-MNI_desc-hn_wmap.nii.gz`, or `_zmap` without a covariate
table): no second copy is written here.

## How the conclusion is reached

![How the classification gets from one image to a conclusion](../classify-flow-en.svg)

1. **Compute a deviation map.** With the method of the first two steps (HN
   by default), the image is converted into a W-score map (or a Z-score map
   without covariates).
2. **Score each region.** Mean map values are extracted across 17 regions
   separately for left and right hemispheres. Scores ≤ −2 count as reduced;
   scores ≥ 2 count as increased.
3. **Cross-check across normalisations.** Any finding that disappears under
   PS (or white-matter normalisation) is flagged. An *increase* only counts if
   it holds under every normalisation, because global scaling makes spared
   regions appear hypermetabolic.
4. **Match against patterns.** Each syndrome is scored from 0 to 100, with
   core features weighted at 60% and supportive features at 40%.

| Pattern | Core features | Supportive features |
| --- | --- | --- |
| MSA-P | Putamen ↓ (posterior first) | Pons ↓, cerebellum ↓ |
| MSA-C | Cerebellum ↓ | Pons ↓, putamen ↓ |
| PSP | Midbrain ↓, medial frontal and anterior cingulate ↓ | Caudate ↓, thalamus ↓, lateral frontal ↓ |
| CBD/CBS | Pericentral and parietal cortex ↓ asymmetrically (clearly worse on one side) | Ipsilateral thalamus ↓, putamen ↓, lateral frontal ↓ |
| DLB | Occipital cortex ↓ | Parietal ↓, precuneus ↓, posterior cingulate relatively spared |
| PD | Putamen, pallidum, thalamus, pons, cerebellum, sensorimotor cortex ↑ | No reduction in striatum, brainstem, or cerebellum |

::: details The same from the command line
```bash
sugar-pet classify patients/*.nii.gz -r hc_reference/ -o reports/ --covariates participants.tsv --lang en
```
:::

## Next

[Step 6: Compare two groups](./compare).
