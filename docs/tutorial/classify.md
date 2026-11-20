# Classify parkinsonism

**Page: Classify Parkinsonism** · step 3 of the individual analysis

For a patient whose diagnosis is uncertain, this step finds the regions where
uptake is lower or higher than in the healthy controls, scores how well that
pattern matches the main parkinsonian syndromes, and writes a report.

::: warning Decision support, not a diagnosis
Current diagnostic criteria (MSA 2022, PSP 2017, DLB 2017) treat FDG-PET as
supportive evidence only. The rules here come from the literature and were
neither fitted to nor validated on your patients. Always read the report with
the clinical findings, structural MRI and dopamine-transporter imaging.
:::

## Before you start

* Steps [1](./normalize) and [2](./zmap) are done: the classification reads
  the maps step 2 made.
* For single patients, 30 or more controls are recommended, scanned under the
  same conditions (eyes open or closed) as the patients.
* Where you can, give the project a **covariate table**, so that the scores
  are adjusted for age and sex, and tick **White matter** under *Control
  statistics of other methods* in the [settings](./normalize#control-statistics-options),
  so that the cross-check has a normalisation that does not rest on gray
  matter.

## Run it

The step reads the deviation map that step 2 made of each patient and asks
for nothing again; nothing is normalised or mapped a second time. While step
2 has not run, or something before it changed since, the page says so and
**Classify** stays disabled.

<Shot name="classify" alt="The Classify Parkinsonism page" :marks="[[1, 109], [2, 269], [3, 310], [4, 351], [5, 392], [6, 526], [7, 596], [8, 'run']]" />

1. **The line of steps:** a tick means a step is done.
2. **Images:** the patients step 2 made maps of.
3. **Method:** the primary method of step 1, which the classification rests
   on, and the normalisations it is checked with when the cross-check is on.
   Keep **Histogram (HN)**: it assumes nothing about which regions are
   spared, which suits undiagnosed cases.
4. **Healthy controls:** how many; the page recommends 30 or more.
5. **Covariate table:** whether the scores are adjusted for age and sex. To
   change any of these, **Change in Step 2**.
6. **Written to:** the `reports` folder of the project.
7. **Options in effect:** the language of the report and the cross-check.
   **Options…** opens **Settings › Classify Parkinsonism** (below).
8. Click **Classify**.

### The options {#options}

<Shot name="settings-classify" alt="Settings › Classify Parkinsonism" />

* **Cross-check with another normalisation:** off, the classification reads
  the maps of step 2 alone and computes nothing again. On, every finding is
  also read on maps made by global scaling (PS) and, when the control
  statistics hold those of deep white matter, by white matter; the maps step
  2 did not make are computed here, which takes longer. A finding the other
  normalisations do not show is marked in the report and counts less towards
  a pattern.
* **Language of the report:** as the app, English, or Chinese.

## Check the results

<Shot name="classify-results" alt="The results of the classification" :marks="[[9, 240], [10, 940, 240]]" />

9. **A row per patient:** the conclusion in bold, with its confidence when it
   names a pattern, and under it the image and how the conclusion was
   reached. The bars beside it score each of the six patterns (MSA-P, MSA-C,
   PSP, CBD, DLB, PD) from 0 to 100; the leading one is in colour. Two bars
   of about the same height, as for the second patient, mean the two
   syndromes cannot be told apart.
10. **Open Report** shows that patient's report in the browser.

Scroll to the end of the page.

<Shot name="classify-figure" alt="The deviation map of one patient" :marks="[[11, 742], [12, 268, 782]]" />

11. **The picture** draws the map of the chosen patient on axial slices of
    their image: blue to cyan where uptake is lower than in the controls,
    red to yellow where it is higher. Click a row to draw that patient.
12. **Show in Finder** opens the `reports` folder.

## Reading the conclusion

| Conclusion | Meaning |
| --- | --- |
| **A pattern**, e.g. *Multiple system atrophy, parkinsonian type (MSA-P)* | One pattern scores at least 50 of 100 and leads the next by at least 15. Confidence is *high* for 70 or more with a lead of 30 or more, otherwise *moderate*; it drops to *low* if the findings it rests on do not hold under every normalisation checked |
| ***A* or *B*** | Two syndromes score close together; both are named |
| **Partial features of…** | The highest score lies between 30 and 50: suggestive, but not enough for a pattern |
| **PD pattern** | No pattern of atypical parkinsonism, but the relative increases typical of Parkinson's disease |
| **No pattern of atypical parkinsonism** | In a patient with parkinsonism, this favours Parkinson's disease, whose scan is often near normal early on |

## The report

The report is a single HTML file that needs nothing else. It shows the
deviation map, the left and right scores of 17 regions, the match and the
supporting findings of every pattern, and the conclusion with its caveats.
Print it to PDF from the browser (**File › Print**).

## Output files

In `reports/`, named after each image:

| File | What it holds |
| --- | --- |
| `sub-pt01_space-MNI_report.html` | The report |
| `sub-pt01_space-MNI_classification.json` | Every number of the report: regions, patterns, conclusion |
| `classification.tsv` | A row per image: the conclusion and the scores of the patterns |

The map under the report is the one in `zmaps/`, written by step 2: no copy
of it is written here.

## How the conclusion is reached

![How the classification gets from one image to a conclusion](../classify-flow-en.svg)

1. **The deviation map.** The map step 2 made with the primary method (HN by
   default): W-scores, or Z-scores without covariates.
2. **Score each region.** The mean of the map over 17 regions, each side on
   its own. A score of −2 or less counts as reduced, +2 or more as increased.
3. **Check across normalisations** (with the cross-check on). A finding that
   goes away under PS, or white matter, is flagged. An *increase* counts only
   if it holds under every normalisation, because global scaling makes spared
   regions look hypermetabolic.
4. **Match the patterns.** Each syndrome is scored from 0 to 100, the core
   features weighing 60% and the supportive ones 40%.

| Pattern | Core features | Supportive features |
| --- | --- | --- |
| MSA-P | Putamen ↓ (posterior first) | Pons ↓, cerebellum ↓ |
| MSA-C | Cerebellum ↓ | Pons ↓, putamen ↓ |
| PSP | Midbrain ↓, medial frontal and anterior cingulate ↓ | Caudate ↓, thalamus ↓, lateral frontal ↓ |
| CBD/CBS | Pericentral and parietal cortex ↓ asymmetrically (clearly worse on one side) | Thalamus ↓, putamen ↓, lateral frontal ↓ on the same side |
| DLB | Occipital cortex ↓ | Parietal ↓, precuneus ↓, posterior cingulate relatively spared |
| PD | Putamen, pallidum, thalamus, pons, cerebellum, sensorimotor cortex ↑ | No reduction in striatum, brainstem or cerebellum |

::: details The same from the command line
```bash
sugar-pet classify patients/*.nii.gz -r hc_reference/ -o reports/ --covariates participants.tsv --lang en
```
:::

## Next

[Quick classification](./quick) of a patient outside the project, or
[Compare two groups](./compare).
