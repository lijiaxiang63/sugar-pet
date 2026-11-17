# sugar-pet

**S**tandardized **U**ptake **G**lobal **A**nd **R**egional-normalization：脑 FDG-**PET** 强度归一化工具。

[English](README.md)

FDG-PET 的体素值受注射剂量、血糖、体重等因素影响，不同受试者的图像不能直接比较。
**强度归一化**的做法是把整幅图像除以一个数，即**归一化因子**：

```
归一化后的图像 = 原图像 / 因子
```

各种方法的区别只在于这个因子怎么估计。sugar-pet 提供常用的几种方法，并实现了
[López-González 等, NeuroImage 2020](https://doi.org/10.1016/j.neuroimage.2020.117229)
中基于直方图的方法（HN、iPS、iHN）。

## 安装

```bash
pip install git+https://github.com/lijiaxiang63/sugar-pet
```

开发用：

```bash
git clone https://github.com/lijiaxiang63/sugar-pet && cd sugar-pet && uv sync
```

需要 Python 3.10+，只依赖 numpy、scipy、nibabel。

## 快速上手

输入图像必须已经**空间标准化到 MNI 空间**（体素大小和包围盒不限）。
还没有标准化的见[图像还不在 MNI 空间？](#图像还不在-mni-空间)。

```bash
# 1. 参考区法：不需要其他数据
sugar-pet normalize sub-01_pet.nii.gz -m cerebellum -o out/

# 2. 直方图法（推荐）：先用健康对照建一次参考库 ...
sugar-pet build-reference controls/*.nii.gz -o hc_reference/

# ... 之后每个患者都用它
sugar-pet normalize patients/*.nii.gz -m hn -r hc_reference/ -o out/
```

`out/` 中每个输入对应一幅归一化图像（`sub-01_desc-hn_pet.nii.gz`），
另有 `factors.tsv` 记录每幅图像的因子。

## 方法

终端里运行 `sugar-pet methods` 可查看同样的列表。

| `-m`             | 因子的估计方式                                             | 需要对照 |
| ---------------- | ---------------------------------------------------------- | :------: |
| `cerebellum`     | 全小脑平均摄取（文中的 RRC）                               |    否    |
| `cerebellar-gm`  | 小脑灰质平均摄取                                           |    否    |
| `pons`           | 脑桥中央平均摄取                                           |    否    |
| `white-matter`   | 大脑深部白质平均摄取                                       |    否    |
| `occipital`      | 枕叶皮层平均摄取                                           |    否    |
| `sensorimotor`   | 初级感觉运动皮层平均摄取                                   |    否    |
| `region`         | 任意自选区域的平均摄取（`--mask` 或 `--labels`，见下）     |    否    |
| `ps`             | **比例缩放**：灰质掩膜内的平均摄取                         |    否    |
| `spm-global`     | SPM 的全局均值（高于全图均值 1/8 的所有体素）              |    否    |
| `hn`             | **直方图归一化**：图像除以对照均值图，取比值中**出现最多**的值 | 是   |
| `ips`            | **迭代 PS**：先做 PS，找出低代谢体素，去掉后重做 PS        |    是    |
| `ihn`            | **迭代 HN**：先做 HN，找出低代谢体素，去掉后重做 HN        |    是    |

一次用多种方法：`-m cerebellum,ps,hn` 或 `-m all`。加 `--factors-only` 只输出因子表。

参考区相关的选项：

| 选项                 | 含义                                                             |
| -------------------- | ---------------------------------------------------------------- |
| `--mask 名称或文件`  | 换用其他区域：[内置掩膜](#内置掩膜)的名称，或自己的 MNI 空间 NIfTI 文件 |
| `--labels 34,85`     | 用图谱标签组合出区域（`sugar-pet labels` 列出全部标签）          |
| `--statistic median` | 取区域的中位数而不是均值（`ps` 也适用）                          |
| `--target 50`        | 把输出缩放到“参考值 = 50”而不是 1（任何方法都可用）              |

例如，文章里另外两个参考区是内置掩膜：`-m region --mask brainstem`（RRBS）和
`-m region --mask temporal-left`（RRTL，选病灶**对侧**）。

### 该选哪种？

按照文章的结论：

* **默认推荐 `hn`。** 不需要预先知道哪些脑区受累，是表现最好的数据驱动方法。
  `ihn` 得到的因子几乎相同（原因见[实现说明](#实现说明及与原文的差异)）。
* **低代谢范围大时 `ps` 有偏。** 病灶拉低了全脑均值，因子偏小，
  正常组织反而显得**高**代谢。`ips` 能消除大部分偏差。
* **参考区法只在参考区足够大、且确实不受疾病影响时才可靠。** 脑桥这类小区域噪声大。

哪些区域不受累取决于疾病，举几个例子：

| 疾病                     | 不宜作参考区             | 通常保留               |
| ------------------------ | ------------------------ | ---------------------- |
| 阿尔茨海默病             | —                        | 小脑、脑桥、感觉运动皮层 |
| 多系统萎缩（MSA）        | 小脑、脑桥               | 枕叶                   |
| 进行性核上性麻痹（PSP）  | 脑桥（邻近中脑萎缩）     | 枕叶、小脑             |
| 皮质基底节综合征（CBS）  | 感觉运动皮层             | 枕叶、小脑             |
| 路易体痴呆、后部皮质萎缩 | 枕叶                     | 小脑、感觉运动皮层     |

以上只是经验规律，具体请结合所研究疾病及分期的文献判断。

`hn`、`ips`、`ps` 都假设**大部分**灰质是正常的。如果疾病让几乎所有区域的摄取都下降，
它们看不出来；这时用白质参考区或[参考簇](#参考簇)做敏感性分析会有帮助。

## 健康对照参考库

`hn`、`ips`、`ihn` 需要把每幅图像与一组健康对照比较。
`sugar-pet build-reference` 把对照汇总成一个小文件夹：

| 文件               | 内容                                                  |
| ------------------ | ----------------------------------------------------- |
| `template.nii.gz`  | 对照经比例缩放后的均值图（HN 的“正常模板”）           |
| `ps_std.nii.gz`    | 对照经 PS 后的标准差                                  |
| `hn_mean.nii.gz`、`hn_std.nii.gz` | 对照经 HN 后的均值与标准差             |
| `mask.nii.gz`      | 估计因子所用的体素（灰质掩膜）                        |
| `reference.json`   | 对照例数、平滑核、每个对照的因子                      |
| `stats-<方法>_mean.nii.gz`、`..._std.nii.gz` | 仅在使用 `--stats-for` 时生成：对照经其他方法归一化后的均值与标准差（用于 [Z 值图](#z-值图)） |

对照应与患者来自同一台扫描仪、同样的重建参数，并经过相同的预处理，建议不少于 20 例。

默认情况下，对照和之后的每个患者都会用 8 mm 高斯核平滑，但**只用于估计因子**；
输出图像是原图除以因子，不会被平滑。如果图像已经平滑过，两个命令都加 `--fwhm 0`。

## 迭代方法的选项

| 选项              | 默认值  | 含义                                                         |
| ----------------- | ------- | ------------------------------------------------------------ |
| `--p-threshold`   | `0.01`  | 与对照相比 p 值（未校正）小于该阈值的体素被排除              |
| `--exclude`       | `hypo`  | `hypo`：只排除低代谢体素（原文做法）；`both`：高代谢也排除   |
| `--max-iter`      | `1`     | 重新估计的次数（原文为 1）                                   |
| `--save-masks`    | 关      | 同时保存估计每个因子所用的掩膜                               |

## Z 值图

Z 值图表示归一化后的图像每个体素偏离对照多远：`z = (图像 − 对照均值) / 对照标准差`，
负值表示摄取低于对照。

```bash
sugar-pet zmap patients/*.nii.gz -m hn -r hc_reference/ -o zmaps/
```

输出 `sub-01_desc-hn_zmap.nii.gz`。比较之前，图像会按对照的方式平滑。
`ps`、`ips`、`hn`、`ihn` 用任何参考库都可以。参考区法要求对照也用同一个参考区缩放过，
所以建参考库时要指明：

```bash
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ --stats-for cerebellum,pons
sugar-pet zmap patients/*.nii.gz -m pons -r hc_reference/ -o zmaps/
```

（`--stats-for all` 保存所有参考区。）Z 值没有做年龄、性别校正，请使用与患者匹配的对照。

## 参考簇

除了从图谱里选参考区，也可以让一组患者的数据自己指出哪些体素不受疾病影响
（[Yakushev 等, 2009](https://doi.org/10.1016/j.neuroimage.2008.07.015)）。
比例缩放之后，未受累的区域与对照相比会显得**高**代谢，因为病灶拉低了全脑均值。
这些体素就作为参考区：

```bash
sugar-pet reference-cluster patients/*.nii.gz -r hc_reference/ -o cluster.nii.gz
sugar-pet normalize patients/*.nii.gz -m region --mask cluster.nii.gz -o out/
```

`--p-threshold`（默认 0.001，未校正）和 `--min-cluster`（默认 1 mL）决定保留哪些体素。

参考簇依赖于生成它的那组患者。之后再拿同一组患者与对照比较属于循环论证：
应当用另一批患者生成参考簇，或做交叉验证。

## Python 接口

```python
from sugar_pet import Reference, normalize, zmap

ref = Reference.build(control_files, fwhm=8)  # 或 Reference.load("hc_reference")
ref.save("hc_reference")

res = normalize("sub-01_pet.nii.gz", method="ihn", reference=ref)
print(res.factor)  # 归一化因子
print(res.info)  # 所用体素数、被排除的比例等
res.save("sub-01_desc-ihn_pet.nii.gz")
res.mask_image()  # 估计因子所用的体素

z = zmap("sub-01_pet.nii.gz", method="ihn", reference=ref)  # NIfTI 图像
```

参考区可以是内置掩膜、自己的文件，或内置图谱的任意标签组合：

```python
from sugar_pet import atlas_labels, labels_mask, reference_cluster

res = normalize(img, "region", mask="brainstem", statistic="median", target=100)

atlas_labels()  # {21: 'Putamen (R)', 72: 'Putamen (L)', ...}
res = normalize(img, "region", mask=labels_mask([40, 91]))  # 双侧丘脑

cluster = reference_cluster(patient_files, ref)
res = normalize(img, "region", mask=cluster)
```

## 内置掩膜

`gray-matter`、`parenchyma`、`cerebellum`、`cerebellar-gm`、`pons`、`white-matter`、
`occipital`、`sensorimotor`、`brainstem`、`temporal-left`、`temporal-right`。
`sugar-pet masks -o masks/` 可把它们导出成 NIfTI 文件以便查看，
`sugar-pet labels` 列出 `--labels` 可用的图谱标签。

掩膜来自两个可自由再分发的 MNI 资源（重采样到 2 mm）：ICBM 2009c 组织概率图和
[CerebrA 图谱](https://doi.org/10.1038/s41597-020-0557-9)，使用时会自动重采样到每幅图像的网格。

脑桥掩膜取 CerebrA 脑干在 z = −44 到 −26 mm 之间的部分并在层面内腐蚀，
是一个实用的脑桥中央区，而非解剖金标准。`occipital` 是整个枕叶皮层；
只要枕叶外侧皮层时用 `--labels 34,85`。使用其他 MNI 版本或图谱时请传入自己的掩膜。

## 图像还不在 MNI 空间？

任何空间标准化方法都可以（SPM、ANTs 等）。如果只有 PET 且装有 SPM，
可以用自带的小封装（SPM “Old Normalise” + PET 模板，需要 MATLAB）：

```bash
sugar-pet spatial-norm raw/*_pet.nii.gz -o mni/ --spm-path /path/to/spm12 --jobs 4
```

它会围绕头部裁剪、把原点移到脑中心，输出 2 mm 的 MNI 空间图像。结果请务必目视检查。

## 实现说明及与原文的差异

* **HN。** 在灰质掩膜内计算比值图（受试者 / 模板），其直方图的峰值就是因子。
  直接取直方图最大值会随分箱宽度变化，因此先用高斯核平滑直方图（核密度估计）再找峰。
* **找异常体素。** 原文用 SPM 的两样本 t 检验（单个患者对照组）。sugar-pet 直接计算等价的
  等方差 t 统计量 `t = (x − 均值) / (标准差 · √(1 + 1/n))`，自由度 n − 1，不需要 SPM。
* **iPS。** 去掉部分体素后，剩余体素在对照中的均值也不再是 1。
  因此 sugar-pet 用受试者均值除以对照模板在**同一批体素**上的均值。
  `--literal-ips` 则按原文字面做法直接取均值。
* **iHN ≈ HN。** 低代谢体素位于比值直方图的尾部，离峰很远，去掉它们几乎不移动峰的位置。
  所以 `hn` 和 `ihn` 的因子通常只差千分之几，这种稳健性正是 HN 好用的原因。
* **统一的尺度。** `ps`、`ips`、`hn`、`ihn` 都按“健康大脑灰质掩膜均值约为 1”来定标。
* **参考簇。** 两样本 t 检验（患者 > 对照，等方差）直接由患者图像和参考库里保存的
  均值、标准差算出，不需要再读对照图像。
* **有意不做的。** 把图像标准化成零均值、单位方差（深度学习前常用）既平移又缩放，
  摄取比值就丢了。SSM/PCA 协方差分析里的对数变换和双中心化属于那种分析本身，
  而且事先怎么缩放都不影响它的结果。

在模拟图像中（大范围皮层病灶、代谢下降 40%，见 `tests/`），因子的误差为：
`ps` −3.7%，`ips` −0.5%，`hn` 和 `ihn` 小于 0.1%，排序与原文一致。

## 开发

```bash
uv sync
uv run pytest
uv run ruff check .
```

`scripts/build_atlas.py` 用于从 MNI 原始下载文件重新生成内置图谱。

## 参考文献

* López-González FJ, Silva-Rodríguez J, Paredes-Pacheco J, et al. Intensity
  normalization methods in brain FDG-PET quantification. *NeuroImage* 2020;222:117229.
* Yakushev I, Hammers A, Fellgiebel A, et al. SPM-based count normalization
  provides excellent discrimination of mild Alzheimer's disease and amnestic
  mild cognitive impairment from healthy aging. *NeuroImage* 2009;44:43–50.
* Manera AL, Dadar M, Fonov V, Collins DL. CerebrA, registration and manual
  label correction of Mindboggle-101 atlas for MNI-ICBM152 template.
  *Sci Data* 2020;7:237.
* Fonov V, Evans AC, Botteron K, et al. Unbiased average age-appropriate
  atlases for pediatric studies. *NeuroImage* 2011;54:313–327.

## 许可

代码采用 MIT 许可。内置图谱文件适用
[`src/sugar_pet/data/LICENSE-MNI`](src/sugar_pet/data/LICENSE-MNI) 中的 MNI 许可。
仅供科研使用，不是医疗器械。
