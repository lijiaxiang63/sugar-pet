"""The folders of a study, laid out as the user guide recommends."""

from __future__ import annotations

import os
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


def _present(target: Path, is_folder: bool) -> bool:
    """Whether an entry of a study is there already.

    One of the wrong kind is an error, and so is a link that leads nowhere:
    writing through it would create a file outside the study.
    """
    if not os.path.lexists(target):
        return False
    if not target.exists():
        raise ValueError(f"'{target}' is a link that leads nowhere")
    # A file is a regular file: a pipe or a device cannot be read as a table.
    if not (target.is_dir() if is_folder else target.is_file()):
        kind = "a folder" if is_folder else "a file"
        raise ValueError(f"'{target}' exists and is not {kind}")
    return True


def _new_table(target: Path) -> None:
    """Create the table with its header.  One that is there is never written over."""
    try:
        with open(target, "x") as table:
            table.write(PARTICIPANTS_HEADER)
    except FileExistsError:  # it is there, or appeared just now: only opening it raises this
        raise
    except OSError:
        # No table is better than one without its header, which would be kept from then on.
        target.unlink(missing_ok=True)
        raise


def init_study(path: str | Path) -> list[tuple[StudyEntry, bool]]:
    """Create the folders of a study and an empty ``participants.tsv`` in ``path``.

    Returns each entry of :data:`STUDY_LAYOUT` with whether it was created
    (``False``: it was there already).  Nothing that exists is changed.
    """
    root = Path(path)
    if root.exists() and not root.is_dir():
        raise ValueError(f"'{root}' is a file, not a folder")
    # Everything is looked at before anything is made.
    present = [_present(root / entry.name, entry.is_folder) for entry in STUDY_LAYOUT]
    root.mkdir(parents=True, exist_ok=True)
    result = []
    for entry, there in zip(STUDY_LAYOUT, present):
        target = root / entry.name
        created = not there
        if created:
            try:
                if entry.is_folder:
                    target.mkdir()
                else:
                    _new_table(target)
            except FileExistsError:
                # It appeared meanwhile: it is kept, if it is what it should be.
                if not _present(target, entry.is_folder):
                    raise ValueError(f"'{target}' changed while the folders were created: run it again") from None
                created = False
        result.append((entry, created))
    return result
