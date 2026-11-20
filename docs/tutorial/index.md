# Tutorial: from raw images to results

This tutorial walks through a complete analysis in SUGAR-PET, one page at a
time. Each chapter explains what to select, shows the window with numbered
callouts, and lists the files the page writes.

## The example study

Suppose Ada is studying patients with atypical parkinsonism. She has:

* **6 patients** and **16 healthy controls**, one static FDG-PET scan each,
  acquired on the same scanner and still in native scanner space;
* `participants.tsv`, recording the age and sex of all 22 subjects.

She wants a deviation map of each patient's abnormal regions, a
classification report saying which parkinsonian syndrome each scan resembles,
and a group-level map showing where the patients differ from the controls.
Later, a new patient comes to her clinic, and she wants that scan classified
against the same controls.

## The chapters

<div class="flow">
  <a href="./project"><span class="n">1</span><span class="t">Make a project</span><span class="d">One folder for the images and everything the analyses write. Add the patients, at least 3 healthy controls and the table of ages and sexes.</span></a>
  <a href="./spatial"><span class="n">2 · only for native space</span><span class="t">Warp to MNI space</span><span class="d">Only if the images are still in native scanner space. Needs MATLAB and SPM.</span></a>
  <a href="./normalize"><span class="n">3 · step 1</span><span class="t">Normalise the intensity</span><span class="d">Summarise the controls, then divide every image by its factor.</span></a>
  <a href="./zmap"><span class="n">4 · step 2</span><span class="t">Z-score and W-score maps</span><span class="d">How far each voxel of a patient is from the controls.</span></a>
  <a href="./classify"><span class="n">5 · step 3</span><span class="t">Classify parkinsonism</span><span class="d">Match each patient's map against the typical syndromes, with a report.</span></a>
  <a href="./quick"><span class="n">6</span><span class="t">Quick classification</span><span class="d">One more patient, classified against the project's controls without joining it.</span></a>
  <a href="./compare"><span class="n">7</span><span class="t">Compare two groups</span><span class="d">Voxel-wise two-sample t-test of the patients against the controls.</span></a>
  <a href="./reference-cluster"><span class="n">8 · advanced</span><span class="t">Reference cluster</span><span class="d">A data-driven reference region, for a sensitivity analysis.</span></a>
  <a href="./masks"><span class="n">Resource</span><span class="t">Masks and atlas</span><span class="d">The built-in regions and the atlas labels.</span></a>
</div>

## Step 1, 2 and 3

Intensity Normalisation, Z-score Maps and Classify Parkinsonism are the three
steps of the **individual analysis**, and the app numbers them so: *step 1*,
*step 2* and *step 3* in this tutorial mean these three pages, not its
chapters. Each step reads the files the step before it wrote, shows what it
takes from it without asking again, and waits until that step has run.

Compare Groups and Reference Cluster rest on step 1 as well: the group
comparison tests the images step 1 normalised, and both compare with the
controls step 1 summarised. Quick Classification goes through all three
steps for a patient outside the project.

## Every patient against the project's controls

A project always has its own healthy controls, at least 3 of them. Step 1
summarises them into the **control statistics**, the mean and spread of every
voxel, and every patient is compared with them: the maps, the
classification, the group comparison and the quick classification all rest
on the same controls. There is nothing to build or import by hand; when the
controls or the settings change, the next run summarises them anew.

## You don't need every chapter

| Goal | Chapters |
| --- | --- |
| Normalised images for other statistics software | [Project](./project), then [step 1](./normalize) |
| Map where each patient is abnormal | [Project](./project), [steps 1](./normalize) and [2](./zmap) |
| Classification reports for the patients | [Project](./project), [steps 1](./normalize), [2](./zmap) and [3](./classify) |
| Classify one more patient later | All of the above once, then [Quick classification](./quick) for each new scan |
| Compare the patients against the controls | [Project](./project), [step 1](./normalize), then [Compare two groups](./compare) |

Images in native space go through [Warp to MNI space](./spatial) first, in
every row.

## Before you start

* The app is [installed](../guide/install) and you have seen
  [the window](../guide/interface).
* Your data are [prepared](../guide/prepare-data).

Start with [Make a project](./project).
