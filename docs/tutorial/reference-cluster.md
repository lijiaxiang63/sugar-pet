# Step 7: Reference cluster

**Page: Reference Cluster** · advanced · needs the reference

Instead of picking a predefined region from an atlas, you can let a patient
group's own data reveal which voxels are spared by the disease
([Yakushev et al., 2009](https://doi.org/10.1016/j.neuroimage.2008.07.015)).

The intuition is straightforward: after proportional scaling to the global
mean, spared regions appear *hyper*metabolic relative to controls because
hypometabolic lesions pull the global mean down. Those voxels then serve as
the reference region.

::: warning Avoid circular analysis
The resulting cluster is derived from the patient scans you pass in.
Comparing those same patients against controls using that cluster is
circular: derive the cluster from an independent patient sample, or use
cross-validation.
:::

## Fill in the page

<Shot name="cluster" alt="The Reference Cluster page, filled in" :marks="[[1, 177], [2, 548], [3, 604], [4, 664], [5, 'run']]" />

1. **Add the patient images** (at least 2 scans, all on the same grid).
2. **Reference:** the healthy controls to compare against; pre-filled after
   [Step 2](./build-reference).
3. **Save mask as:** output path for the NIfTI mask, e.g.
   `study/reference_cluster.nii.gz`.
4. **Options in effect:** the threshold of the test. **Options…** opens
   **Settings › Reference Cluster**, with two options:
   * *p-value*: one-sided uncorrected threshold (default `0.001`);
   * *Smallest cluster (mL)*: connected clusters smaller than this volume
     are discarded (default `1` mL).
5. Click **Find Cluster**.

## Check the result

<Shot name="cluster-results" alt="The reference cluster that was found" :marks="[[6, 347], [7, 560], [8, 784]]" />

6. **Result:** the volume of the cluster and where the mask was saved.
7. **The picture** draws the cluster in orange on axial slices of the
   template. Check where it lies—a tiny cluster of a few mL in an unexpected
   spot is not a reliable reference.
8. **Normalise Intensity with This Region** opens the
   [Intensity Normalisation](./normalize) page with *Custom region* enabled
   and this mask pre-selected. Add the scans you want to normalise and run.

::: details The same from the command line
```bash
sugar-pet reference-cluster patients/*.nii.gz -r hc_reference/ -o reference_cluster.nii.gz
sugar-pet normalize others/*.nii.gz -m region --mask reference_cluster.nii.gz -o normalised/
```
:::

## Next

Browse the built-in [masks and atlas labels](./masks).
