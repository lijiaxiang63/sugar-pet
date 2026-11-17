"""Draw the flowchart of ``sugar-pet classify``, in English and Chinese.

    python scripts/classify_flowchart.py

writes docs/classify-flow-en.svg and docs/classify-flow-zh.svg, the figures
of the README.  The numbers in it are the ones of src/sugar_pet/classify.py;
change them here when they change there.
"""

from pathlib import Path
from xml.sax.saxutils import escape

OUT = Path(__file__).resolve().parent.parent / "docs"

W = 1200
INK, MUTED, LINE, PANEL = "#17202a", "#5d6b78", "#c9d1d9", "#f6f8fa"
BLUE, BLUE_L = "#2166ac", "#dce9f6"
RED, RED_L = "#c0392b", "#f9e1dd"
TEAL, TEAL_L = "#0f766e", "#e0f2ef"
AMBER, AMBER_L = "#8a5a00", "#fff3d1"
GREEN = "#1a7f37"
FONT = (
    "'Helvetica Neue', Helvetica, Arial, 'PingFang SC', 'Hiragino Sans GB', "
    "'Microsoft YaHei', 'Noto Sans CJK SC', sans-serif"
)
MONO = "'SF Mono', Menlo, Consolas, 'PingFang SC', 'Microsoft YaHei', monospace"

# The panels start here; left of it, the step and what it does.
X0, X1 = 350, 1160

