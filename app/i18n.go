package main

import "fmt"

// chinese is whether the interface is shown in Chinese (else English).
var chinese bool

// T picks the text in the language shown.
func T(en, zh string) string {
	if chinese {
		return zh
	}
	return en
}

// Tf is T with fmt.Sprintf.
func Tf(en, zh string, args ...any) string { return fmt.Sprintf(T(en, zh), args...) }

func toString(v any) string { return fmt.Sprint(v) }

// uiLang is the language of the app, as the reports name it.
func uiLang() string {
	if chinese {
		return "zh"
	}
	return "en"
}

type label struct{ en, zh string }

func (l label) String() string { return T(l.en, l.zh) }

// Names and one-line descriptions of the methods and masks, as shown.
var methodLabels = map[string][2]label{
	"whole-brain":   {{"Whole brain", "全脑"}, {"Mean uptake of the brain parenchyma (gray + white matter)", "全脑实质（灰质 + 白质）的平均摄取"}},
	"cerebellum":    {{"Cerebellum", "全小脑"}, {"Mean uptake of the whole cerebellum (RRC in the paper)", "全小脑的平均摄取（文中的 RRC）"}},
	"cerebellar-gm": {{"Cerebellar gray matter", "小脑灰质"}, {"Mean uptake of the cerebellar gray matter", "小脑灰质的平均摄取"}},
	"pons":          {{"Pons", "脑桥"}, {"Mean uptake of the central pons", "脑桥中央的平均摄取"}},
	"white-matter":  {{"White matter", "深部白质"}, {"Mean uptake of the deep cerebral white matter", "大脑深部白质的平均摄取"}},
	"occipital":     {{"Occipital cortex", "枕叶皮层"}, {"Mean uptake of the occipital cortex", "枕叶皮层的平均摄取"}},
	"sensorimotor":  {{"Sensorimotor cortex", "感觉运动皮层"}, {"Mean uptake of the primary sensorimotor cortex", "初级感觉运动皮层的平均摄取"}},
	"region":        {{"Custom region", "自选区域"}, {"Mean uptake of a region you choose: a built-in mask, a mask file or atlas labels", "任意自选区域的平均摄取：内置掩膜、掩膜文件或图谱标签"}},
	"ps":            {{"Proportional scaling (PS)", "比例缩放 PS"}, {"Mean uptake inside the gray-matter mask", "灰质掩膜内的平均摄取"}},
	"spm-global":    {{"SPM global mean", "SPM 全局均值"}, {"Mean of all voxels above 1/8 of the image mean", "高于全图均值 1/8 的所有体素的均值"}},
	"hn":            {{"Histogram (HN)", "直方图归一化 HN"}, {"Image divided by the control mean; the most frequent ratio is the factor", "图像除以对照均值图，取比值中出现最多的值"}},
	"ips":           {{"Iterative PS (iPS)", "迭代比例缩放 iPS"}, {"PS, then repeated without the hypometabolic voxels", "先做 PS，去掉低代谢体素后重做"}},
	"ihn":           {{"Iterative HN (iHN)", "迭代直方图 iHN"}, {"HN, then repeated without the hypometabolic voxels", "先做 HN，去掉低代谢体素后重做"}},
}

// methodHints is the line under the name of a method on its tile: what the
// name leaves out, in a few words, and the same in both languages.  The
// tooltip of the tile is the whole description (methodLabels).
var methodHints = map[string]label{
	"hn":            {"Peak ratio to the controls", "与对照比值的直方图主峰"},
	"ihn":           {"HN again, without abnormal voxels", "去掉异常体素后重做 HN"},
	"ips":           {"PS again, without abnormal voxels", "去掉异常体素后重做 PS"},
	"ps":            {"Inside the gray-matter mask", "灰质掩膜内的摄取"},
	"spm-global":    {"Voxels above 1/8 of the mean", "高于全图均值 1/8 的体素"},
	"whole-brain":   {"Gray and white matter", "灰质和白质"},
	"cerebellum":    {"Gray and white matter", "含灰质和白质"},
	"cerebellar-gm": {"Hemispheres and vermis", "含半球和蚓部"},
	"pons":          {"Central part, eroded", "中央部分，经腐蚀"},
	"white-matter":  {"Centrum semiovale, eroded", "半卵圆中心，经腐蚀"},
	"occipital":     {"Lateral and medial surfaces", "外侧面和内侧面"},
	"sensorimotor":  {"Pre-, post- and paracentral", "中央前回、中央后回、旁中央小叶"},
	"region":        {"Any mask, or atlas labels", "任意掩膜或图谱标签"},
}

var maskLabels = map[string][2]label{
	"gray-matter":    {{"Gray matter", "灰质"}, {"Gray matter (probability > 0.1); the paper's mask for PS and HN", "灰质（概率 > 0.1），文中 PS 和 HN 所用的掩膜"}},
	"parenchyma":     {{"Brain parenchyma", "脑实质"}, {"Whole brain parenchyma (gray + white matter probability > 0.5)", "全脑实质（灰质 + 白质概率 > 0.5）"}},
	"cerebellum":     {{"Cerebellum", "全小脑"}, {"Whole cerebellum, gray and white matter (RRC in the paper)", "全小脑，含灰质和白质（文中的 RRC）"}},
	"cerebellar-gm":  {{"Cerebellar gray matter", "小脑灰质"}, {"Cerebellar gray matter, hemispheres and vermis", "小脑灰质，含半球和蚓部"}},
	"pons":           {{"Pons", "脑桥"}, {"Central pons (brainstem between z = -44 and -26 mm, eroded)", "脑桥中央（z = -44 至 -26 mm 的脑干，经腐蚀）"}},
	"white-matter":   {{"White matter", "深部白质"}, {"Deep cerebral white matter (eroded, centrum semiovale)", "大脑深部白质（经腐蚀，半卵圆中心）"}},
	"occipital":      {{"Occipital cortex", "枕叶皮层"}, {"Occipital cortex (lateral occipital, cuneus, pericalcarine, lingual)", "枕叶皮层（外侧枕叶、楔叶、距状沟周围、舌回）"}},
	"sensorimotor":   {{"Sensorimotor cortex", "感觉运动皮层"}, {"Primary sensorimotor cortex (pre-, post- and paracentral)", "初级感觉运动皮层（中央前回、中央后回、旁中央小叶）"}},
	"brainstem":      {{"Brainstem", "脑干"}, {"Whole brainstem (RRBS in the paper)", "全脑干（文中的 RRBS）"}},
	"temporal-left":  {{"Left temporal cortex", "左侧颞叶皮层"}, {"Left temporal cortex (RRTL for a right-sided lesion)", "左侧颞叶皮层（病灶在右侧时的 RRTL）"}},
	"temporal-right": {{"Right temporal cortex", "右侧颞叶皮层"}, {"Right temporal cortex (RRTL for a left-sided lesion)", "右侧颞叶皮层（病灶在左侧时的 RRTL）"}},
}

func methodName(id string) string  { return methodLabels[id][0].String() }
func methodAbout(id string) string { return methodLabels[id][1].String() }
func methodHint(id string) string  { return methodHints[id].String() }
func maskName(id string) string    { return maskLabels[id][0].String() }
func maskAbout(id string) string   { return maskLabels[id][1].String() }
