---
layout: home
title: SUGAR-PET
titleTemplate: 脑 FDG-PET 强度归一化

hero:
  name: SUGAR-PET
  text: 脑 FDG-PET 强度归一化
  tagline: 跨平台桌面应用（macOS / Windows / Linux），无需配置 Python 环境，带你从原始图像一步步完成分析。
  image:
    light: /screenshots/zh/normalize.png
    dark: /screenshots/zh/normalize-dark.png
    alt: SUGAR-PET 的“强度归一化”页面
  actions:
    - theme: brand
      text: 开始教程
      link: /zh/tutorial/
    - theme: alt
      text: 安装指南
      link: /zh/guide/install
    - theme: alt
      text: 下载应用
      link: https://github.com/lijiaxiang63/sugar-pet/releases/latest

features:
  - title: 13 种归一化方法
    details: 涵盖经典参考区法、比例缩放（PS），以及 López-González 等（2020）提出的直方图法（HN、iPS、iHN）。
    link: /zh/reference/methods
    linkText: 查看方法说明
  - title: 自建健康对照参考库
    details: 只需将健康对照组汇总一次，后续所有患者都可以直接比对，还支持校正年龄和性别。
    link: /zh/tutorial/build-reference
    linkText: 建立参考库
  - title: 个体化 Z 值图与 W 值图
    details: 逐体素评估单个患者的脑代谢相对于健康对照的偏离程度。
    link: /zh/tutorial/zmap
    linkText: 生成 Z 值图
  - title: 帕金森综合征辅助分类
    details: 将患者的代谢分布与 MSA、PSP、CBD、DLB 和 PD 的典型模式比对，生成可打印的 HTML 报告。
    link: /zh/tutorial/classify
    linkText: 了解分类流程
  - title: 组间统计比较
    details: 自动完成归一化、平滑与双样本 t 检验，支持 GRF 和 FDR 多重比较校正并输出团块报告。
    link: /zh/tutorial/compare
    linkText: 进行组间比较
  - title: 与命令行完全互通
    details: 桌面端与 sugar-pet Python 包读写同一套文件格式，既可以单独使用，也可以随时混用。
    link: /zh/reference/cli
    linkText: 命令行对照
---
