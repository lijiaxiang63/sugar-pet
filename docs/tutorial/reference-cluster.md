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

<Shot name="cluster" alt="The Reference Cluster page, filled in" :marks="[[1, 181], [2, 514], [3, 571], [4, 627], [5, 683]]" />

1. **Add the patient images** (at least 2 scans, all on the same grid).
2. **Reference:** the healthy controls to compare against; pre-filled after
   [Step 2](./build-reference).
3. **p-value:** one-sided uncorrected threshold for the test (default `0.001`).
4. **Smallest cluster:** connected clusters smaller than this volume (in mL)
   are discarded (default `1` mL).
5. **Save mask as:** output path for the NIfTI mask, e.g.
   `study/reference_cluster.nii.gz`.

Scroll down, then click **Find Cluster**.

<Shot name="cluster-results" alt="The reference cluster that was found" :marks="[[6, 'run'], [7, 732], [8, 784]]" />

6. Click **Find Cluster**.
7. **Result:** reports the cluster volume and output path. Open the mask
   over an MNI template to verify its location—a tiny cluster of a few mL in
   an unexpected spot is not a reliable reference.
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