TEXT = {
    "en": {
        "input": ("Input", ["One patient whose diagnosis", "is open, and the controls", "of your own scanner."]),
        "patient": ("Patient FDG-PET image", ["in MNI space", "age and sex (optional)"]),
        "controls": (
            "Healthy-control reference",
            ["30 or more controls, same scanner:", "voxel mean, SD, age and sex model"],
        ),
        "s1": ("Intensity normalisation", ["Three ways side by side:", "one to classify by,", "two to check it with."]),
        "lanes": [
            ("hn", "histogram normalisation", "needs no spared region", "main"),
            ("ps", "global mean scaling", "the classic choice", "check"),
            ("white-matter", "reference region", "does not rest on grey matter", "check"),
        ],
        "s2": (
            "Map against the controls",
            [
                "Each voxel: how far from what",
                "is expected at this age and sex,",
                "in SD of the controls (W-score;",
                "z-score without age and sex).",
            ],
        ),
        "map": "W-score map",
        "s3": ("Regional scores", ["Mean of each map in 17 regions,", "left and right on their own."]),
        "groups": [
            ("Striatum and thalamus", ["caudate · putamen · posterior putamen", "pallidum · thalamus"]),
            ("Brainstem and cerebellum", ["midbrain · pons", "cerebellar hemispheres · vermis"]),
            (
                "Cortex",
                [
                    "medial frontal · lateral frontal · peri-central",
                    "parietal · precuneus · posterior cingulate",
                    "lateral temporal · occipital",
                ],
            ),
        ],
        "example": "Example",
        "bars": ["putamen", "pons", "cerebellum", "thalamus", "parietal", "occipital"],
        "hypo": "≤ −2 hypometabolic",
        "hyper": "≥ 2 hypermetabolic",
        "s4": (
            "Does the finding hold?",
            ["A finding that only one", "normalisation shows is marked", "and cannot carry the conclusion."],
        ),
        "down": ("Decrease ↓", ["stable if both checks show it too:", "same direction, |score| ≥ 1"]),
        "up": ("Increase ↑", ["counts only if every normalisation", "shows it: the lowest score is used"]),
        "case": "putamen, case {0}",
        "stable": "stable",
        "unstable": "not stable",
        "s5": (
            "Pattern match, 0 to 100",
            [
                "How completely the regional",
                "scores show each published",
                "pattern. Computed twice:",
                "*S* from all findings,",
                "*R* from the stable ones only.",
            ],
        ),
        "evidence": "Evidence of a decrease",
        "score": "regional score",
        "core": "core 60%",
        "support": "supportive 40%",
        "worked": ["Example above, MSA-P:", "core 1.0, supportive 0.63", "match = 60 + 0.63 × 40 = *85*"],
        "twice": [
            ("S", 85, ["all findings"]),
            ("R", 25, ["stable findings only: here, if the", "putamen were not stable"]),
        ],
        "rows": [
            "putamen (posterior part first)",
            "pallidum",
            "caudate",
            "thalamus",
            "midbrain",
            "pons",
            "cerebellum",
            "medial frontal, ant. cingulate",
            "lateral frontal",
            "peri-central",
            "parietal",
            "precuneus",
            "posterior cingulate",
            "occipital",
        ],
        "legend": ["core ↓", "supportive ↓", "relative increase ↑", "preserved"],
        "notes": [
            "CBD: one hemisphere only, and clearly worse than the other.",
            "PD: and no decrease in striatum, brainstem or cerebellum.",
        ],
        "s6": (
            "Conclusion",
            [
                "*S*: best match among MSA,",
                "PSP, CBD and DLB.",
                "*Δ*: its lead over the next",
                "disease (MSA-P and MSA-C",
                "count as one).",
                "*R*: the same match from",
                "stable findings only.",
            ],
        ),
        "q": ["S ≥ 50 ?", "S ≥ 30 ?", "PD match ≥ 30 ?", "Δ ≥ 15 ?"],
        "yes": "yes",
        "no": "no",
        "assigned": ("Pattern assigned", ""),
        "indeterminate": ("Indeterminate", "both diseases named"),
        "partial": ("Partial features", "not assigned"),
        "pd": ("PD pattern", ""),
        "none": ("No pattern", "favours PD"),
        "confidence": "Confidence",
        "levels": [("high", "R ≥ 70 and Δ ≥ 30"), ("moderate", "R ≥ 50"), ("low", "R < 50")],
        "output": (
            "Output",
            [
                "Decision support, not a",
                "diagnosis: read it with the",
                "clinical findings, MRI and",
                "dopamine-transporter imaging.",
            ],
        ),
        "files": [
            ("Report", "HTML, English or Chinese"),
            ("Score map", "NIfTI"),
            ("Every number", "JSON"),
            ("Summary table", "TSV, one row per image"),
        ],
    },
    "zh": {
        "input": ("输入", ["一位诊断未明的患者，", "以及本机构同一台扫描仪的", "健康对照。"]),
        "patient": ("患者 FDG-PET 图像", ["已配准到 MNI 空间", "年龄、性别（可选）"]),
        "controls": ("健康对照参考库", ["同一扫描仪的对照 30 例以上：", "体素均值、标准差、年龄性别模型"]),
        "s1": ("强度归一化", ["三种方式并行：", "一种用于分类，", "两种用于交叉核对。"]),
        "lanes": [
            ("hn", "直方图归一化", "无需指定未受累区域", "主方法"),
            ("ps", "全局均值缩放", "经典做法", "核对"),
            ("white-matter", "白质参考区", "不依赖灰质", "核对"),
        ],
        "s2": (
            "与对照比较成图",
            ["每个体素：与同年龄、同性别者", "的预期值相差多少个对照标准差", "（W 值；不校正年龄性别时", "为 Z 值）。"],
        ),
        "map": "W 值图",
        "s3": ("各脑区评分", ["每幅图在 17 个脑区内取平均，", "左右分别计算。"]),
        "groups": [
            ("纹状体与丘脑", ["尾状核 · 壳核 · 壳核后部", "苍白球 · 丘脑"]),
            ("脑干与小脑", ["中脑 · 脑桥", "小脑半球 · 小脑蚓部"]),
            ("皮层", ["内侧额叶 · 外侧额叶 · 中央周围区", "顶叶 · 楔前叶 · 后扣带回", "颞叶外侧 · 枕叶"]),
        ],
        "example": "示例",
        "bars": ["壳核", "脑桥", "小脑", "丘脑", "顶叶", "枕叶"],
        "hypo": "≤ −2 代谢减低",
        "hyper": "≥ 2 代谢增高",
        "s4": ("换一种归一化还成立吗？", ["只在一种归一化下出现的发现", "会被标出，且不能作为", "结论的依据。"]),
        "down": ("减低 ↓", ["两种核对方式下也出现才算稳定：", "方向相同，且 |评分| ≥ 1"]),
        "up": ("增高 ↑", ["每种归一化下都出现才算数：", "取各方式中最低的评分"]),
        "case": "壳核，病例 {0}",
        "stable": "稳定",
        "unstable": "不稳定",
        "s5": (
            "模式匹配度（0–100）",
            [
                "各脑区评分在多大程度上",
                "呈现文献报道的某种模式。",
                "计算两次：",
                "*S* 用全部发现，",
                "*R* 只用稳定的发现。",
            ],
        ),
        "evidence": "减低的证据强度",
        "score": "脑区评分",
        "core": "核心特征 60%",
        "support": "支持特征 40%",
        "worked": ["以上面的示例算 MSA-P：", "核心 1.0，支持 0.63", "匹配度 = 60 + 0.63 × 40 = *85*"],
        "twice": [("S", 85, ["全部发现"]), ("R", 25, ["只用稳定的发现：这里假设", "壳核的减低不稳定"])],
        "rows": [
            "壳核（后部优先）",
            "苍白球",
            "尾状核",
            "丘脑",
            "中脑",
            "脑桥",
            "小脑",
            "内侧额叶、前扣带回",
            "外侧额叶",
            "中央周围区",
            "顶叶",
            "楔前叶",
            "后扣带回",
            "枕叶",
        ],
        "legend": ["核心 ↓", "支持 ↓", "相对增高 ↑", "相对保留"],
        "notes": ["CBD：仅一侧半球受累，且明显重于对侧。", "PD：同时要求纹状体、脑干、小脑无减低。"],
        "s6": (
            "结论",
            [
                "*S*：MSA、PSP、CBD、DLB",
                "中最高的匹配度。",
                "*Δ*：它领先下一种疾病的分数",
                "（MSA-P 与 MSA-C 视为",
                "同一种病）。",
                "*R*：只用稳定发现算出的",
                "同一匹配度。",
            ],
        ),
        "q": ["S ≥ 50 ？", "S ≥ 30 ？", "PD 匹配度 ≥ 30 ？", "Δ ≥ 15 ？"],
        "yes": "是",
        "no": "否",
        "assigned": ("判定为该模式", ""),
        "indeterminate": ("无法区分", "两种疾病都列出"),
        "partial": ("部分特征", "不作判定"),
        "pd": ("PD 模式", ""),
        "none": ("无特征性模式", "倾向 PD"),
        "confidence": "置信度",
        "levels": [("高", "R ≥ 70 且 Δ ≥ 30"), ("中等", "R ≥ 50"), ("低", "R < 50")],
        "output": ("输出", ["辅助决策，不是诊断：", "须结合临床表现、MRI 和", "多巴胺转运体显像判断。"]),
        "files": [
            ("报告", "HTML，中文或英文"),
            ("评分图", "NIfTI"),
            ("全部数值", "JSON"),
            ("汇总表", "TSV，每幅图像一行"),
        ],
    },
}

