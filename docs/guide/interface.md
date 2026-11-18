# A tour of the window

Every page in SUGAR-PET shares the same core layout and controls. Once you
know where things live here, the tutorial will simply tell you what to select.

<Shot name="normalize" alt="The parts of the window" :marks="[[1, 216, 112], [2, 222, 830], [3, 140, 874], [4, 163], [5, 492], [6, 257, 862], [7, 'run']]" />

1. **Navigation sidebar.** Lists the pages in the order of a typical workflow,
   from spatial normalisation to group comparison. Click any item to switch
   pages. A tick marks the pages whose last run succeeded, and a spinner the
   one that is running.
2. **Reference in use.** Displays the active healthy-control reference folder.
   Click it to switch to a different reference. This selection is shared across
   all pages.
3. **Settings.** Opens the [Settings window](#the-settings-window), which holds
   the language, the updates, and the options of every analysis. The number
   next to it is the version of the app; it turns into an **Update** badge
   when a newer version is available.
4. **Image list.** Where you load input scans (see [below](#lists-of-images)).
5. **What the run needs.** Tiles to tick, dropdowns, and path selectors
   grouped into cards, with brief helper text under each heading. A page asks
   only for what a run cannot do without: the images, the methods, the
   reference, and where to write.
6. **Status bar.** Shows what is still needed before running (e.g., "Choose
   the output folder."), tracks progress during execution, and reports success
   or errors when finished.
7. **Run button.** Starts the task and turns into **Cancel** while running.
   Only one task can run at a time.

## Lists of images

Input images must be NIfTI files (`.nii` or `.nii.gz`). You can add them in
three ways:

* **Drag and drop** files or entire folders directly onto the list;
* Click **Add Files…** to pick individual images;
* Click **Add Folder…** to import all images in a directory.

The counter next to the heading shows the total number of queued images. Click
**×** on a row to remove a single image, or **Clear** to empty the list. The
parent folder is shown next to each filename so you can tell identically named
files apart.

## Folders and files

Folder and file selectors show **Choose…** when empty. Once a path is set, the
selected name appears with a **×** button to clear it, and the button label
changes to **Change…**.

## Options

Thresholds, smoothing, masks and other parameters that most runs leave alone
are not on the pages: they live in the [Settings window](#the-settings-window),
and their defaults match the original papers. So that nothing is hidden, a
line under the cards of each page says which options are in effect.

<Shot name="normalize-output" alt="The line of options under the cards of a page" :marks="[[1, 391], [2, 1016, 391]]" />

1. **Options in effect.** A one-line summary, here *Mean of the region ·
   target 1 · smoothing automatic*. When an option differs from its default, a
   badge such as **1 changed** appears at the end of the line; hover over it
   to see which.
2. **Options…** opens the Settings window at the section that holds the
   options of this page.

The Z-score Maps page shows this line only while iPS or iHN is ticked (they
are the only methods with options there), and Masks & Atlas has none.

## The Settings window

Click **Settings** at the bottom of the sidebar, press <kbd>⌘</kbd> <kbd>,</kbd>
on macOS or <kbd>Ctrl</kbd> <kbd>,</kbd> on Windows and Linux, or choose
**SUGAR-PET › Settings…** in the macOS menu bar. The settings are a window of
their own, so you can keep it open next to the main one. A change takes
effect at once and is saved: there is no OK button.

<Shot name="settings-general" alt="The Settings window, on its General section" :marks="[[1, 197, 134], [2, 170, 202], [3, 152], [4, 266], [5, 327]]" />

1. **Sections.** *General*, then one section for each analysis with the
   options its page leaves out: **Spatial Normalisation** (SPM, MATLAB and
   the template), **Intensity Normalisation** (including the options of iPS
   and iHN, which the Z-score Maps page shares), **Reference**, **Classify
   Parkinsonism**, **Compare Groups**, and **Reference Cluster**. The
   tutorial describes each section in the step that uses it.
2. **A dot** marks a section in which an option differs from its default. The
   bottom of such a section names the changed options, and **Restore
   Defaults** puts that section back as it was.
3. **Language of the app.** English or 中文; the whole app switches at once.
4. **Version and updates.** The version you are running, when the app last
   looked for a newer one, and **Check for Updates…** to look right now.
5. **Automatic updates.** Whether the app checks by itself, and whether it
   also downloads and installs what it finds.

### Updates {#updates}

From the version after 0.6.0 on, the app updates itself from its
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

Your chosen language, last visited page, active reference, output folder,
MATLAB/SPM paths, and every option of the Settings window are saved across
sessions. Image lists are cleared on exit.

## Next

[Prepare your data](./prepare-data).
