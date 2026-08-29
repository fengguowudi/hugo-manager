package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// nativeUI is the cross-platform shell. Business operations stay in App/content.go/hugorun.go.
type nativeUI struct {
	a       *App
	win     fyne.Window
	tabs    *container.AppTabs
	status  *widget.Label
	posts   []Post
	list    *widget.List
	editor  *fyne.Container
	current string

	title, section, bin, siteDir, port *widget.Entry
	includeDrafts                      *widget.Check
	postTitle, postDate, postSlug      *widget.Entry
	postTags, postCategories           *widget.Entry
	postDescription, postBody          *widget.Entry
	postDraft                          *widget.Check
	logs                               *widget.Entry
}

func runNativeApp(a *App) {
	ui := &nativeUI{a: a}
	ui.win = app.New().NewWindow("Hugo 管理器")
	ui.win.Resize(fyne.NewSize(1100, 720))
	ui.status = widget.NewLabel("正在读取站点状态…")
	ui.tabs = container.NewAppTabs(
		container.NewTabItem("概览", ui.overview()),
		container.NewTabItem("内容", ui.content()),
		container.NewTabItem("构建", ui.build()),
		container.NewTabItem("设置", ui.settings()),
	)
	ui.win.SetContent(container.NewBorder(nil, ui.status, nil, nil, ui.tabs))
	ui.refresh()
	ui.win.ShowAndRun()
}

func (ui *nativeUI) setStatus(format string, args ...any) {
	ui.status.SetText(fmt.Sprintf(format, args...))
}

func (ui *nativeUI) refresh() {
	cfg := ui.a.cfgSnapshot()
	if cfg.SiteDir == "" {
		ui.setStatus("尚未配置站点")
	} else {
		posts, _ := listPosts(cfg.SiteDir)
		ui.posts = posts
		if ui.list != nil {
			ui.list.Refresh()
		}
		ui.setStatus("站点：%s · %d 篇内容", cfg.SiteDir, len(posts))
	}
	if ui.bin != nil {
		ui.bin.SetText(cfg.HugoBin)
	}
	if ui.siteDir != nil {
		ui.siteDir.SetText(cfg.SiteDir)
	}
	if ui.port != nil {
		ui.port.SetText(strconv.Itoa(cfg.ServerPort))
	}
	if ui.includeDrafts != nil {
		ui.includeDrafts.SetChecked(cfg.IncludeDrafts)
	}
}

func (ui *nativeUI) overview() fyne.CanvasObject {
	cfg := ui.a.cfgSnapshot()
	info := widget.NewLabel("")
	info.Wrapping = fyne.TextWrapWord
	update := func() {
		st := ui.a.buildState()
		info.SetText(fmt.Sprintf("站点标题：%v\nbaseURL：%v\n站点目录：%v\nHugo：%v\n内容：%v 篇（草稿 %v 篇）",
			st["siteTitle"], st["baseURL"], st["siteDir"], st["hugoVersion"], st["countTotal"], st["countDrafts"]))
	}
	update()
	buttons := container.NewHBox(
		widget.NewButton("新建文章", func() { ui.tabs.SelectIndex(1) }),
		widget.NewButton("启动预览服务器", func() { ui.startServer() }),
		widget.NewButton("构建站点", func() { ui.buildSite() }),
	)
	if cfg.SiteDir == "" {
		return container.NewBorder(widget.NewLabel("欢迎使用 Hugo 管理器"), nil, nil, nil,
			container.NewVBox(widget.NewLabel("请先在“设置”中填写 Hugo 路径和站点目录。"), widget.NewButton("打开设置", func() { ui.tabs.SelectIndex(3) })))
	}
	return container.NewBorder(widget.NewLabel("站点概览"), buttons, nil, nil, container.NewVBox(info))
}