# Which region each pattern asks for: c core decrease, s supportive decrease, u increase, p preserved.
PATTERNS = ["MSA-P", "MSA-C", "PSP", "CBD", "DLB", "PD"]
MATRIX = [
    "cs.s.u",  # putamen
    ".....u",  # pallidum
    "..s...",  # caudate
    "..ss.u",  # thalamus
    "..c...",  # midbrain
    "ss...u",  # pons
    "sc...u",  # cerebellum
    "..c...",  # medial frontal
    "..ss..",  # lateral frontal
    "...c.u",  # peri-central
    "...cs.",  # parietal
    "....s.",  # precuneus
    "....p.",  # posterior cingulate
    "....c.",  # occipital
]
GROUP_ENDS = (4, 7)  # a rule after the striatum and thalamus, and after brainstem and cerebellum


class Figure:
    def __init__(self):
        self.parts = []

    def add(self, markup):
        self.parts.append(markup)

    def text(self, x, y, value, size=13, fill=INK, anchor="start", weight=400, family=FONT):
        """One line; ``*…*`` is set in bold."""
        spans = []
        for n, piece in enumerate(value.split("*")):
            if piece:
                bold = ' font-weight="700"' if n % 2 else ""
                spans.append(f"<tspan{bold}>{escape(piece)}</tspan>")
        self.add(
            f'<text x="{x:.1f}" y="{y:.1f}" font-family="{family}" font-size="{size}" font-weight="{weight}" '
            f'fill="{fill}" text-anchor="{anchor}">{"".join(spans)}</text>'
        )

    def lines(self, x, y, values, size=12.5, step=18, **style):
        for n, value in enumerate(values):
            self.text(x, y + n * step, value, size, **style)

    def box(self, x, y, w, h, fill="#ffffff", stroke=LINE, r=8, width=1.2, dash=None):
        dash = f' stroke-dasharray="{dash}"' if dash else ""
        self.add(
            f'<rect x="{x:.1f}" y="{y:.1f}" width="{w:.1f}" height="{h:.1f}" rx="{r}" fill="{fill}" '
            f'stroke="{stroke}" stroke-width="{width}"{dash}/>'
        )

    def line(self, points, stroke=MUTED, width=1.4, arrow=False, dash=None):
        path = "M" + " L".join(f"{x:.1f} {y:.1f}" for x, y in points)
        end = ' marker-end="url(#arrow)"' if arrow else ""
        dash = f' stroke-dasharray="{dash}"' if dash else ""
        self.add(f'<path d="{path}" fill="none" stroke="{stroke}" stroke-width="{width}"{end}{dash}/>')

    def arrow(self, *points):
        self.line(points, arrow=True)

    def brain(self, cx, cy, lesion=None, scale=1.0):
        """An axial slice, with the putamen in ``lesion`` colour."""
        rx, ry = 21 * scale, 26 * scale
        self.add(
            f'<ellipse cx="{cx}" cy="{cy}" rx="{rx}" ry="{ry}" fill="#e9edf1" stroke="#9aa5b1" stroke-width="1.2"/>'
        )
        self.add(f'<ellipse cx="{cx}" cy="{cy}" rx="{rx - 5 * scale}" ry="{ry - 5 * scale}" fill="#f9fafb"/>')
        self.line([(cx, cy - ry + 3), (cx, cy + ry - 3)], stroke="#c3cbd3", width=1)
        for side in (-1, 1):
            self.add(
                f'<ellipse cx="{cx + side * 7.5 * scale}" cy="{cy + 1}" rx="{3.6 * scale}" ry="{7.5 * scale}" '
                f'fill="{lesion or "#d5dbe1"}" transform="rotate({side * -14} {cx + side * 7.5 * scale} {cy + 1})"/>'
            )

    def node(self, y, label=None):
        """A stop on the line down the left: a numbered step, or an open ring for input and output."""
        if label is None:
            self.add(f'<circle cx="62" cy="{y}" r="9" fill="#ffffff" stroke="{TEAL}" stroke-width="2.4"/>')
        else:
            self.add(f'<circle cx="62" cy="{y}" r="15" fill="{TEAL}"/>')
            self.text(62, y + 5.5, label, 15, "#ffffff", "middle", 700)

    def heading(self, y, entry, label=None):
        self.node(y + 18, label)
        self.text(92, y + 24, entry[0], 16.5, weight=700)
        self.lines(92, y + 48, entry[1], 12.5, 18, fill=MUTED)

    def mark(self, x, y, kind, r=6.2):
        if kind == "c":
            self.add(f'<circle cx="{x}" cy="{y}" r="{r}" fill="{BLUE}"/>')
        elif kind == "s":
            self.add(f'<circle cx="{x}" cy="{y}" r="{r - 0.9}" fill="#ffffff" stroke="{BLUE}" stroke-width="1.8"/>')
        elif kind == "u":
            self.add(
                f'<path d="M{x} {y - r - 0.5} L{x + r + 0.8} {y + r - 0.8} L{x - r - 0.8} {y + r - 0.8} Z" fill="{RED}"/>'
            )
        elif kind == "p":
            self.add(
                f'<path d="M{x} {y - r - 1} L{x + r + 1} {y} L{x} {y + r + 1} L{x - r - 1} {y} Z" fill="#ffffff" '
                f'stroke="{TEAL}" stroke-width="1.8"/>'
            )


