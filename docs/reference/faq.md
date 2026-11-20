# Questions and problems

## Starting the app

### macOS says the app cannot be opened

The macOS build is ad-hoc signed, not notarised by Apple. Right-click the app
and choose **Open**. If macOS still blocks it, open **System Settings ›
Privacy & Security**, find the message about SUGAR-PET, and click **Open
Anyway**. You only need to do this once.

### Windows warns about an unknown publisher

Click **More info**, then **Run anyway**.

### How do I change the language?

Click **Settings** at the bottom of the sidebar (or press <kbd>⌘</kbd>
<kbd>,</kbd> / <kbd>Ctrl</kbd> <kbd>,</kbd>) and choose under **General ›
Language of the app**. The language of the classification reports can be set
apart, under **Settings › Classify Parkinsonism**.

### How do I update the app?

From version 0.7.0 on, the app updates itself: it checks GitHub
soon after it opens and once a day, shows what is new, and installs the new
version when you agree. To check right now, open **Settings › General** and
click **Check for Updates…**; the two switches below it turn the automatic
check and the automatic download on or off.

Two cases need a manual update: copies installed from the `.deb` or `.rpm`
package (install the newer package from the
[releases page](https://github.com/lijiaxiang63/sugar-pet/releases) with your
package manager), and version 0.6.0 or earlier, which has no updater
(download and install the newer version once).

## Images and files

### Which file formats are supported?

NIfTI images (`.nii` or `.nii.gz`). DICOM files must be converted first—for
example, using [dcm2niix](https://github.com/rordenlab/dcm2niix).

### Must the images be in MNI space?

Every analysis reads images in MNI space: the built-in masks and atlas are
defined there, and an unwarped image samples the wrong regions without any
error. Images straight from the scanner can still be used: set *The images
are in* to **Native Space** on the **Data** page, and
[Spatial Normalisation](../tutorial/spatial) warps them before any other page
reads them.

### "… and … would both be written as …"

Output filenames are derived from input filenames. Two inputs with the same
name in different folders (`a/pet.nii.gz`, `b/pet.nii.gz`) would overwrite
each other, so Spatial Normalisation and Classify Parkinsonism refuse to run
before writing anything. Give each image a unique filename (such as
`sub-01_pet.nii.gz`), or run them in separate batches into different output
folders.

### My images are already smoothed

Set the smoothing to `0` in the settings, in each section that has one:
**Settings › Control Statistics** (*Smoothing (FWHM)*),
**Settings › Intensity Normalisation** (*Smoothing before estimating*, with
*Automatic* unticked), and **Settings › Compare Groups** (*Smoothing before
the test*, with *Automatic* unticked). **Options…** on a page opens its
section.

## The controls

### How many controls do I need?

At least 3: the app runs nothing with fewer. Voxel-wise statistics become
unreliable below roughly 10 controls; 20 or more are recommended in general,
and 30 or more for single-patient classification.

### Can I use control statistics built elsewhere?

No. Every project summarises its own controls, so the normalisation, the
maps, the classification and the group comparison all rest on the same
controls. To use controls of another study, add their images to the project
on the **Data** page.

### Can I add controls later?

Yes. Add them on the **Data** page and run
[step 1](../tutorial/normalize) again: it summarises all the controls anew.
The pages after it say that they need to run again.

### Does a scan on Quick Classification join the project?

No. [Quick Classification](../tutorial/quick) reads the project's control
statistics and settings, but the scan is not added to the patients, no step
of the project changes, and its files go to the project's `quick/` folder
only.

### Can patients and controls come from different scanners?

Avoid this if possible. Scanner and reconstruction differences can mimic
group differences. If you have no alternative, state this clearly in your
paper and interpret the results cautiously.

## Methods and maps

### A method is grayed out on the Z-score Maps page

Mapping with a reference region needs control statistics of that region.
Tick it under **Control statistics of other methods** in **Settings ›
Control Statistics** ([step 1](../tutorial/normalize#control-statistics-options))
and run step 1 again. PS, iPS, HN, and iHN are always available.

### Only plain Z-scores are available

The project has no covariate table. Choose one on the **Data** page
([Age and sex](../tutorial/project#age-and-sex)) and run step 1 again to
enable W-scores.

### A warning says the patient's age is outside the control range

That patient's W-scores are extrapolated beyond the age range of your
controls. Interpret them carefully, or add controls covering that age range.

### HN and iHN give almost the same factor. Is that a bug?

No—this is expected. The hypometabolic voxels excluded by iHN lie in the tail
of the ratio histogram, far from the peak that HN estimates, so removing them
barely shifts the mode.

### Regions look hypermetabolic after PS

When hypometabolism is widespread, the global mean drops, making the factor
too small and causing spared regions to appear elevated relative to controls.
Switch to HN or iPS instead.

### The result changes with the normalisation method

That means the finding is sensitive to the choice of normalisation and is
not robust—which is itself important to know and report transparently. See
[Check that the result is robust](../tutorial/compare#check-that-the-result-is-robust).

## Spatial normalisation

### MATLAB was not found

Open **Settings › Spatial Normalisation** (**Set Up…** or **Change…** in the
*SPM and MATLAB* row of the Spatial Normalisation page opens it), click
**Choose…** in the **MATLAB** row, and point to the program manually: on
macOS, select the MATLAB application (`MATLAB_R20xx.app`); on Windows and
Linux, select `matlab` inside the `bin` folder of your MATLAB installation.

### Which SPM folder should I select?

Select the SPM12 or SPM25 directory that directly contains `spm.m`, under
**Settings › Spatial Normalisation › SPM folder**.

### A warped image looks wrong

PET-only registration is less robust than MRI-guided registration. Try
supplying your own template (the mean of your warped controls) under
**Settings › Spatial Normalisation › PET template**, or warp the scan with
another tool. Never include a poorly
registered image in downstream analyses.

## Results

### What should I report in a paper?

Report the normalisation method (e.g. HN), the number, age range, and sex
breakdown of the controls, the smoothing kernel (8 mm by default), the
multiple-comparison correction parameters, and the SUGAR-PET version.

### Can I use the classification for clinical diagnosis?

No. It is a research decision aid only. Diagnostic criteria treat FDG-PET as
supportive evidence, and these rules have not been validated on your cohort.

### Where can I find the version number?

Next to **Settings** at the bottom of the sidebar, and in **Settings ›
General**. The versions that exist are listed on the
[releases page](https://github.com/lijiaxiang63/sugar-pet/releases).

## Something else

Open an issue on [GitHub](https://github.com/lijiaxiang63/sugar-pet/issues)
describing what you did, what you expected, and what happened instead.
