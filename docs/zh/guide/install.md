# 安装应用

SUGAR-PET 安装包只有约 12 MB，开箱即用，不需要配置 Python 环境，也不需要 MATLAB（除非你要使用可选的[空间标准化](../tutorial/spatial)页面来调用 SPM）。

## 1. 下载安装包

前往[最新发布页](https://github.com/lijiaxiang63/sugar-pet/releases/latest)，根据你的操作系统选择对应的文件（下文中的 `<版本号>` 如 `0.8.0`）：

| 系统平台 | 下载文件 |
| --- | --- |
| macOS（Apple 芯片 / Intel） | `sugar-pet-<版本号>.dmg` |
| Windows（安装版） | `sugar-pet-setup-<版本号>-amd64.exe`（或 `arm64`） |
| Windows（绿色免安装版） | `sugar-pet-<版本号>-windows-amd64-portable.exe`（或 `arm64`） |
| Debian / Ubuntu | `sugar-pet_<版本号>_amd64.deb`（或 `arm64`） |
| Fedora / RHEL | `sugar-pet-<版本号>-1.x86_64.rpm`（或 `aarch64`） |
| 其他 Linux 发行版 | `sugar-pet-<版本号>-linux-amd64.tar.gz`（或 `arm64`）和 `install.sh` |

发布页附带的 `SHA256SUMS.txt` 可用于校验文件完整性。

## 2. 安装步骤

::: code-group

```text [macOS]
1. 双击打开 .dmg 文件，把 SUGAR-PET 拖入“应用程序”文件夹。
2. 首次启动时，请右键点击应用图标，选择“打开”。
```

```text [Windows]
安装版：运行 sugar-pet-setup-<版本号>-amd64.exe，按向导提示安装即可。
免安装版：将 .exe 文件放到任意目录，双击直接运行。
```

```bash [Debian / Ubuntu]
sudo apt install ./sugar-pet_<版本号>_amd64.deb
```

```bash [Fedora / RHEL]
sudo dnf install ./sugar-pet-<版本号>-1.x86_64.rpm
```

```bash [其他 Linux]
# 确保 .tar.gz 和 install.sh 在同一目录下
sh install.sh
```

:::

::: warning macOS 提示“无法打开”怎么办？
由于 macOS 版目前只有临时签名、未经过 Apple 公证，首次直接双击会被系统拦截。右键点击应用图标选择**打开**即可。如果依然提示无法打开，可以前往**系统设置 › 隐私与安全性**，在底部找到 SUGAR-PET 的拦截提示并点击**仍要打开**。只需操作这一次。
:::

::: tip Windows 提示“未知发布者”怎么办？
如果出现 SmartScreen 蓝色警告窗口，点击**更多信息**，再点击**仍要运行**即可。
:::

## 3. 启动应用

打开 SUGAR-PET 后，默认会进入**强度归一化**页面。

<Shot name="normalize" alt="SUGAR-PET 启动后的窗口（图中已添加了图像）" />

应用启动时默认使用系统语言。要在中文和英文之间切换，点击侧边栏底部的**设置**，在**通用 › 界面语言**中选择即可，软件会自动记住你的偏好。

## 4. 更新

从 0.7.0 版开始，应用会自行更新：启动后不久以及此后每天各检查一次 GitHub 上的新版本，告诉你更新了什么，并在你同意后安装。在**设置 › 通用**中点击**检查更新…**可以立即检查，下方的两个开关分别控制是否自动检查、是否自动下载并安装（详见[软件更新](./interface#updates)）。

::: info 两种例外情况
* 通过 `.deb` 或 `.rpm` 软件包安装的应用不会自行替换：请从发布页下载新的软件包，用包管理器安装。
* 0.6.0 及更早的版本没有更新功能，需要从发布页下载新版本，手动覆盖安装一次。
:::

## 从源码构建（可选）

如果你需要自己编译，需安装 Go 1.27 或更高版本：

```bash
git clone https://github.com/lijiaxiang63/sugar-pet
cd sugar-pet/app
go tool mygo build     # 为当前系统构建，产物在 build/<系统>-<架构>/
go tool mygo dev       # 开发模式下直接运行
```

## 下一步

继续阅读[认识界面](./interface)。