def draw(lang):
    t = TEXT[lang]
    f = Figure()
    mid = (X0 + X1) / 2
    lane_w, lane_gap = 256, 21
    lanes = [X0 + n * (lane_w + lane_gap) for n in range(3)]
    centres = [x + lane_w / 2 for x in lanes]
    colours = [(TEAL, TEAL_L, None), (MUTED, "#ffffff", "5 4"), (MUTED, "#ffffff", "5 4")]

    # Input.
    y = 36
    top = y
    f.heading(y, t["input"])
    for x, key, icon in ((X0, "patient", True), (X0 + 420, "controls", False)):
        f.box(x, y, 390, 84, PANEL)
        left = x + 22
        if icon:
            f.brain(x + 44, y + 42)
            left = x + 84
        else:
            for n in range(3):  # a stack of scans
                f.box(x + 22 + n * 7, y + 20 + n * 7, 34, 30, "#f9fafb", "#9aa5b1", 4)
            left = x + 92
        f.text(left, y + 31, t[key][0], 14.5, weight=700)
        f.lines(left, y + 52, t[key][1], 12.5, 17, fill=MUTED)
    y += 84
    f.line([(X0 + 195, y), (X0 + 195, y + 16), (X0 + 615, y + 16), (X0 + 615, y)])
    f.line([(centres[0], y + 16), (centres[2], y + 16)])

    # 1. Intensity normalisation.
    y += 44
    f.heading(y, t["s1"], "1")
    for x, cx, (name, what, why, tag), (stroke, fill, dash) in zip(lanes, centres, t["lanes"], colours, strict=True):
        f.arrow((cx, y - 28), (cx, y - 3))
        f.box(x, y, lane_w, 76, fill, stroke, width=1.8 if dash is None else 1.3, dash=dash)
        f.text(x + 16, y + 28, name, 15, TEAL if dash is None else INK, weight=700, family=MONO)
        f.text(x + 16, y + 48, what, 13)
        f.text(x + 16, y + 65, why, 11.5, MUTED)
        width = 14 + 12.5 * sum(1.0 if ord(c) > 0x2E80 else 0.56 for c in tag)
        f.box(x + lane_w - width - 12, y + 12, width, 21, TEAL if dash is None else "#ffffff", stroke, 10.5, 1)
        f.text(x + lane_w - 12 - width / 2, y + 27, tag, 11.5, "#ffffff" if dash is None else MUTED, "middle", 700)

    # 2. The map against the controls.
    y += 76 + 34
    f.heading(y, t["s2"], "2")
    for x, cx, (name, *_), (stroke, fill, dash), shade in zip(
        lanes, centres, t["lanes"], colours, (BLUE, "#7fa8d4", "#7fa8d4"), strict=True
    ):
        f.arrow((cx, y - 34), (cx, y - 3))
        f.box(x, y, lane_w, 84, fill, stroke, width=1.8 if dash is None else 1.3, dash=dash)
        f.brain(x + 44, y + 42, shade)
        f.text(x + 82, y + 38, t["map"], 14, weight=700)
        f.text(x + 82, y + 58, name, 12.5, MUTED, family=MONO)

    # 3. Regional scores.
    y += 84 + 34
    height = 232
    f.heading(y, t["s3"], "3")
    for cx in centres:
        f.arrow((cx, y - 34), (cx, y - 3))
    f.box(X0, y, X1 - X0, height, PANEL)
    row = y + 30
    for title, members in t["groups"]:
        f.text(X0 + 22, row, title, 13, weight=700)
        f.lines(X0 + 22, row + 19, members, 12.5, 17.5, fill=MUTED)
        row += 19 + 17.5 * len(members) + 14
    # An example of regional scores, with the two thresholds.
    left, unit = 890, 34.0
    zero, first = left + 4 * unit, y + 46
    f.box(770, y + 14, 376, height - 28, "#ffffff")
    f.text(784, y + 34, t["example"], 11.5, MUTED, weight=700)
    f.add(f'<rect x="{left}" y="{first - 8}" width="{2 * unit}" height="118" fill="{BLUE_L}"/>')
    f.add(f'<rect x="{zero + 2 * unit}" y="{first - 8}" width="{unit}" height="118" fill="{RED_L}"/>')
    for value, colour in ((-2, BLUE), (2, RED)):
        f.line(
            [(zero + value * unit, first - 8), (zero + value * unit, first + 110)], stroke=colour, width=1.2, dash="4 3"
        )
    f.line([(zero, first - 8), (zero, first + 110)], stroke="#9aa5b1", width=1)
    for n, (name, value) in enumerate(zip(t["bars"], (-3.1, -2.2, -1.7, -0.4, 0.3, 0.6), strict=True)):
        mid_y = first + n * 18 + 4
        f.text(left - 8, mid_y + 4.5, name, 12, anchor="end")
        colour = BLUE if value <= -2 else "#8fb4dc" if value <= -1.5 else "#aab4bf"
        x = zero + min(value, 0) * unit
        f.add(
            f'<rect x="{x:.1f}" y="{mid_y - 5.5}" width="{abs(value) * unit:.1f}" height="11" rx="2" fill="{colour}"/>'
        )
        shown = f"{value:.1f}".replace("-", "−")
        f.text(zero + (6 if value < 0 else -6), mid_y + 4, shown, 11, MUTED, "start" if value < 0 else "end")
    for value in (-4, -2, 0, 2):
        f.text(zero + value * unit, first + 124, str(value).replace("-", "−"), 11, MUTED, "middle")
    f.text(left, first + 146, t["hypo"], 12, BLUE, weight=700)
    f.text(zero + 3 * unit, first + 146, t["hyper"], 12, RED, "end", 700)

    # 4. Stable under every normalisation?
    y += height + 34
    height = 172
    f.heading(y, t["s4"], "4")
    f.arrow((mid, y - 34), (mid, y - 3))
    f.box(X0, y, X1 - X0, height, PANEL)
    for n, (key, colour, tint) in enumerate((("down", BLUE, BLUE_L), ("up", RED, RED_L))):
        top_card = y + 14 + n * 74
        f.box(X0 + 14, top_card, 392, 66, "#ffffff")
        f.add(f'<rect x="{X0 + 14}" y="{top_card}" width="92" height="66" rx="8" fill="{tint}"/>')
        f.text(X0 + 60, top_card + 38, t[key][0], 13.5, colour, "middle", 700)
        f.lines(X0 + 120, top_card + 28, t[key][1], 12.5, 18)
    f.box(770, y + 14, 376, height - 28, "#ffffff")
    columns = [(948, "hn"), (1000, "ps"), (1066, "white-matter")]
    for x, name in columns:
        f.text(x, y + 40, name, 11.5, MUTED, "middle", 700, MONO)
    for n, (values, good) in enumerate((((-2.7, -2.1, -1.8), True), ((-2.7, -2.1, 5.4), False))):
        base = y + 74 + n * 50
        f.line([(784, base - 22), (1132, base - 22)], stroke=LINE, width=1)
        f.text(784, base, t["case"].format(n + 1), 12.5)
        for (x, _), value in zip(columns, values, strict=True):
            shown = f"{value:.1f}".replace("-", "−")
            f.text(x, base, shown, 13, BLUE if value <= -1 else RED, "middle", 700)
        label = ("✓ " if good else "✗ ") + t["stable" if good else "unstable"]
        f.text(1132, base + 21, label, 12.5, GREEN if good else AMBER, "end", 700)

    # 5. Pattern match.
    y += height + 34
    height = 502
    f.heading(y, t["s5"], "5")
    f.arrow((mid, y - 34), (mid, y - 3))
    f.box(X0, y, X1 - X0, height, PANEL)
    # How a regional score becomes evidence, and how evidence becomes a match.
    f.box(X0 + 14, y + 14, 262, height - 28, "#ffffff")
    f.text(X0 + 30, y + 40, t["evidence"], 13, weight=700)
    ax, ay, aw, ah = X0 + 56, y + 62, 196, 96
    f.line([(ax, ay), (ax, ay + ah), (ax + aw, ay + ah)], stroke="#9aa5b1", width=1.2)
    scale = aw / 3.5  # the axis runs from 0 down to -3.5
    f.line([(ax, ay + ah), (ax + scale, ay + ah), (ax + 2.5 * scale, ay), (ax + aw, ay)], stroke=BLUE, width=2.6)
    for value in (1, 2.5):
        f.line([(ax + value * scale, ay + ah), (ax + value * scale, ay + ah + 5)], stroke="#9aa5b1", width=1.2)
        f.text(ax + value * scale, ay + ah + 19, f"−{value:g}", 11.5, MUTED, "middle")
    f.line([(ax + 2.5 * scale, ay), (ax + 2.5 * scale, ay + ah)], stroke=LINE, width=1, dash="3 3")
    f.text(ax - 8, ay + 4, "1", 11.5, MUTED, "end")
    f.text(ax - 8, ay + ah + 4, "0", 11.5, MUTED, "end")
    f.text(ax + aw, ay + ah + 37, t["score"] + " →", 11.5, MUTED, "end")
    bar = y + 232
    f.add(f'<rect x="{X0 + 30}" y="{bar}" width="138" height="30" rx="5" fill="{BLUE}"/>')
    f.add(
        f'<rect x="{X0 + 168}" y="{bar}" width="92" height="30" rx="5" fill="#ffffff" stroke="{BLUE}" stroke-width="1.8"/>'
    )
    f.text(X0 + 99, bar + 20, t["core"], 12, "#ffffff", "middle", 700)
    f.text(X0 + 214, bar + 20, t["support"], 11.5, BLUE, "middle", 700)
    f.line([(X0 + 30, bar + 52), (X0 + 260, bar + 52)], stroke=LINE, width=1)
    f.lines(X0 + 30, bar + 78, t["worked"], 12.5, 20, fill=INK)
    f.line([(X0 + 30, bar + 136), (X0 + 260, bar + 136)], stroke=LINE, width=1)
    row = bar + 162
    for name, value, about in t["twice"]:
        f.text(X0 + 30, row + 4.5, name, 13.5, TEAL, weight=700)
        f.add(f'<rect x="{X0 + 50}" y="{row - 6}" width="170" height="12" rx="6" fill="#e9edf1"/>')
        f.add(f'<rect x="{X0 + 50}" y="{row - 6}" width="{1.7 * value}" height="12" rx="6" fill="{TEAL}"/>')
        f.text(X0 + 260, row + 4.5, str(value), 13, anchor="end", weight=700)
        f.lines(X0 + 50, row + 23, about, 11.5, 15, fill=MUTED)
        row += 28 + 15 * len(about)
    # Which regions each pattern asks for.
    mx, label_w, cell = 640, 196, 48
    f.box(mx, y + 14, X1 - 14 - mx, height - 28, "#ffffff")
    grid = mx + 12 + label_w
    for n, name in enumerate(PATTERNS):
        f.text(grid + n * cell + cell / 2, y + 42, name, 12.5, weight=700, anchor="middle")
    first = y + 56
    f.line([(mx + 12, first), (X1 - 26, first)], stroke="#9aa5b1", width=1.2)
    for r, (name, cells) in enumerate(zip(t["rows"], MATRIX, strict=True)):
        mid_y = first + 13 + r * 24
        if r % 2 == 0:
            f.add(f'<rect x="{mx + 12}" y="{mid_y - 12}" width="{X1 - 38 - mx}" height="24" fill="#f6f8fa"/>')
        f.text(mx + 20, mid_y + 4.5, name, 12.5)
        for c, kind in enumerate(cells):
            f.mark(grid + c * cell + cell / 2, mid_y, kind)
        if r + 1 in GROUP_ENDS:
            f.line([(mx + 12, mid_y + 12), (X1 - 26, mid_y + 12)], stroke="#9aa5b1", width=1)
    last = first + 24 * len(MATRIX)
    f.line([(mx + 12, last), (X1 - 26, last)], stroke="#9aa5b1", width=1.2)
    x = mx + 22
    for kind, label in zip("csup", t["legend"], strict=True):
        f.mark(x + 6, last + 22, kind, 5.4)
        f.text(x + 18, last + 26.5, label, 12, MUTED)
        x += 30 + 12 * sum(1.0 if ord(ch) > 0x2E80 else 0.54 for ch in label)
    f.lines(mx + 20, last + 50, t["notes"], 11.5, 16, fill=MUTED)

    # 6. Conclusion.
    y += height + 34
    height = 340
    f.heading(y, t["s6"], "6")
    f.arrow((mid, y - 34), (mid, y - 3))
    f.box(X0, y, X1 - X0, height, PANEL)
    qw, qh = 150, 42
    q1, q2, q3, end = X0 + 125, X0 + 345, X0 + 555, X0 + 735
    r1, r2, r3 = y + 46, y + 132, y + 222

    def question(cx, cy, label):
        f.box(cx - qw / 2, cy - qh / 2, qw, qh, "#ffffff", INK, 21, 1.5)
        f.text(cx, cy + 5, label, 13.5, anchor="middle", weight=700)

    def outcome(cx, cy, entry, fill, stroke, colour, w=150):
        f.box(cx - w / 2, cy - 23, w, 46, fill, stroke, 8, 1.5)
        if entry[1]:
            f.text(cx, cy - 2, entry[0], 13.5, colour, "middle", 700)
            f.text(cx, cy + 15, entry[1], 11.5, colour, "middle")
        else:
            f.text(cx, cy + 5, entry[0], 13.5, colour, "middle", 700)

    for a, b in ((q1, q2), (q2, q3)):
        f.arrow((a + qw / 2, r1), (b - qw / 2 - 3, r1))
        f.text((a + b) / 2, r1 - 7, t["no"], 12, MUTED, "middle", 700)
    f.arrow((q3 + qw / 2, r1), (end - 68, r1))
    f.text((q3 + qw / 2 + end - 65) / 2, r1 - 8, t["no"], 12, MUTED, "middle", 700)
    for cx in (q1, q2, q3):
        f.arrow((cx, r1 + qh / 2), (cx, r2 - (qh / 2 if cx == q1 else 23) - 3))
        f.text(cx + 8, (r1 + r2) / 2 + 3, t["yes"], 12, MUTED, weight=700)
    question(q1, r1, t["q"][0])
    question(q2, r1, t["q"][1])
    question(q3, r1, t["q"][2])
    question(q1, r2, t["q"][3])
    outcome(end, r1, t["none"], "#ffffff", "#9aa5b1", MUTED, 130)
    outcome(q2, r2, t["partial"], AMBER_L, "#d4a72c", AMBER)
    outcome(q3, r2, t["pd"], RED_L, RED, RED)
    left, right = q1 - 25, q1 + 155
    fork = r2 + qh / 2 + 16
    f.line([(q1, r2 + qh / 2), (q1, fork)])
    f.arrow((q1, fork), (left, fork), (left, r3 - 26))
    f.arrow((q1, fork), (right, fork), (right, r3 - 26))
    f.text(left - 8, fork + 20, t["yes"], 12, MUTED, "end", 700)
    f.text(right + 8, fork + 20, t["no"], 12, MUTED, weight=700)
    outcome(left, r3, t["assigned"], TEAL, TEAL, "#ffffff", 156)
    outcome(right, r3, t["indeterminate"], AMBER_L, "#d4a72c", AMBER, 170)
    strip = y + 276
    f.arrow((left, r3 + 23), (left, strip - 3))
    f.box(X0 + 14, strip, X1 - X0 - 28, 48, "#ffffff", TEAL, 8, 1.5)
    f.text(X0 + 32, strip + 29, t["confidence"], 13.5, TEAL, weight=700)
    x = X0 + 32 + 20 + 13.5 * sum(1.0 if ord(ch) > 0x2E80 else 0.56 for ch in t["confidence"])
    for name, rule in t["levels"]:
        width = 22 + 12.5 * sum(1.0 if ord(ch) > 0x2E80 else 0.58 for ch in name)
        f.box(x, strip + 12, width, 24, TEAL_L, TEAL_L, 12)
        f.text(x + width / 2, strip + 28.5, name, 12.5, TEAL, "middle", 700)
        f.text(x + width + 8, strip + 28.5, rule, 12.5)
        x += width + 8 + 12.5 * sum(1.0 if ord(ch) > 0x2E80 else 0.56 for ch in rule) + 26

    # Output.
    y += height + 34
    f.heading(y, t["output"])
    f.arrow((mid, y - 34), (mid, y - 3))
    width = (X1 - X0 - 3 * 16) / 4
    for n, (name, kind) in enumerate(t["files"]):
        x = X0 + n * (width + 16)
        f.box(x, y, width, 62, TEAL_L if n == 0 else PANEL, TEAL if n == 0 else LINE, width=1.8 if n == 0 else 1.2)
        f.text(x + width / 2, y + 27, name, 14, TEAL if n == 0 else INK, "middle", 700)
        f.text(x + width / 2, y + 46, kind, 11.5, MUTED, "middle")
    bottom = y + 126

    # The line down the left, behind its stops.
    spine = f'<path d="M62 {top + 18} L62 {y + 18}" stroke="{TEAL}" stroke-width="2.4" stroke-opacity="0.35"/>'
    head = (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {bottom}" width="{W}" height="{bottom}">'
        f'<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" '
        f'orient="auto-start-reverse"><path d="M0 0 L10 5 L0 10 Z" fill="{MUTED}"/></marker></defs>'
        f'<rect width="{W}" height="{bottom}" fill="#ffffff"/>'
    )
    return head + spine + "\n" + "\n".join(f.parts) + "\n</svg>\n"


if __name__ == "__main__":
    OUT.mkdir(exist_ok=True)
    for lang in TEXT:
        path = OUT / f"classify-flow-{lang}.svg"
        path.write_text(draw(lang), encoding="utf-8")
        print(path)
