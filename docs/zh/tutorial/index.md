# 教程：从原始图像到分析结果

本教程按页面顺序带你走一遍完整的分析流程。每一章说明要怎么操作，附上带数字标注的界面截图，并列出该页面写出的文件。

## 示例研究

假设 Ada 正在做一项非典型帕金森综合征的研究，手头有：

* **6 例患者**和 **16 例健康对照**的 FDG-PET 图像，每人一幅，来自同一台扫描仪，还在扫描仪的原始空间；
* 一份 `participants.tsv`，记录了这 22 名受试者的年龄和性别。

她想得到三类结果：每位患者相对于对照的异常代谢脑区图、判断每例患者更像哪种帕金森综合征的分类报告，以及患者组与对照组在全脑水平的差异。之后门诊又来了一位新患者，她想用同一组对照为这例图像分类。

## 各章内容

<div class="flow">
  <a href="./project"><span class="n">1</span><span class="t">建立项目</span><span class="d">一个文件夹存放图像和所有分析结果。添加患者、至少 3 例健康对照以及年龄和性别表。</span></a>
  <a href="./spatial"><span class="n">2 · 仅限原始空间</span><span class="t">空间标准化</span><span class="d">仅当图像还在扫描仪原始空间时需要，需配合 MATLAB 和 SPM。</span></a>
  <a href="./normalize"><span class="n">3 · 第 1 步</span><span class="t">强度归一化</span><span class="d">先汇总对照，再把每幅图像除以各自的因子。</span></a>
  <a href="./zmap"><span class="n">4 · 第 2 步</span><span class="t">Z 值图与 W 值图</span><span class="d">患者每个体素偏离对照多远。</span></a>
  <a href="./classify"><span class="n">5 · 第 3 步</span><span class="t">帕金森综合征分类</span><span class="d">把每例患者的图与典型综合征比对，并生成报告。</span></a>
  <a href="./quick"><span class="n">6</span><span class="t">快速分类</span><span class="d">项目之外再来一例患者，用本项目的对照分类，但不加入项目。</span></a>
  <a href="./compare"><span class="n">7</span><span class="t">组间比较</span><span class="d">患者组与对照组的逐体素双样本 t 检验。</span></a>
  <a href="./reference-cluster"><span class="n">8 · 进阶</span><span class="t">参考簇</span><span class="d">由数据得出的参考区，用于敏感性分析。</span></a>
  <a href="./masks"><span class="n">资源</span><span class="t">掩膜与图谱</span><span class="d">内置脑区与图谱标签。</span></a>
</div>

## 第 1、2、3 步

强度归一化、Z 值图和帕金森综合征分类是**个案分析**的三步，软件也这样给它们编号：本教程里说的*第 1 步*、*第 2 步*、*第 3 步*指这三个页面，而不是教程的章节。每一步读取上一步写出的文件，把从上一步沿用的内容展示出来而不再重复询问，并且要等上一步运行完才能开始。

组间比较和参考簇同样建立在第 1 步之上：组间比较检验的是第 1 步归一化后的图像，二者也都和第 1 步汇总的对照比较。快速分类则为项目之外的一例患者走完全部三步。

## 每例患者都与本项目的对照比较

每个项目都有自己的健康对照，至少 3 例。第 1 步把它们汇总成**对照统计**，即每个体素的均值和离散程度，此后每例患者都与之比较：Z 值图、分类、组间比较和快速分类都建立在同一组对照上。不需要手动建立或导入任何东西；对照或设置改变后，下一次运行会重新汇总。

## 不一定要做完每一章

| 目标 | 需要的章节 |
| --- | --- |
| 得到归一化图像，交给其他统计软件 | [项目](./project)，然后[第 1 步](./normalize) |
| 看每例患者哪里异常 | [项目](./project)、[第 1 步](./normalize)和[第 2 步](./zmap) |
| 给患者出分类报告 | [项目](./project)、[第 1 步](./normalize)、[第 2 步](./zmap)和[第 3 步](./classify) |
| 之后再为一例新患者分类 | 先完成上一行，再对每例新图像做[快速分类](./quick) |
| 比较患者组与对照组 | [项目](./project)、[第 1 步](./normalize)，然后[组间比较](./compare) |

原始空间的图像在以上每一行都要先经过[空间标准化](./spatial)。

## 开始之前

* 已经[安装](../guide/install)好软件，并[认识了界面](../guide/interface)。
* 已经[准备好数据](../guide/prepare-data)。

从[建立项目](./project)开始。
