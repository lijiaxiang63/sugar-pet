# Changelog

What changed in SUGAR-PET, the desktop app, from version to version. The
section of a version is what the app's update window shows when it offers
that version, so write it for the people who use the app.

## [Unreleased]

### Added

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
  images wait on a page that has not run.
- Each method tile says in a few words what the method does.
- Classify Parkinsonism shows the confidence of a reading as a coloured
  label beside it.
- A list of images from one folder names that folder once, not on every row.

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
