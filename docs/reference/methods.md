# Choosing a method

Every intensity normalisation method divides the image by a single scalar.
They differ only in how that number—the normalisation factor—is estimated.

## The methods

| In the app | Command line `-m` | How the factor is estimated | Needs the reference |
| --- | --- | --- | :---: |
| **Histogram (HN)** | `hn` | Divides the image by the control mean template and takes the *most frequent* ratio | yes |
| Iterative HN (iHN) | `ihn` | Runs HN, excludes hypometabolic voxels, and re-estimates HN | yes |
| Iterative PS (iPS) | `ips` | Runs PS, excludes hypometabolic voxels, and re-estimates PS | yes |
| Proportional scaling (PS) | `ps` | Mean uptake inside the gray-matter mask | no |
| SPM global mean | `spm-global` | Mean of all voxels above 1/8 of the image mean | no |
| Whole brain | `whole-brain` | Mean uptake across gray and white matter | no |
| Cerebellum | `cerebellum` | Mean uptake of the whole cerebellum | no |
| Cerebellar gray matter | `cerebellar-gm` | Mean uptake of cerebellar gray matter | no |
| Pons | `pons` | Mean uptake of the central pons | no |
| White matter | `white-matter` | Mean uptake of deep cerebral white matter | no |
| Occipital cortex | `occipital` | Mean uptake of the occipital cortex | no |
| Sensorimotor cortex | `sensorimotor` | Mean uptake of primary sensorimotor cortex | no |
| Custom region | `region` | Mean uptake of a custom region (built-in mask, NIfTI file, or atlas labels) | no |

## Which one should I use?

Following [López-González et al. (2020)](https://doi.org/10.1016/j.neuroimage.2020.117229):

* **HN is a strong default.** It makes no prior assumption about which regions
  are spared and performed best among data-driven methods. iHN yields nearly
  identical factors.
* **PS is biased when hypometabolism is extensive.** Widespread lesions pull
  the global mean down, making the factor too small and causing spared
  tissue to appear *hyper*metabolic. iPS removes most of this bias.
* **Reference-region methods depend on their underlying assumption:** the
  chosen region must be large enough and genuinely spared by the disease.
  Small regions like the pons are noisier.

## Which regions are typically spared?

| Disease | Avoid as reference | Usually spared |
| --- | --- | --- |
| Alzheimer's disease | — | Cerebellum, pons, sensorimotor cortex |
| Multiple system atrophy (MSA) | Cerebellum, pons | Occipital cortex |
| Progressive supranuclear palsy (PSP) | Pons (adjacent to atrophic midbrain) | Occipital cortex, cerebellum |
| Corticobasal syndrome (CBS) | Sensorimotor cortex | Occipital cortex, cerebellum |
| Dementia with Lewy bodies, posterior cortical atrophy | Occipital cortex | Cerebellum, sensorimotor cortex |

These are general rules of thumb—always check the literature for your
specific disease and clinical stage.

## What HN cannot detect

HN, iPS, and PS all assume that **most** of the gray matter is normal. A
disease that reduces uptake diffusely across nearly the entire cortex will
be missed by them. In such cases, a white-matter reference or a
[reference cluster](../tutorial/reference-cluster) provides a useful
sensitivity check.

## Recommended study workflow

For studies comparing patients against healthy controls (designed with
atypical parkinsonism in mind):

1. **Preprocess patients and controls identically:** use the same scanner and
   reconstruction where possible, run the same spatial normalisation, and
   visually inspect every warped image.
2. **Build one reference from all controls**, including age and sex, with
   *Occipital cortex* and *White matter* ticked
   ([Step 2](../tutorial/build-reference)).
3. **Run the primary group comparison with HN**
   ([Step 6](../tutorial/compare)).
4. **Generate W-score maps for individual patients**
   ([Step 4](../tutorial/zmap)), and classification reports where diagnosis is
   uncertain ([Step 5](../tutorial/classify)).
5. **Repeat steps 3 and 4 with occipital cortex and white matter** as a
   sensitivity analysis.

**Common pitfalls to avoid:**

* choosing a reference region affected by the disease (cerebellum and pons in
  MSA, pons in PSP, sensorimotor cortex in CBS);
* normalising patients and controls with different methods;
* using plain Z-scores when patients and controls differ in age;
* deriving a reference cluster from the same patients you subsequently test
  with it.

## Implementation notes

* **HN** locates the peak of the ratio histogram after smoothing it with a
  Gaussian kernel (kernel density estimate), because the highest bin of a raw
  histogram depends on bin width.
* **iHN ≈ HN.** Hypometabolic voxels sit in the tail of the ratio histogram,
  far from its peak, so excluding them barely shifts the mode. The two
  factors typically agree within a few tenths of a percent—this robustness is
  why HN works so well.
* **Consistent scale.** PS, iPS, HN, and iHN are all scaled so that healthy
  gray matter averages around 1.
* **W-scores** are computed via voxel-wise ordinary least-squares regression of
  the normalised controls on the covariates; residuals are divided by the
  residual standard deviation of the controls.

See the
[README](https://github.com/lijiaxiang63/sugar-pet#how-it-works-and-where-it-differs-from-the-paper)
for full technical notes and references.
