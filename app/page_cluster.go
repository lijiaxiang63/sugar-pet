package main

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"sugarpet/core"
)

type clusterPage struct {
	files     fileList
	reference string
	output    string

	st     status
	saved  string
	volume float64
	pic    picture
}

func (p *clusterPage) init() {}
func (p *clusterPage) problem() string {
	switch {
	case len(p.files.files) < 2:
		return T("Add at least 2 images of the patient group.", "请添加至少 2 幅患者组图像。")
	case p.reference == "":
		return T("Choose a reference, or build one on the Reference page.", "请选择参考库，或先到“参考库”页面建立。")
	case p.output == "":
		return T("Choose where to save the mask.", "请选择掩膜的保存位置。")
	}
	return ""
}

func (a *app) clusterView(c *ui.Context) {
	p := &a.cluster
	a.frame(c, T("Reference Cluster", "参考簇"),
		T("A reference region found from the data of a patient group", "从患者组数据中找出的参考区"),
		func() {
			section(c, T("Patient group", "患者组"), T("After scaling by the global mean, regions spared by the disease look hypermetabolic. A t-test against the controls picks them out (Yakushev et al., 2009).",
				"按全局均值缩放后，未受疾病累及的区域会显得高代谢，与对照做 t 检验即可找出这些区域（Yakushev 等，2009）。"), func() {
				a.filesCard(c, "cl-files", &p.files, T("Patient images", "患者图像"), T("at least 2 images, all on the same grid", "至少 2 幅，须在同一网格上"))
			})
			section(c, T("Reference and output", "参考库与输出"), "", func() {
				rows(c,
					func() {
						a.pathSetting(c, "cl-ref", T("Reference", "参考库"), T("The healthy controls the patients are compared with.", "与患者比较的健康对照。"),
							&p.reference, pathFolder, nil, func() { a.setReference(p.reference) })
					},
					func() {
						a.pathSetting(c, "cl-out", T("Save mask as", "掩膜保存为"), T("A NIfTI file, e.g. cluster.nii.gz.", "NIfTI 文件，例如 cluster.nii.gz。"),
							&p.output, pathSave, nil, nil)
					},
				)
				a.optionsLine(c, "cluster", "cluster", groupCluster)
			})
			caution(c, T("The mask depends on the group it came from. Using it to compare that same group with controls is circular: derive it from other patients, or cross-validate.",
				"掩膜依赖于产生它的患者组。再用它去比较同一组患者与对照属于循环论证：请用另一批患者得出掩膜，或做交叉验证。"))
			if p.saved != "" {
				a.results(c, T("Result", "结果"), "", func() {
					stats(c,
						[2]string{strconv.FormatFloat(p.volume, 'f', 0, 64) + " mL", T("Volume of the reference cluster", "参考簇体积")},
						[2]string{filepath.Base(p.saved), shortDir(filepath.Dir(p.saved))},
					)
					a.show(&p.pic, p.saved, overlayPicture(p.saved, core.PictureOptions{Solid: true}))
					a.figure(c, &p.pic, filepath.Base(p.saved), legend{kind: legendMask, text: T("Reference cluster", "参考簇")})
					ui.Row(c).Gap(8).Children(func() {
						b := button(c, T("Normalise Intensity with This Region", "用该区域做强度归一化"), nil, buttonPrimary, 26)
						if b.Clicked() {
							a.norm.methods = map[string]bool{"region": true}
							a.norm.region.kind, a.norm.region.file = 1, p.saved
							a.page = "normalize"
						}
						revealButton(c, "cl-reveal", revealLabel(), p.saved)
					})
				})
			}
		},
		func() { a.runBar(c, "cluster", p.problem(), T("Find Cluster", "查找参考簇"), &p.st, a.runCluster) },
	)
}

func (a *app) runCluster() {
	p := &a.cluster
	files, reference, out := append([]string(nil), p.files.files...), p.reference, p.output
	if !core.IsNiftiName(out) {
		out = strings.TrimSuffix(out, filepath.Ext(out)) + ".nii.gz"
	}
	opt := core.ClusterOptions{PThreshold: a.opt.ClusterThreshold, MinClusterML: a.opt.MinCluster}
	p.saved = ""
	a.start("cluster", T("Comparing the patients with the controls", "正在比较患者与对照"), func(ctx context.Context, step func(int, int)) (func(), error) {
		ref, err := core.LoadReference(reference)
		if err != nil {
			return nil, err
		}
		opt.Progress = step
		step(0, len(files))
		cluster, err := core.ReferenceCluster(ctx, files, ref, opt)
		if err != nil {
			return nil, err
		}
		if err := core.SaveNifti(out, cluster, true); err != nil {
			return nil, err
		}
		n := 0
		for _, v := range cluster.Data {
			if v > 0 {
				n++
			}
		}
		volume := float64(n) * core.VoxelVolumeML(cluster.Aff)
		return func() {
			p.saved, p.volume, p.output = out, volume, out
			p.st.ok = Tf("Reference cluster of %.0f mL saved.", "已保存参考簇（%.0f mL）。", volume)
		}, nil
	}, func(err error) { p.st.err = err.Error() })
}
