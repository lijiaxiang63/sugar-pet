# Command line

The desktop app is a graphical front end for **sugar-pet**, a Python package
with a command-line interface. Both implement the same methods and read and
write identical files, so you can mix them freely—build a reference in the
app and pass it to a script, or vice versa.

Use the CLI for batch processing across studies, running on a remote server,
or calling methods directly from Python.

## Install

```bash
pip install git+https://github.com/lijiaxiang63/sugar-pet
```

Requires Python 3.10 or newer. Group comparisons also require
[GrouVox](https://github.com/lijiaxiang63/GrouVox):

```bash
pip install "sugar-pet[stats] @ git+https://github.com/lijiaxiang63/sugar-pet"
```

## App pages and CLI commands

| App page | Command |
| --- | --- |
| Study Folder | `sugar-pet init` |
| Spatial Normalisation | `sugar-pet spatial-norm` |
| Intensity Normalisation | `sugar-pet normalize` |
| Reference | `sugar-pet build-reference` |
| Z-score Maps | `sugar-pet zmap` |
| Classify Parkinsonism | `sugar-pet classify` |
| Compare Groups | `sugar-pet compare` |
| Reference Cluster | `sugar-pet reference-cluster` |
| Masks & Atlas | `sugar-pet masks`, `sugar-pet labels`, `sugar-pet methods` |

Run `sugar-pet <command> --help` to see all options for a subcommand.

## Running the tutorial as a script

The full [tutorial](../tutorial/) translates to:

```bash
# Before you start: create the study folder, then put your data in it
sugar-pet init study/
cd study/

# Step 1 (optional): warp to MNI space
sugar-pet spatial-norm raw/*_pet.nii.gz -o patients/ --spm-path /path/to/spm12 --jobs 4

# Step 2: build the reference
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ \
    --covariates participants.tsv --stats-for occipital,white-matter

# Step 3: normalise intensity
sugar-pet normalize patients/*.nii.gz -m hn,cerebellum -r hc_reference/ -o normalised/

# Step 4: W-score maps
sugar-pet zmap patients/*.nii.gz -m hn -r hc_reference/ -o zmaps/ --covariates participants.tsv

# Step 5: classify parkinsonism
sugar-pet classify patients/*.nii.gz -r hc_reference/ -o reports/ --covariates participants.tsv

# Step 6: compare groups
sugar-pet compare --group1 patients/*.nii.gz --group2 controls/*.nii.gz \
    -m hn -r hc_reference/ -o compare/ --covariates participants.tsv --name patients_vs_controls

# Step 7: reference cluster
sugar-pet reference-cluster patients/*.nii.gz -r hc_reference/ -o reference_cluster.nii.gz
```

## App settings and CLI flags

| App setting | CLI flag |
| --- | --- |
| Ticked methods | `-m hn,cerebellum` (or `-m all`) |
| Reference | `-r hc_reference/` |
| Output folder | `-o out/` |
| Covariate table, *Adjust for* | `--covariates participants.tsv`, `--adjust-for age,sex` |
| Smoothing | `--fwhm 8` |
| Custom region: built-in mask or file | `-m region --mask brainstem`, `--mask my_mask.nii.gz` |
| Custom region: atlas labels | `-m region --labels 34,85` |
| Region statistic | `--statistic median` |
| Target value | `--target 50` |
| Write normalised images (off) | `--factors-only` |
| Save masks | `--save-masks` |
| Exclusion p-value, Exclude, Re-estimations | `--p-threshold 0.01`, `--exclude hypo`, `--max-iter 1` |
| Plain mean for iPS | `--literal-ips` |
| Z-score maps with other methods | `--stats-for occipital,white-matter` |
| Multiple-comparison correction | `--correction grf` (`--voxel-p`, `--cluster-p`), `fdr` (`--q`), `none` |
| Voxels to test | `--analysis-mask gray-matter` |
| Keep the images that were tested | `--keep-images` |
| Language of the report | `--lang en` |

## Python API

```python
from sugar_pet import Reference, normalize, zmap, classify

ref = Reference.load("hc_reference")

res = normalize("sub-pt01_space-MNI_pet.nii.gz", method="hn", reference=ref)
print(res.factor)
res.save("sub-pt01_space-MNI_desc-hn_pet.nii.gz")

w = zmap("sub-pt01_space-MNI_pet.nii.gz", method="hn", reference=ref,
         covariates={"age": 63, "sex": "F"})

report = classify("sub-pt01_space-MNI_pet.nii.gz", reference=ref,
                  covariates={"age": 63, "sex": "F"})
print(report.conclusion)
```

See the [README](https://github.com/lijiaxiang63/sugar-pet#readme) for full
documentation of every command, option, and function.
