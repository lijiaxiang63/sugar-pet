# 命令行对照

桌面应用本质上是 **sugar-pet** Python 包的图形界面。两套工具的底层算法和输出格式完全一致，文件可以随时互通——比如在桌面端建好参考库，再拿到服务器脚本里调用，或者反过来。

当你需要批量处理大样本、在服务器跑流程，或者把归一化嵌入自己的 Python 脚本时，命令行会更顺手。

## 安装

```bash
pip install git+https://github.com/lijiaxiang63/sugar-pet
```

需要 Python 3.10 及以上版本。如果要用到组间比较功能，还需要安装包含统计依赖 [GrouVox](https://github.com/lijiaxiang63/GrouVox) 的版本：

```bash
pip install "sugar-pet[stats] @ git+https://github.com/lijiaxiang63/sugar-pet"
```

## 页面与命令对照

| 桌面应用页面 | 对应命令 |
| --- | --- |
| 空间标准化 | `sugar-pet spatial-norm` |
| 强度归一化 | `sugar-pet normalize` |
| 参考库 | `sugar-pet build-reference` |
| Z 值图 | `sugar-pet zmap` |
| 帕金森综合征分类 | `sugar-pet classify` |
| 组间比较 | `sugar-pet compare` |
| 参考簇 | `sugar-pet reference-cluster` |
| 掩膜与图谱 | `sugar-pet masks`、`sugar-pet labels`、`sugar-pet methods` |

运行 `sugar-pet <命令> --help` 可以查看每个子命令的完整参数。

## 用脚本复现教程流程

在 `study/` 目录下，下面几行命令就能跑完[教程](../tutorial/)里的全部步骤：

```bash
# 第 1 步（可选）：空间标准化
sugar-pet spatial-norm raw/*_pet.nii.gz -o patients/ --spm-path /path/to/spm12 --jobs 4

# 第 2 步：建立参考库
sugar-pet build-reference controls/*.nii.gz -o hc_reference/ \
    --covariates participants.tsv --stats-for occipital,white-matter

# 第 3 步：强度归一化
sugar-pet normalize patients/*.nii.gz -m hn,cerebellum -r hc_reference/ -o normalised/

# 第 4 步：生成 W 值图
sugar-pet zmap patients/*.nii.gz -m hn -r hc_reference/ -o zmaps/ --covariates participants.tsv

# 第 5 步：帕金森综合征分类
sugar-pet classify patients/*.nii.gz -r hc_reference/ -o reports/ --covariates participants.tsv --lang zh

# 第 6 步：组间比较
sugar-pet compare --group1 patients/*.nii.gz --group2 controls/*.nii.gz \
    -m hn -r hc_reference/ -o compare/ --covariates participants.tsv --name patients_vs_controls

# 第 7 步：提取参考簇
sugar-pet reference-cluster patients/*.nii.gz -r hc_reference/ -o reference_cluster.nii.gz
```

## 界面设置与 CLI 参数对照

| 桌面应用中的设置 | 对应命令行参数 |
| --- | --- |
| 勾选的方法 | `-m hn,cerebellum`（或 `-m all`） |
| 参考库 | `-r hc_reference/` |
| 输出文件夹 | `-o out/` |
| 协变量表、*校正的列* | `--covariates participants.tsv`、`--adjust-for age,sex` |
| 平滑 | `--fwhm 8` |
| 自选区域：内置掩膜或掩膜文件 | `-m region --mask brainstem`、`--mask my_mask.nii.gz` |
| 自选区域：图谱标签 | `-m region --labels 34,85` |
| 区域统计量 | `--statistic median` |
| 目标值 | `--target 50` |
| 输出归一化图像（关闭） | `--factors-only` |
| 保存掩膜 | `--save-masks` |
| 剔除体素的 p 值、剔除范围、重新估计次数 | `--p-threshold 0.01`、`--exclude hypo`、`--max-iter 1` |
| iPS 用普通均值 | `--literal-ips` |
| 其他方法的 Z 值图 | `--stats-for occipital,white-matter` |
| 多重比较校正 | `--correction grf`（`--voxel-p`、`--cluster-p`）、`fdr`（`--q`）、`none` |
| 检验的体素 | `--analysis-mask gray-matter` |
| 保留参与检验的图像 | `--keep-images` |
| 报告语言 | `--lang zh` |

## 在 Python 代码中调用

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

完整的命令行参数说明与 Python API 文档，请查阅
[README](https://github.com/lijiaxiang63/sugar-pet/blob/main/README_zh.md)。
