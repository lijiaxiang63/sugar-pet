# 输出文件一览

这里按功能页面汇总了软件生成的所有输出文件。下文中的 `sub-01` 代表输入文件名去掉 `_pet.nii.gz` 后的前缀，`<方法>` 代表去掉连字符的方法简称（例如 `hn`、`cerebellum`、`whitematter`）。

桌面应用和命令行工具生成的文件结构与命名完全一致，两者生成的文件夹可以直接互换使用。

## 空间标准化

| 文件 | 说明 |
| --- | --- |
| `sub-01_space-MNI_pet.nii.gz` | 配准到 MNI 空间的 PET 图像，体素大小 2 mm |

## 强度归一化

第 1 步先写出对照统计，再写出归一化图像。

### 对照统计 {#control-statistics}

项目的 `hc_reference/` 文件夹保存对照的汇总结果，之后的每个页面都会读取它：

| 文件 | 说明 |
| --- | --- |
| `reference.json` | 记录对照例数、平滑核、掩膜、协变量及每例对照的归一化因子 |
| `template.nii.gz` | 对照经比例缩放后的均值图 |
| `ps_std.nii.gz` | 对照经 PS 归一化后的标准差图 |
| `hn_mean.nii.gz`、`hn_std.nii.gz` | 对照经 HN 归一化后的均值与标准差图 |
| `mask.nii.gz` | 估计因子所用的体素 |
| `stats-<方法>_mean.nii.gz`、`…_std.nii.gz` | 仅限*其他方法的对照统计*中勾选的方法 |
| `model-<方法>_beta.nii.gz`、`…_sd.nii.gz` | 仅当有协变量表时生成：摄取对协变量的回归参数 |

### 归一化图像

在 `normalised/patients/` 和 `normalised/controls/` 中：

| 文件 | 说明 |
| --- | --- |
| `sub-01_desc-<方法>_pet.nii.gz` | 归一化后的图像（每幅输入、每种方法对应一个文件） |
| `sub-01_desc-<方法>_pet_mask.nii.gz` | 开启*保存掩膜*后生成，记录估计因子所用的体素范围 |
| `factors.tsv` | 因子汇总表，每幅图像、每种方法占一行 |

`factors.tsv` 包含以下列：

| 列名 | 说明 |
| --- | --- |
| `image` | 输入图像路径 |
| `method` | 方法简称 |
| `factor` | 计算得到的归一化因子 |
| `n_voxels` | 参与估计因子的体素数 |
| `excluded_fraction` | 仅 iPS / iHN：被剔除的异常体素比例 |
| `mask` | 所使用的掩膜名称或路径 |

## Z 值图

| 文件 | 说明 |
| --- | --- |
| `sub-01_desc-<方法>_zmap.nii.gz` | 未提供协变量表时输出的 Z 值图 |
| `sub-01_desc-<方法>_wmap.nii.gz` | 提供协变量表时输出的 W 值图（已校正年龄、性别） |

## 帕金森综合征分类

| 文件 | 说明 |
| --- | --- |
| `sub-01_report.html` | 独立网页报告，可在浏览器中直接打印为 PDF |
| `sub-01_classification.json` | 报告中所有脑区得分、模式匹配度与结论的结构化数据 |
| `classification.tsv` | 汇总表，每位患者一行，包含结论与各模式得分 |

软件读取“Z 值图”页面生成的图，不在这里另存一份；命令行的 `classify` 会同时写出它计算的图 `sub-01_desc-hn_wmap.nii.gz`。

## 快速分类

每例分类的图像在 `quick/<图像名>/` 中：

| 文件 | 说明 |
| --- | --- |
| `sub-new01_space-MNI_pet.nii.gz` | 配准到 MNI 空间的图像（仅原始空间图像） |
| `sub-new01_space-MNI_desc-hn_pet.nii.gz` | 用主方法归一化的图像 |
| `sub-new01_space-MNI_desc-hn_wmap.nii.gz` | W 值图（无协变量时为 `_zmap`） |
| `sub-new01_report.html` | 独立网页报告 |
| `sub-new01_classification.json` | 报告背后的结构化数值 |

## 组间比较

`<名称>` 对应界面上填写的*结果文件名称*，默认为 `group1_vs_group2`。

| 文件 | 说明 |
| --- | --- |
| `<名称>_T.nii.gz` | 未设阈值的原始 T 值图（第 1 组减第 2 组） |
| `ClusterThresholded_<名称>_T.nii.gz` | 仅保留通过多重比较校正的显著团块的 T 值图 |
| `ClusterReport_<名称>_T.csv` | 团块统计表：体素数、峰值、AAL 和 Harvard-Oxford 脑区标注 |
| `<名称>_factors.tsv` | 参与比较的所有图像的归一化因子 |
| `<名称>_mask.nii.gz` | 实际纳入检验的体素掩膜 |
| `<名称>_beta.nii.gz`、`<名称>_cohen_f2.nii.gz` | 线性模型回归系数与效应量 |

## 参考簇

| 文件 | 说明 |
| --- | --- |
| 自定义文件名（如 `reference_cluster.nii.gz`） | 提取出的参考区二值掩膜 |

## 掩膜与图谱

点击**全部导出为 NIfTI…** 后，会在指定文件夹内为每个内置掩膜导出一个以对应简称命名的 `.nii.gz` 文件。
