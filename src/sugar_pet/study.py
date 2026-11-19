"""The folders of a study, laid out as the user guide recommends."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

PARTICIPANTS_HEADER = "participant_id\tage\tsex\n"


@dataclass(frozen=True)
class StudyEntry:
    """A folder of a study, or its table of ages and sexes."""

    name: str
    is_folder: bool
    yours: bool  # you fill it; the others are written by the analyses
    text: str


STUDY_LAYOUT = (
    StudyEntry(
        "raw",
        True,
        True,
        "Images still in native scanner space. Only needed if yours are not in MNI space yet: "
        "'spatial-norm' warps them into patients/ and controls/.",
    ),
    StudyEntry(
        "patients", True, True, "Patient images in MNI space, one scan per subject, e.g. sub-pt01_space-MNI_pet.nii.gz."
    ),
    StudyEntry(
        "controls",
        True,
        True,
        "Healthy-control images in MNI space, from the same scanner and preprocessing as the patients.",
    ),
    StudyEntry(
        "participants.tsv",
        False,
        True,
        "Age and sex of every subject, patients and controls, one row each. Used to adjust for them.",
    ),
    StudyEntry(
        "hc_reference",
        True,
        False,
        "The reference that 'build-reference' makes from controls/. Most other steps read it.",
    ),
    StudyEntry("normalised", True, False, "Intensity-normalised images and factors.tsv, from 'normalize'."),
    StudyEntry("zmaps", True, False, "Z-score or W-score maps of the patients, from 'zmap'."),
    StudyEntry("reports", True, False, "Classification reports of the patients, from 'classify'."),
    StudyEntry("compare", True, False, "T-maps and cluster tables of patients against controls, from 'compare'."),
)


def init_study(path: str | Path) -> list[tuple[StudyEntry, bool]]:
    """Create the folders of a study and an empty ``participants.tsv`` in ``path``.

    Returns each entry of :data:`STUDY_LAYOUT` with whether it was created
    (``False``: it was there already).  Nothing that exists is changed.
    """
    root = Path(path)
    if root.exists() and not root.is_dir():
        raise ValueError(f"'{root}' is a file, not a folder")
    for entry in STUDY_LAYOUT:
        target = root / entry.name
        if target.exists() and target.is_dir() != entry.is_folder:
            kind = "a folder" if entry.is_folder else "a file"
            raise ValueError(f"'{target}' exists and is not {kind}")
    root.mkdir(parents=True, exist_ok=True)
    result = []
    for entry in STUDY_LAYOUT:
        target = root / entry.name
        created = not target.exists()
        if created and entry.is_folder:
            target.mkdir()
        elif created:
            target.write_text(PARTICIPANTS_HEADER)
        result.append((entry, created))
    return result