func (ui *nativeUI) content() fyne.CanvasObject {
	ui.section = widget.NewEntry()
	ui.section.SetPlaceHolder("板块，例如 posts")
	ui.title = widget.NewEntry()
	ui.title.SetPlaceHolder("新文章标题")
	newButton := widget.NewButton("新建", func() { ui.newPost() })
	newBar := container.NewBorder(nil, nil, nil, newButton, container.NewGridWithColumns(2, ui.title, ui.section))

	ui.list = widget.NewList(func() int { return len(ui.posts) }, func() fyne.CanvasObject {
		return widget.NewLabel("文章")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		p := ui.posts[id]
		label := obj.(*widget.Label)
		state := "已发布"
		if p.Draft {
			state = "草稿"
		}
		label.SetText(fmt.Sprintf("%s  [%s]\n%s", fallback(p.Title, "(无标题)"), state, p.Path))
		label.Wrapping = fyne.TextWrapWord
	})
	ui.list.OnSelected = func(id widget.ListItemID) { ui.openPost(ui.posts[id].Path) }
	ui.editor = ui.makeEditor()
	split := container.NewHSplit(container.NewBorder(newBar, nil, nil, nil, ui.list), ui.editor)
	split.Offset = .32
	return split
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func (ui *nativeUI) makeEditor() *fyne.Container {
	ui.postTitle = widget.NewEntry()
	ui.postDate = widget.NewEntry()
	ui.postSlug = widget.NewEntry()
	ui.postTags = widget.NewEntry()
	ui.postCategories = widget.NewEntry()
	ui.postDescription = widget.NewMultiLineEntry()
	ui.postBody = widget.NewMultiLineEntry()
	ui.postBody.Wrapping = fyne.TextWrapOff
	ui.postDraft = widget.NewCheck("草稿", nil)
	for _, e := range []*widget.Entry{ui.postTitle, ui.postDate, ui.postSlug, ui.postTags, ui.postCategories} {
		e.SetPlaceHolder("未设置")
	}
	form := widget.NewForm(
		widget.NewFormItem("标题", ui.postTitle), widget.NewFormItem("日期", ui.postDate),
		widget.NewFormItem("Slug", ui.postSlug), widget.NewFormItem("标签", ui.postTags),
		widget.NewFormItem("分类", ui.postCategories), widget.NewFormItem("摘要", ui.postDescription),
		widget.NewFormItem("状态", ui.postDraft),
	)
	buttons := container.NewHBox(widget.NewButton("保存", func() { ui.savePost() }), widget.NewButton("删除", func() { ui.deletePost() }))
	return container.NewBorder(container.NewVBox(widget.NewLabel("选择一篇文章开始编辑"), form), buttons, nil, nil, ui.postBody)
}

func (ui *nativeUI) openPost(path string) {
	p, err := ui.a.safeSitePath(path)
	if err != nil {
		ui.showError(err)
		return
	}
	format, fields, arrays, body, _, err := parsePost(p)
	if err != nil {
		ui.showError(err)
		return
	}
	if format == "" {
		ui.showError(fmt.Errorf("该文件没有可识别的 front matter"))
		return
	}
	ui.current = path
	ui.postTitle.SetText(fields["title"])
	ui.postDate.SetText(fields["date"])
	ui.postSlug.SetText(fields["slug"])
	ui.postTags.SetText(strings.Join(arrays["tags"], ", "))
	ui.postCategories.SetText(strings.Join(arrays["categories"], ", "))
	ui.postDescription.SetText(fields["description"])
	ui.postBody.SetText(body)
	ui.postDraft.SetChecked(fields["draft"] == "true")
	ui.setStatus("正在编辑：%s", path)
}

func (ui *nativeUI) savePost() {
	if ui.current == "" {
		ui.setStatus("请先从内容列表选择文章")
		return
	}
	p, err := ui.a.safeSitePath(ui.current)
	if err != nil {
		ui.showError(err)
		return
	}
	fields := map[string]string{"title": ui.postTitle.Text, "date": ui.postDate.Text, "slug": ui.postSlug.Text, "description": ui.postDescription.Text, "draft": strconv.FormatBool(ui.postDraft.Checked)}
	arrays := map[string][]string{"tags": splitNative(ui.postTags.Text), "categories": splitNative(ui.postCategories.Text)}
	if err := savePost(p, fields, arrays, ui.postBody.Text); err != nil {
		ui.showError(err)
		return
	}
	ui.setStatus("已保存：%s", ui.current)
	ui.refresh()
}

func splitNative(s string) []string {
	var out []string
	for _, v := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '，' }) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (ui *nativeUI) newPost() {
	if strings.TrimSpace(ui.title.Text) == "" {
		ui.setStatus("请填写新文章标题")
		return
	}
	section := strings.TrimSpace(ui.section.Text)
	if section == "" {
		section = "posts"
	}
	rel := filepath.Join(section, time.Now().Format("20060102-150405")+".md")
	if err := ui.a.newContent(filepath.ToSlash(rel), "", ui.title.Text); err != nil {
		ui.showError(err)
		return
	}
	ui.refresh()
	ui.openPost("content/" + filepath.ToSlash(rel))
	ui.setStatus("已创建：%s", rel)
}

