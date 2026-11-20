# Changelog

What changed in SUGAR-PET, the desktop app, from version to version. The
section of a version is what the app's update window shows when it offers
that version, so write it for the people who use the app.

## [Unreleased]

### Added

- **Projects.** The app now works on one project at a time. It opens with a
  page to make a project, open one, or pick one opened lately; the pages of
  the analyses appear once a project is open. A project is a folder, laid
  out as the study folder was, with a file in it, `sugar-pet-project.json`,
  that records the project: its images, what you chose on every page, the
  options of the analyses, and what each step last did. It is written as you
  work; there is nothing to save. Open the project again and every step is
  where you left it, results included.
- A **Data** page, the first of a project: the patients, the healthy
  controls and the covariate table are given there once, and every other
  page takes them from it. The pages no longer have lists of images, tables
  or output folders of their own: each writes into its folder of the
  project.
- Images from outside the project folder either stay where they are (the
  project notes where) or are copied into it, as you choose when you make
  the project and can change on the Data page. What lies in the project
  folder is recorded by where it is in it, so a project that holds its
  images can be moved, or opened on another computer.
- A project says whether its images are in MNI space or still in native
  space. Native images are warped by Spatial Normalisation into the
  `patients` and `controls` folders of the project, and the pages after it
  read them from there. Spatial Normalisation then leads the step bar of the
  individual analysis, and Intensity Normalisation says how many images it
  leaves out because they are not warped yet.
- **Intensity Normalisation normalises the patients and the healthy
  controls of the project together**, the same way, and shows the factors of
  both in one table and one chart, by group. The normalised images are
  written by group, in `normalised/patients` and `normalised/controls`, and
  `factors.tsv` has a `group` column.
- **The reference is now the control statistics, and no step or page of
  its own.** A project needs at least 3 healthy controls of its own, and
  every patient is compared with them: the z-score maps, the classification
  and the group comparison all rest on the same controls. Intensity
  Normalisation summarises them before it normalises the images, and does
  so anew after the controls or the settings of the control statistics
  change. Where only the covariates changed, no factor rests on them: the
  images normalised still stand, and Z-score Maps summarises the controls
  anew before it makes the maps. The statistics always hold those of the
  methods chosen, so a z-score map can be made with each of them. The
  Intensity Normalisation page shows them, with the mean image of the
  controls.
- A step whose files are no longer in the project, or whose images changed
  on disk since it ran, is not shown as done when the project is opened.
- **New Project…**, **Open Project…** and **Close Project** in the File menu
  (macOS). A project folder dropped on the opening page is opened.

### Changed

- The options of the analyses (the settings window, all but *General* and
  SPM and MATLAB) are kept in the project and no longer in the app: a new
  project starts with the defaults. The window shows them while a project is
  open.
- The Study Folder page is gone: making a project lays out its folders. A
  study folder of an earlier version is listed among the projects and is
  made one when you open it: what it holds is kept, the images in
  `patients` and `controls`, its `hc_reference` and a filled-in
  `participants.tsv` are taken into the project.
- One covariate table serves the whole project. It is chosen on the Data
  page, with the columns to adjust for; the control statistics, the W-score maps and
  the group comparison all use it. Compare Groups no longer takes a second
  table.
- Compare Groups compares the patients of the project with its controls;
  **Swap Groups** turns them round. It reads the images Intensity
  Normalisation wrote, by one of the methods that ran with, and normalises
  nothing itself: what is tested is what step 1 shows, made with the
  settings of the project. Before, it normalised every image again with the
  default settings, whatever the project's were. It waits for Intensity
  Normalisation as Z-score Maps does.
- The individual analysis is three steps in a row: Intensity Normalisation,
  then Z-score Maps, then Classify Parkinsonism. A line of steps at the top
  of the three pages shows where you are and which steps are done, and a
  **Next** button beside the run button leads on.
- A later step takes its images, methods, controls and covariate table from
  the step before it: they are shown, not asked for again, and are changed
  where they were chosen. A step waits until the one before it has run, and
  again when what that one ran with was changed.
- Nothing is computed twice along the three steps. Z-score Maps reads the
  normalised images that Intensity Normalisation wrote and no longer
  normalises the original images again; Classify Parkinsonism reads the maps
  that Z-score Maps wrote and computes only the maps of its cross-checks.
  The results are the same as before.
- Classify Parkinsonism no longer cross-checks its findings with another
  normalisation unless asked to: it reads the maps of Z-score Maps alone and
  computes no map itself. **Settings › Classify Parkinsonism › Cross-check
  with another normalisation** turns the check by global scaling (and by
  deep white matter, if the control statistics hold its own) back on; the
  maps for it are then computed during the classification, as before.
- Classifying many patients is several times faster. The regions the
  classification reads were put on the grid of the image anew for every
  patient, which took most of its time; they are now put on a grid once,
  and so are the built-in masks that Intensity Normalisation and Z-score
  Maps use. The results are the same.
