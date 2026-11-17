# Install the app

SUGAR-PET is a standalone ~12 MB executable. It requires no Python
environment and no MATLAB installation (unless you use the optional
[Spatial Normalisation](../tutorial/spatial) page to drive SPM).

## 1. Download

Go to the [latest release](https://github.com/lijiaxiang63/sugar-pet/releases/latest)
and download the package for your operating system (`<version>` denotes the
release number, e.g. `0.5.0`):

| System | File |
| --- | --- |
| macOS (Apple silicon & Intel) | `sugar-pet-<version>.dmg` |
| Windows (installer) | `sugar-pet-setup-<version>-amd64.exe` (or `arm64`) |
| Windows (portable) | `sugar-pet-<version>-windows-amd64-portable.exe` (or `arm64`) |
| Debian / Ubuntu | `sugar-pet_<version>_amd64.deb` (or `arm64`) |
| Fedora / RHEL | `sugar-pet-<version>-1.x86_64.rpm` (or `aarch64`) |
| Other Linux | `sugar-pet-<version>-linux-amd64.tar.gz` (or `arm64`) and `install.sh` |

`SHA256SUMS.txt` on the same page provides checksums for verification.

## 2. Install

::: code-group

```text [macOS]
1. Open the .dmg and drag SUGAR-PET into Applications.
2. On first launch, right-click the app and choose Open.
```

```text [Windows]
Installer:  run sugar-pet-setup-<version>-amd64.exe and follow the prompts.
Portable:   place the .exe anywhere and double-click to launch.
```

```bash [Debian / Ubuntu]
sudo apt install ./sugar-pet_<version>_amd64.deb
```

```bash [Fedora / RHEL]
sudo dnf install ./sugar-pet-<version>-1.x86_64.rpm
```

```bash [Other Linux]
# Keep the .tar.gz and install.sh in the same directory
sh install.sh
```

:::

::: warning macOS: "cannot be opened"
The macOS build is ad-hoc signed rather than notarised by Apple, so
double-clicking it the first time will be blocked. Right-click the app and
choose **Open**. If macOS still blocks it, go to **System Settings › Privacy &
Security**, scroll down to the message about SUGAR-PET, and click **Open
Anyway**. You only need to do this once.
:::

::: tip Windows: "unknown publisher"
If SmartScreen shows a blue warning when launching the installer, click
**More info** and then **Run anyway**.
:::

## 3. Launch

Open SUGAR-PET. It opens on the **Intensity Normalisation** page by default.

<Shot name="normalize" alt="SUGAR-PET after it starts (here already filled with images)" />

Use the **English** / **中文** buttons in the bottom-left corner to switch
languages; the app remembers your preference.

## Build from source

Requires Go 1.27 or newer:

```bash
git clone https://github.com/lijiaxiang63/sugar-pet
cd sugar-pet/app
go tool mygo build     # current platform: build/<os>-<arch>/
go tool mygo dev       # run directly during development
```

## Next

Take [a tour of the window](./interface).
