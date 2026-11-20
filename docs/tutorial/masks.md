# Masks and atlas

**Page: Masks & Atlas** · a resource, not a step

This page lists the brain regions built into SUGAR-PET. Use it to see what a
reference region covers, to export the masks and look at them in a viewer,
or to find the label numbers of a custom region.

## Built-in masks

<Shot name="masks" alt="The built-in masks" :marks="[[1, 178]]" />

1. **The masks:** each row gives the name, what it covers, the short name
   (used by the command line and in file names) and the volume.

| Mask | Short name | Used by |
| --- | --- | --- |
| Gray matter | `gray-matter` | PS, HN, iPS, iHN; the default voxels of a group comparison |
| Brain parenchyma | `parenchyma` | Whole brain |
| Cerebellum | `cerebellum` | Reference region (RRC in the paper) |
| Cerebellar gray matter | `cerebellar-gm` | Reference region |
| Pons | `pons` | Reference region |
| White matter | `white-matter` | Reference region |
| Occipital cortex | `occipital` | Reference region |
| Sensorimotor cortex | `sensorimotor` | Reference region |
| Brainstem | `brainstem` | Custom region (RRBS in the paper) |
| Left / right temporal cortex | `temporal-left`, `temporal-right` | Custom region (RRTL: take the side *opposite* the lesion) |

All masks are on a 2 mm MNI grid (ICBM 2009c) and are resampled to each image
by themselves. The pons mask is a pragmatic central-pons region, not an
anatomical gold standard. With another MNI template or atlas, give a mask
file of your own.

## Atlas labels

Scroll down.

<Shot name="masks-atlas" alt="The labels of the CerebrA atlas" :marks="[[2, 351, 387], [3, 503], [4, 544]]" />

2. **Export All as NIfTI…** writes every built-in mask as a NIfTI file into a
   folder you choose, to look at over an MNI template.
3. **Search** for a region by name.
4. **The labels:** each region has a number for the right side and one for
   the left. Tick the rows of the regions you want: a bar under the table
   lists their numbers, and **Use as Custom Region** opens Intensity
   Normalisation with them as its custom region.

To type the numbers yourself, tick **Custom region** in
[Intensity Normalisation](./normalize#a-region-of-your-own), choose **Atlas
labels**, and give them separated by commas: `34,85` for the lateral
occipital cortex of both sides, `40,91` for both thalami.

::: details The same from the command line
```bash
sugar-pet masks -o masks/     # export the built-in masks
sugar-pet labels              # list the atlas labels
```
:::
