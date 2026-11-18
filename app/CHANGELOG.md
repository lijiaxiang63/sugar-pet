# Changelog

What changed in SUGAR-PET, the desktop app, from version to version. The
section of a version is what the app's update window shows when it offers
that version, so write it for the people who use the app.

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
