# Reference cluster

**Page: Reference Cluster** · advanced · needs step 1

Instead of a predefined region from an atlas, the data of a patient group can
show which voxels the disease spares
([Yakushev et al., 2009](https://doi.org/10.1016/j.neuroimage.2008.07.015)).

After proportional scaling to the global mean, spared regions look
*hyper*metabolic against the controls, because the hypometabolic ones pull
the global mean down. A t-test of the patients against the controls picks
those voxels out, and they serve as the reference region.

::: warning Avoid circular analysis
The cluster comes from the patients you give. Comparing those same patients
with the controls by that cluster is circular: derive it from another sample
of patients, or cross-validate.
:::

## Run it

<Shot name="cluster" alt="The Reference Cluster page" :marks="[[1, 180], [2, 316], [3, 357], [4, 450], [5, 'run']]" />

1. **Patients:** the patients of the project, at least 2, on the same grid.
   To change them, **Change in Data**.
2. **Healthy controls:** the control statistics step 1 summarised, which the
   patients are tested against.
3. **Mask saved as:** `reference_cluster.nii.gz` in the project folder.
4. **Options in effect:** the threshold of the test. **Options…** opens
   **Settings › Reference Cluster**, with two options:
   * *p-value*: one-sided, uncorrected (`0.001` by default);
   * *Smallest cluster (mL)*: smaller connected clusters are dropped (`1` mL
     by default).
5. Click **Find Cluster**.

## Check the result

<Shot name="cluster-results" alt="The reference cluster found" :marks="[[6, 332], [7, 742], [8, 403, 782]]" />

6. **Result:** the volume of the cluster and where the mask was saved.
7. **The picture** draws the cluster in orange on axial slices of the
   template. Check where it lies: a cluster of a few mL in an unexpected place
   is no reliable reference.
8. **Normalise Intensity with This Region** opens
   [Intensity Normalisation](./normalize) with *Custom region* ticked and this
   mask chosen.

::: details The same from the command line
```bash
sugar-pet reference-cluster patients/*.nii.gz -r hc_reference/ -o reference_cluster.nii.gz
sugar-pet normalize others/*.nii.gz -m region --mask reference_cluster.nii.gz -o normalised/
```
:::

## Next

The built-in [masks and atlas labels](./masks).
