# A tour of the window

Every page in SUGAR-PET shares the same core layout and controls. Once you
know where things live here, the tutorial will simply tell you what to select.

<Shot name="normalize" alt="The parts of the window" :marks="[[1, 216, 112], [2, 222, 831], [3, 222, 874], [4, 163], [5, 492], [6, 257, 862], [7, 'run']]" />

1. **Navigation sidebar.** Lists the pages in the order of a typical workflow,
   from spatial normalisation to group comparison. Click any item to switch
   pages. A tick marks the pages whose last run succeeded, and a spinner the
   one that is running.
2. **Reference in use.** Displays the active healthy-control reference folder.
   Click it to switch to a different reference. This selection is shared across
   all pages.
3. **Language toggle.** Switch between English and Chinese.
4. **Image list.** Where you load input scans (see [below](#lists-of-images)).
5. **Settings.** Tiles to tick, dropdowns, and path selectors grouped into
   cards, with brief helper text under each heading.
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

Less frequently used parameters are tucked under **Options**—click to expand
them. The defaults match the original papers, so you can leave them unchanged
on a first run.

## Results

When a task finishes, the results appear at the bottom of the page and the
window scrolls down automatically. You will see a summary table alongside
action buttons such as **Show in Finder** (or your system file manager) to
open the output files. If any image fails, the rest still complete, and the
reason for each failure is noted below the table.

## What the app remembers

Your chosen language, last visited page, active reference, output folder, and
MATLAB/SPM paths are saved across sessions. Image lists are cleared on exit.

## Next

[Prepare your data](./prepare-data).
