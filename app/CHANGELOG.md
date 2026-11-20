# Changelog

What changed in SUGAR-PET, the desktop app, from version to version. The
section of a version is what the app's update window shows when it offers
that version, so write it for the people who use the app.

## [Unreleased]

### Changed

- The individual analysis is three steps in a row: Intensity Normalisation,
  then Z-score Maps, then Classify Parkinsonism. A line of steps at the top
  of the three pages shows where you are and which steps are done, and a
  **Next** button beside the run button leads on.
- A later step takes its images, methods, reference and covariate table from
  the step before it: they are shown, not asked for again, and are changed
  where they were chosen. A step waits until the one before it has run, and
  again when what that one ran with was changed.
- Nothing is computed twice along the three steps. Z-score Maps reads the
  normalised images that Intensity Normalisation wrote and no longer
  normalises the original images again; Classify Parkinsonism reads the maps
  that Z-score Maps wrote and computes only the maps of its cross-checks.
  The results are the same as before.
- Z-score Maps makes a map with each method of Intensity Normalisation that
  the reference holds control statistics for, and says which it leaves out.
- A reference is built with the *Region statistic* and the *Smoothing before
  estimating* of Settings › Intensity Normalisation, and records them: the
  factors of its controls are estimated the way those of your images are.
  Z-score Maps asks that step 1 and the reference agree in the two, and
  when they do not it offers both ways out: change the settings and run
  step 1 again, or build the reference again. References built before were
  all built with the mean and automatic smoothing, and are read so.
- When a setting of Intensity Normalisation is changed after the step ran,
  a note in the settings says so, the app tells you when you leave the
  settings window, and Z-score Maps and Classify Parkinsonism wait until
  the step is run again with the new settings. The same holds when the
  reference is built again in its folder.
- Classify Parkinsonism rests on the maps of Z-score Maps and writes no
  second copy of the map into its own folder. Its method is no longer an
  option in the settings; when the maps were made with several methods, the
  page asks which one to classify by (HN unless you say otherwise). A
  `classify_method` kept in the settings is no longer used.
- Compare Groups starts with the method chosen in Intensity Normalisation and
  with the covariate table the reference was built with, until others are
  chosen on its page.

### Removed

- The option **Write normalised images** of Intensity Normalisation: the
  images are always written, as the steps after it read them. A
  `write_images` kept in the settings is no longer used.

### Added

- Spatial Normalisation hands its images on to the Reference page as well:
  **Use to Build a Reference**, for controls.

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