func (ui *nativeUI) deletePost() {
	if ui.current == "" {
		return
	}
	dialog.ShowConfirm("删除文章", "此操作不可恢复，确定继续？", func(ok bool) {
		if !ok {
			return
		}
		p, err := ui.a.safeSitePath(ui.current)
		if err == nil {
			err = os.Remove(p)
		}
		if err != nil {
			ui.showError(err)
			return
		}
		ui.current = ""
		ui.refresh()
		ui.setStatus("文章已删除")
	}, ui.win)
}

func (ui *nativeUI) settings() fyne.CanvasObject {
	cfg := ui.a.cfgSnapshot()
	ui.bin = widget.NewEntry()
	ui.bin.SetText(cfg.HugoBin)
	ui.bin.SetPlaceHolder("留空则从 PATH 查找")
	ui.siteDir = widget.NewEntry()
	ui.siteDir.SetText(cfg.SiteDir)
	ui.siteDir.SetPlaceHolder("博客根目录")
	ui.port = widget.NewEntry()
	ui.port.SetText(strconv.Itoa(cfg.ServerPort))
	ui.includeDrafts = widget.NewCheck("构建包含草稿", nil)
	ui.includeDrafts.SetChecked(cfg.IncludeDrafts)
	save := widget.NewButton("保存设置", func() { ui.saveSettings() })
	detect := widget.NewButton("检测 Hugo", func() { ui.detectHugo() })
	form := widget.NewForm(widget.NewFormItem("Hugo 路径", ui.bin), widget.NewFormItem("站点目录", ui.siteDir), widget.NewFormItem("预览端口", ui.port), widget.NewFormItem("构建选项", ui.includeDrafts))
	return container.NewBorder(widget.NewLabel("设置"), container.NewHBox(save, detect), nil, nil, form)
}

func (ui *nativeUI) saveSettings() {
	port, _ := strconv.Atoi(ui.port.Text)
	if port <= 0 {
		port = 1313
	}
	cfg := Config{HugoBin: strings.TrimSpace(ui.bin.Text), SiteDir: strings.TrimSpace(ui.siteDir.Text), ServerPort: port, IncludeDrafts: ui.includeDrafts.Checked}
	if cfg.SiteDir != "" {
		if err := validateSiteDir(cfg.SiteDir); err != nil {
			ui.showError(err)
			return
		}
	}
	ui.a.mu.Lock()
	ui.a.cfg = cfg
	ui.a.mu.Unlock()
	ui.a.saveCfg()
	ui.a.probe()
	ui.refresh()
	ui.setStatus("设置已保存")
}

func validateSiteDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("站点目录不存在：%s", dir)
	}
	return nil
}

func (ui *nativeUI) detectHugo() {
	bin, err := exec.LookPath("hugo")
	if err != nil {
		ui.showError(err)
		return
	}
	ui.bin.SetText(bin)
	ui.setStatus("检测到：%s", probeVersion(bin))
}

func (ui *nativeUI) build() fyne.CanvasObject {
	ui.logs = widget.NewMultiLineEntry()
	ui.logs.Wrapping = fyne.TextWrapOff
	ui.logs.Disable()
	start := widget.NewButton("启动预览服务器", func() { ui.startServer() })
	stop := widget.NewButton("停止", func() {
		if err := ui.a.stopServer(); err != nil {
			ui.showError(err)
		}
		ui.refresh()
	})
	run := widget.NewButton("开始构建", func() { ui.buildSite() })
	reload := widget.NewButton("刷新日志", func() { ui.loadLogs() })
	return container.NewBorder(container.NewHBox(start, stop, run, reload), nil, nil, nil, ui.logs)
}

func (ui *nativeUI) startServer() {
	if err := ui.a.startServer(); err != nil {
		ui.showError(err)
		return
	}
	ui.setStatus("预览服务器已启动，端口 %d", ui.a.cfgSnapshot().ServerPort)
	ui.loadLogs()
}
func (ui *nativeUI) buildSite() {
	go func() {
		err := ui.a.runBuild()
		fyne.Do(func() {
			if err != nil {
				ui.showError(err)
			} else {
				ui.setStatus("构建完成")
			}
			ui.loadLogs()
		})
	}()
}
func (ui *nativeUI) loadLogs() {
	ui.logs.SetText(strings.Join(ui.a.hub.historySnapshot(), "\n"))
}
func (ui *nativeUI) showError(err error) { dialog.ShowError(err, ui.win) }
