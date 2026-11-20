---
layout: home
title: SUGAR-PET
titleTemplate: Intensity normalisation for brain FDG-PET

hero:
  name: SUGAR-PET
  text: Intensity normalisation for brain FDG-PET
  tagline: A lightweight desktop app for macOS, Windows, and Linux. No Python or complex setup required—follow the tutorial from raw images to final results.
  image:
    light: /screenshots/en/normalize.png
    dark: /screenshots/en/normalize-dark.png
    alt: The Intensity Normalisation page of SUGAR-PET
  actions:
    - theme: brand
      text: Start the tutorial
      link: /tutorial/
    - theme: alt
      text: Install the app
      link: /guide/install
    - theme: alt
      text: Download
      link: https://github.com/lijiaxiang63/sugar-pet/releases/latest

features:
  - title: Thirteen normalisation methods
    details: Reference regions, proportional scaling (PS), and the histogram-based methods HN, iPS, and iHN from López-González et al. (2020).
    link: /reference/methods
    linkText: Choosing a method
  - title: Built from your own controls
    details: Every project summarises its own healthy controls, and every patient, map and comparison rests on them—optionally adjusting for age and sex.
    link: /tutorial/normalize
    linkText: Normalise the intensity
  - title: Z-score and W-score maps
    details: Quantify, voxel by voxel, how far a patient's uptake deviates from healthy controls.
    link: /tutorial/zmap
    linkText: Make the maps
  - title: Parkinsonism classification
    details: Compare a patient's metabolic pattern against MSA, PSP, CBD, DLB, and PD, and export a printable report.
    link: /tutorial/classify
    linkText: Classify a patient
  - title: Quick classification
    details: Classify one more patient against the controls of a project, straight from native space, without adding the scan to the project.
    link: /tutorial/quick
    linkText: Classify one scan
  - title: Group comparisons
    details: Voxel-wise two-sample t-tests of patients against controls, with GRF or FDR correction and cluster tables.
    link: /tutorial/compare
    linkText: Compare two groups
  - title: Full CLI interoperability
    details: The desktop app reads and writes the exact same files as the sugar-pet Python package, so you can mix both workflows freely.
    link: /reference/cli
    linkText: Command line
---
