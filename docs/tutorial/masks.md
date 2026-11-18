# Masks and atlas

**Page: Masks & Atlas** · reference resource, not a workflow step

This page lists the brain regions built into SUGAR-PET. Use it to check what
each reference region covers, export the masks to inspect in a viewer, or
look up label IDs for a custom region.

## Built-in masks

<Shot name="masks" alt="The built-in masks, with their short names and volumes" :marks="[[1, 192], [2, 805]]" />

1. **Mask list:** each row shows the mask name, anatomical coverage, short
   name (used by the CLI and output filenames), and volume.
2. **Export All as NIfTI…** writes every built-in mask as a separate NIfTI
   file into a chosen folder so you can inspect them over an MNI template.

| Mask | Short name | Used by |
| --- | --- | --- |
| Gray matter | `gray-matter` | PS, HN, iPS, iHN; default analysis mask for group comparisons |
| Brain parenchyma | `parenchyma` | Whole brain |
| Cerebellum | `cerebellum` | Reference region (RRC in the paper) |
| Cerebellar gray matter | `cerebellar-gm` | Reference region |
| Pons | `pons` | Reference region |
| White matter | `white-matter` | Reference region |
| Occipital cortex | `occipital` | Reference region |
| Sensorimotor cortex | `sensorimotor` | Reference region |
| Brainstem | `brainstem` | Custom region (RRBS in the paper) |
| Left / right temporal cortex | `temporal-left`, `temporal-right` | Custom region (RRTL: pick the side *opposite* the lesion) |

All masks are defined on a 2 mm MNI grid (ICBM 2009c) and resampled
automatically to match each input image. The pons mask is a pragmatic
central-pons region rather than an anatomical gold standard. If you work
with a different MNI template or atlas, supply your own mask file.

## Atlas labels

Scroll down to browse the CerebrA atlas labels.

<Shot name="masks-atlas" alt="The labels of the CerebrA atlas" :marks="[[3, 503], [4, 577]]" />

3. **Search** for a region by name.
4. **Note the label numbers.** Each region has separate IDs for the right and
   left hemispheres.

To use them, go to [Intensity Normalisation](./normalize#a-region-of-your-own),
tick **Custom region**, select **Atlas labels**, and enter the comma-separated
IDs—for example `34,85` for bilateral lateral occipital cortex, or `40,91`
for bilateral thalamus.

::: details The same from the command line
```bash
sugar-pet masks -o masks/     # export the built-in masks
sugar-pet labels              # list the atlas labels
```
:::