- Z-score Maps makes a map with each method of Intensity Normalisation that
  the control statistics hold, and says which it leaves out.
- The controls are summarised with the *Region statistic* and the
  *Smoothing before estimating* of Settings › Intensity Normalisation: the
  factors of the controls are estimated the way those of the patients are.
- When a setting of Intensity Normalisation is changed after the step ran,
  a note in the settings says so, the app tells you when you leave the
  settings window, and Z-score Maps and Classify Parkinsonism wait until
  the step is run again with the new settings. The same holds when the
  control statistics are summarised again in their folder.
- Classify Parkinsonism rests on the maps of Z-score Maps and writes no
  second copy of the map into its own folder. Its method is no longer an
  option in the settings. A `classify_method` kept in the settings is no
  longer used.
- **One primary method.** When Intensity Normalisation runs with several
  methods, its page asks which one is primary (HN unless you say otherwise).
  Classify Parkinsonism and Compare Groups both use it; the other methods
  are for comparison. Their pages no longer have a method of their own to
  choose.

### Removed

- The Reference page, the reference shown in the sidebar, and importing a
  reference built elsewhere. A project that used one imported lets it go
  when opened, and Intensity Normalisation summarises its own controls.
  The `sugar-pet build-reference` command of the Python package is
  unchanged.

- The option **Write normalised images** of Intensity Normalisation: the
  images are always written, as the steps after it read them. A
  `write_images` kept in the settings is no longer used.

## [0.8.0] - 2026-11

### Added

- A Study Folder page, first in the sidebar, creates the folders the guide
  recommends for a study (`raw`, `patients`, `controls`, `hc_reference`,
  `normalised`, `zmaps`, `reports`, `compare`) and a `participants.tsv`
  holding only its header, and says what each is for. What is there
  already is kept. Each folder becomes the output of its page, and the app
  uses the study again at the next launch.
- With a study folder in use, the lists of images offer its `patients`,
  `controls` or `raw` folder with one click, and the pages offer its
  `participants.tsv` as the covariate table.
- The Study Folder page takes an existing study folder in one step:
  **Choose Study Folder…** fills in its location and name, and **Create
  Folders** adds only what is missing.
- Compare Groups can swap its two groups, which turns the sign of the T-map.
- On Masks & Atlas, tick atlas labels and take them to Intensity
  Normalisation as its custom region.
- A list of more than 8 images can be filtered by name.
- When a page lacks a reference, or SPM is not set up, the bar at its bottom
  has a button that leads to where that is mended; pages whose results
  feed another have a button that goes there.

### Changed

- The sidebar groups the pages (prepare, single patients, groups, resources)
  and lists Reference before Intensity Normalisation, as the recommended
  methods need a reference. It shows the study folder in use, and how many
  images are queued on a page that has no tick.
- Each method tile says in a few words what the method does.
- Classify Parkinsonism shows the confidence of a reading as a coloured
  label beside it.
- A list of images from one folder names that folder once, not on every row.
- The Reference and Compare Groups pages read the covariate table again
  when its file changes, so a `participants.tsv` filled in after it was
  chosen is used as it is now.

### Fixed

- A folder that is a link is searched for images where it leads.
- The same image added by two paths, one of them through a link, is listed
  once.

## [0.7.0] - 2026-11

### Added

- A settings window (the gear at the bottom of the sidebar, or ⌘, / Ctrl+,)
  holds the language, the version, and the options of every analysis that
  most runs leave alone: region statistic and target value, smoothing, the
  iterative methods, how a reference is built, the correction of the group
  comparison, and more. They are kept between launches, and each section
  can be put back to its defaults.
- The app updates itself. It checks GitHub for a new version soon after it
  opens and once a day, shows what is new, and installs it; **Check for
  Updates…** in the settings checks at once. Copies installed from the
  `.deb` or `.rpm` package are updated by the package manager instead.
- Results come with pictures. Z-score maps, classifications, group
  comparisons and reference clusters are drawn on axial slices of the
  brain; warped images are drawn with the outline of the template, to check
  the warp; the Reference page shows the mean of the controls.
- Intensity Normalisation charts the factor of every image, one line per
  method, and Classify Parkinsonism shows how well each patient matches
  each pattern.

### Changed

- The pages show only what a run cannot do without: the images, the
  method, the reference and where to write. A line under them says which
  options are in effect, marks those that were changed, and opens the
  settings that hold them.
- SPM and MATLAB are set up once, in the settings, instead of on the
  Spatial Normalisation page.
- The reference regions, of which there are many, stay folded away on the
  method pickers until they are asked for or one is chosen.
- iPS and iHN use the same options on the Intensity Normalisation and
  Z-score Maps pages.

## [0.6.0] - 2026-11

### Changed

- A new look for the whole app: the page on a card, soft panels for groups
  of settings, and a sidebar that marks the steps that are done.

### Added

- A user guide, in English and Chinese, at
  https://lijiaxiang63.github.io/sugar-pet/.
