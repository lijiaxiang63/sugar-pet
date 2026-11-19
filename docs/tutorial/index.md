# Tutorial: from raw images to results

This tutorial walks through a complete analysis in SUGAR-PET, one page at a
time. Each step explains what to select, shows the window with numbered
callouts, and lists the output files produced.

## The example study

Suppose Ada is studying patients with atypical parkinsonism. She has:

* **12 patients** and **16 healthy controls**, one static FDG-PET scan each,
  acquired on the same scanner;
* `participants.tsv`, recording the age and sex of all 28 subjects;
* the [folder layout](../guide/prepare-data#a-folder-layout-that-works) from
  the previous page, stored under `study/`.

She wants three outputs: a deviation map of each patient's abnormal regions, a
classification report indicating which parkinsonian syndrome each scan
resembles, and a group-level map showing where patients differ from controls.

## The steps

<div class="flow">
  <a href="./spatial"><span class="n">Step 1 · optional</span><span class="t">Warp to MNI space</span><span class="d">Only if images are still in native scanner space. Requires MATLAB and SPM.</span></a>
  <a href="./build-reference"><span class="n">Step 2</span><span class="t">Build the reference</span><span class="d">Summarise the healthy controls once. Most later steps rely on it.</span></a>
  <a href="./normalize"><span class="n">Step 3</span><span class="t">Normalise the intensity</span><span class="d">Divide each image by its factor; export normalised scans and a factor table.</span></a>
  <a href="./zmap"><span class="n">Step 4</span><span class="t">Z-score and W-score maps</span><span class="d">Quantify how far each voxel of a patient deviates from controls.</span></a>
  <a href="./classify"><span class="n">Step 5</span><span class="t">Classify parkinsonism</span><span class="d">Match a patient's pattern against typical syndromes and generate a report.</span></a>
  <a href="./compare"><span class="n">Step 6</span><span class="t">Compare two groups</span><span class="d">Voxel-wise two-sample t-test of patients vs. controls, with correction.</span></a>
  <a href="./reference-cluster"><span class="n">Step 7 · advanced</span><span class="t">Reference cluster</span><span class="d">Derive a data-driven reference region for sensitivity analysis.</span></a>
  <a href="./masks"><span class="n">Resource</span><span class="t">Masks and atlas</span><span class="d">Browse or export the built-in regions and atlas labels.</span></a>
</div>

::: tip Do you need a reference?
The tutorial follows the sidebar: **Reference** comes before **Intensity
Normalisation**, because the recommended method, HN, relies on a reference.
The simplest methods (such as a cerebellar reference region) do not require
controls: with those you can skip building one.
:::

## You don't need every step

Steps 3 to 7 are independent of one another—pick only the ones your study
needs:

| Goal | Workflow |
| --- | --- |
| Normalised images for external statistics software | Step 2, then [Step 3](./normalize) |
| Same, using a simple reference region with no controls | [Step 3](./normalize) alone |
| Map where a single patient is abnormal | Step 2, then [Step 4](./zmap) |
| Generate a parkinsonism classification report | Step 2, then [Step 5](./classify) |
| Compare a patient group against controls | Step 2, then [Step 6](./compare) |

Steps 4, 5, and 6 handle normalisation internally, so you do not need to run
Step 3 beforehand.

## Before you start

* The app is [installed](../guide/install) and you are familiar with
  [the interface](../guide/interface).
* Your data are [prepared](../guide/prepare-data).

Start with [Step 1](./spatial), or jump straight to
[Step 2](./build-reference) if your images are already in MNI space.
