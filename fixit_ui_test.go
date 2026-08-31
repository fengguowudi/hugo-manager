package main

// FixIt 主题 UI 层适配基准：编辑器按 ThemeSchema 渲染主题专有字段，
// 并能无损往返（填入 → 收集）。输出 METRIC fixit_ui=N（共 4 分）。

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"hugo-manager/core"
)

func TestFixItUI(t *testing.T) {
	score := 0
	ck := func(name string, ok bool) {
		if !ok {
			t.Logf("MISS %s", name)
		} else {
			score++
		}
	}

	schema := core.ThemeSchema("FixIt")

	// u1: 额外表单包含 FixIt 专有字段（默认字段不重复出现）
	form, widgets := buildExtraFields(schema)
	if form == nil {
		t.Fatal("buildExtraFields 返回 nil form")
	}
	wantKeys := []string{"subtitle", "weight", "featured_image", "collections", "hidden_from_home_page"}
	okAll := len(widgets) > 0
	for _, k := range wantKeys {
		if _, ok := widgets[k]; !ok {
			okAll = false
		}
	}
	for _, k := range []string{"title", "date", "draft", "tags"} { // 默认字段不重复
		if _, ok := widgets[k]; ok {
			okAll = false
		}
	}
	ck("ui.schemaForm", okAll)

	// u2: 填入 → 收集往返（标量/布尔/数组/数字）
	if len(widgets) == 0 {
		ck("ui.roundtrip", false)
		ck("ui.fillBack", false)
	} else {
		widgets["subtitle"].(*widget.Entry).SetText("副标题X")
		widgets["weight"].(*widget.Entry).SetText("7")
		widgets["collections"].(*widget.Entry).SetText("合集A, 合集B")
		widgets["hidden_from_home_page"].(*widget.Check).SetChecked(true)
		fields, arrays := collectExtra(schema, widgets)
		ck("ui.roundtrip",
			fields["subtitle"] == "副标题X" && fields["weight"] == "7" &&
				fields["hidden_from_home_page"] == "true" &&
				strings.Join(arrays["collections"], ",") == "合集A,合集B")

		// u3: 解析结果回填（打开文章路径）
		fillExtra(schema, widgets, map[string]string{
			"subtitle": "回显副标题", "weight": "3", "hidden_from_home_page": "false",
		}, map[string][]string{"collections": {"回显"}})
		ck("ui.fillBack",
			widgets["subtitle"].(*widget.Entry).Text == "回显副标题" &&
				widgets["weight"].(*widget.Entry).Text == "3" &&
				!widgets["hidden_from_home_page"].(*widget.Check).Checked &&
				widgets["collections"].(*widget.Entry).Text == "回显")
	}

	// u4: 概览页显示站点主题名
	site := filepath.Join(".auto", "fixtures", "site")
	a := core.NewApp(filepath.Join(t.TempDir(), "config.json"))
	a.SetConfig(core.Config{SiteDir: site})
	ui := &nativeUI{a: a}
	var texts strings.Builder
	collectTexts(ui.overview(), &texts)
	ck("ui.overviewTheme", strings.Contains(texts.String(), "FixIt"))
	fmt.Printf("METRIC fixit_ui=%d\n", score)
}

// collectTexts 递归收集控件树中所有 Label 文本。
func collectTexts(o fyne.CanvasObject, sb *strings.Builder) {
	switch o := o.(type) {
	case *widget.Label:
		sb.WriteString(o.Text)
		sb.WriteByte('\n')
	case *fyne.Container:
		for _, c := range o.Objects {
			collectTexts(c, sb)
		}
	}
}
