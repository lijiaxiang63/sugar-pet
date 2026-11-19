# 教程：从原始图像到分析结果

本教程会按页面顺序带你走一遍完整的分析流程。每一步都会说明具体操作、附上带数字标注的界面截图，并列出最终生成的文件。

## 示例研究背景

假设研究者 Ada 正在做一项非典型帕金森综合征的研究，手头有以下数据：

* **12 例患者**和 **16 例健康对照**的 FDG-PET 图像（每人一幅，均来自同一台扫描仪）；
* 一份 `participants.tsv`，记录了这 28 名受试者的年龄和性别；
* 所有文件都按照上一节介绍的[目录结构](../guide/prepare-data#layout)存放在 `study/` 文件夹下。

她希望通过这次分析拿到三类结果：每位患者相对于对照的异常代谢脑区图、辅助判断各患者倾向于哪种帕金森综合征的分类报告，以及患者组与对照组在全脑水平上的组间差异。

## 流程概览

<div class="flow">
  <a href="./spatial"><span class="n">第 1 步 · 可选</span><span class="t">空间标准化</span><span class="d">仅当图像还在扫描仪原始空间时需要，需配合 MATLAB 和 SPM。</span></a>
  <a href="./build-reference"><span class="n">第 2 步</span><span class="t">建立参考库</span><span class="d">将健康对照数据汇总一次，供后续大多数分析直接调用。</span></a>
  <a href="./normalize"><span class="n">第 3 步</span><span class="t">强度归一化</span><span class="d">将每幅图像除以各自因子，输出归一化图像和因子表。</span></a>
  <a href="./zmap"><span class="n">第 4 步</span><span class="t">Z 值图与 W 值图</span><span class="d">评估单个患者各个体素偏离健康对照的程度。</span></a>
  <a href="./classify"><span class="n">第 5 步</span><span class="t">帕金森综合征分类</span><span class="d">比对典型代谢模式并生成可打印的 HTML 报告。</span></a>
  <a href="./compare"><span class="n">第 6 步</span><span class="t">组间比较</span><span class="d">患者组与对照组的逐体素 t 检验及多重比较校正。</span></a>
  <a href="./reference-cluster"><span class="n">第 7 步 · 进阶</span><span class="t">参考簇</span><span class="d">基于患者组数据自动提取未受累区域，用于敏感性分析。</span></a>
  <a href="./masks"><span class="n">资源</span><span class="t">掩膜与图谱</span><span class="d">查看或导出软件内置的标准脑区掩膜和图谱标签。</span></a>
</div>

::: tip 一定要先建参考库吗？
教程的顺序和左侧边栏一致：**参考库**排在**强度归一化**前面，因为推荐的直方图归一化（HN）依赖参考库。如果只用小脑等经典参考区，并不需要健康对照数据，可以跳过建库这一步。
:::

## 按需挑选步骤

第 3 到第 7 步彼此独立，实际使用时按你的研究需求挑选即可，不必每次都从头跑到尾：

| 如果你需要… | 建议操作 |
| --- | --- |
| 导出归一化图像，拿到 SPM 等其他软件里做统计 | 第 2 步，然后[第 3 步](./normalize) |
| 同上，但只想用小脑等简单参考区（不用对照） | 直接做[第 3 步](./normalize) |
| 查看单个患者的代谢异常脑区 | 第 2 步，然后[第 4 步](./zmap) |
| 生成帕金森综合征分型辅助报告 | 第 2 步，然后[第 5 步](./classify) |
| 比较患者组与对照组的代谢差异 | 第 2 步，然后[第 6 步](./compare) |

需要注意的是，第 4、5、6 步在内部会自动完成归一化，因此不需要先跑一遍第 3 步。

## 准备工作

在开始前，请确认你已经完成：

* [安装应用](../guide/install)，并大致[熟悉了界面](../guide/interface)；
* 按照[准备数据](../guide/prepare-data)的要求整理好图像和表格。

准备好之后，就可以从[第 1 步](./spatial)开始了；如果你的图像已经配准到 MNI 空间，可以直接跳到[第 2 步](./build-reference)。
