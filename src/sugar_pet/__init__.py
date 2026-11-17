"""sugar-pet: Standardized Uptake Global And Regional-normalization for brain FDG-PET."""

from importlib.metadata import PackageNotFoundError, version

try:
    __version__ = version("sugar-pet")
except PackageNotFoundError:  # running from a source tree without installing
    __version__ = "0.0.0"

from .cluster import reference_cluster
from .masks import MASKS, atlas_labels, builtin_mask, labels_mask
from .normalize import METHODS, NormResult, normalize
from .reference import Reference
from .zscore import zmap

__all__ = [
    "MASKS",
    "METHODS",
    "NormResult",
    "Reference",
    "atlas_labels",
    "builtin_mask",
    "labels_mask",
    "normalize",
    "reference_cluster",
    "zmap",
]
