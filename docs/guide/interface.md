# A tour of the window

Every page in SUGAR-PET shares the same core layout and controls. Once you
know where things live here, the tutorial will simply tell you what to select.

<Shot name="normalize" alt="The parts of the window" :marks="[[1, 216, 116], [2, 216, 185], [3, 216, 874], [4, 109], [5, 871, 351], [6, 474, 861], [7, 734, 861], [8, 913, 861]]" />

1. **The project.** The name of the open [project](../tutorial/project) and
   how many patients and controls it holds. Click it for the Data page.
2. **Navigation sidebar.** The pages in the order of an analysis, in groups:
   the project (Data, and Spatial Normalisation for images in native space),
   the individual analysis, the group analysis, Quick Classification outside
   the project, and Masks & Atlas. A tick marks the pages whose last run
   succeeded and still rests on what came before, a spinner the one that is
   running, and a number how many images a page has still to run on.
3. **Settings.** Opens the [Settings window](#the-settings-window), which holds
   the language, the updates, and the options of every analysis. The number
   next to it is the version of the app; it turns into an **Update** badge
   when a newer version is available.
4. **The line of steps.** The three pages of the **individual analysis**
   (Intensity Normalisation, Z-score Maps, Classify Parkinsonism) are steps
   in a row, with Spatial Normalisation before them for images in native
   space. A tick marks a step that is done, and a click goes to a step.
5. **What a page reads.** The images are added on the Data page only; the
   other pages show them, read-only, with a button that leads to where they
   are changed, here **Change in Data**. A later step likewise shows the
   methods of the step before it and does not ask for them again.
6. **Status bar.** Shows what is still needed before running (for example
   that the project needs at least 3 controls), tracks progress during
   execution, and reports success or errors when finished.
7. **Run button.** Starts the task and turns into **Cancel** while running.
   Only one task can run at a time.
8. **Next.** Once a step is done, a button beside the run button leads to
   the next page.

When something a page rests on has changed since it last ran (the images, an
option, an earlier step), its results are kept but the page says so, its tick
in the sidebar goes away, and the status bar names the page to run again.

## Lists of images

Images are added on the [Data page](../tutorial/project#the-data-page), the
only one with lists to fill. Input images must be NIfTI files (`.nii` or
`.nii.gz`). You can add them in three ways:

* **Drag and drop** files or entire folders directly onto the list;
* Click **Add Files…** to pick individual images;
* Click **Add Folder…** to import all images in a directory.

When the project's own `patients/` or `controls/` folder holds images, a
fourth button, such as **Load patients/**, adds them in one click.

The counter next to the heading shows the number of images. Click **×** on a
row to remove a single image, or **Clear** to empty the list; neither deletes
a file. When all images come from one folder, that folder is named once, in
the heading; otherwise the parent folder is shown next to each filename so
you can tell identically named files apart.

A list of more than 8 images has a **filter** box above it. Filtering changes
only which rows are shown: the run, the counter and **Clear** still act on the
whole list.

## Folders and files

Folder and file selectors show **Choose…** when empty. Once a path is set, the
selected name appears with a **×** button to clear it, and the button label
changes to **Change…**.

## Options

Thresholds, smoothing, masks and other parameters that most runs leave alone
are not on the pages: they live in the [Settings window](#the-settings-window),
and their defaults match the original papers. So that nothing is hidden, a
line under the cards of each page says which options are in effect.

<Shot name="normalize-output" alt="A line of options on a page" :marks="[[1, 500], [2, 880, 500], [3, 961, 500]]" />

1. **Options in effect.** A one-line summary, here that of the control
   statistics: the mask, the smoothing, and the statistics of further
   methods kept.
2. **A badge** such as **1 changed** appears at the end of the line when an
   option differs from its default; hover over it to see which.
3. **Options…** opens the Settings window at the section that holds these
   options.

The Data page and Masks & Atlas have no options.

## The Settings window

Click **Settings** at the bottom of the sidebar, press <kbd>⌘</kbd> <kbd>,</kbd>
on macOS or <kbd>Ctrl</kbd> <kbd>,</kbd> on Windows and Linux, or choose
**SUGAR-PET › Settings…** in the macOS menu bar. The settings are a window of
their own, so you can keep it open next to the main one. A change takes
effect at once and is saved: there is no OK button.

<Shot name="settings-general" alt="The Settings window, on its General section" :marks="[[1, 196, 68], [2, 143], [3, 256], [4, 318]]" />

1. **Sections.** *General*, then one section for each analysis with the
   options its page leaves out: **Spatial Normalisation** (SPM, MATLAB and
   the template), **Intensity Normalisation** (including the options of iPS
   and iHN), **Control Statistics** (how the controls are summarised),
   **Classify Parkinsonism**, **Compare Groups**, and **Reference Cluster**.
   The tutorial describes each section in the step that uses it. A dot marks
   a section in which an option differs from its default; the bottom of such
   a section names the changed options, and **Restore Defaults** puts that
   section back as it was.
2. **Language of the app.** English or 中文; the whole app switches at once.
3. **Version and updates.** The version you are running, when the app last
   looked for a newer one, and **Check for Updates…** to look right now.
4. **Automatic updates.** Whether the app checks by itself, and whether it
   also downloads and installs what it finds.

### Updates {#updates}

From version 0.7.0 on, the app updates itself from its
[GitHub releases](https://github.com/lijiaxiang63/sugar-pet/releases). It
checks soon after it opens and then once a day. When there is a newer
version, a window shows what is new and offers to install it, and
**Settings** in the sidebar carries an **Update** badge until you do. With
*Download and install automatically* switched on, new versions are downloaded
in the background and take effect the next time the app opens.

Copies installed from the `.deb` or `.rpm` package are updated by the
system's package manager instead; there the General section links to the
releases page. Version 0.6.0 and earlier cannot update themselves:
[install](./install) the newer version over them once.

## Results

When a task finishes, the results appear at the bottom of the page and the
window scrolls down automatically. Most pages draw a picture first: axial
slices of the brain with the result on them (a warped image inside the
outline of the template, a z-score map, a T-map, a reference cluster), a
chart of the normalisation factors, or bars of pattern scores. Under a
picture, **R** and **L** are the patient's right and left sides. When a run
covers several images, the arrows above the picture, or a click on a row of
the table, choose which one is drawn.

Below the picture you will see a summary table alongside action buttons such
as **Show in Finder** (or your system file manager) to open the output files.
If any image fails, the rest still complete, and the reason for each failure
is noted below the table.

## What the app remembers

The app keeps your language, the projects opened recently, and the paths of
MATLAB and SPM. Everything else belongs to a project and is kept in its file,
`sugar-pet-project.json`: the images, what you chose on every page, the
options of the Settings window, and the results of every step. Open a
project again and it is where you left it.

## Next

[Prepare your data](./prepare-data).
