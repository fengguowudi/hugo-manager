package main

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// nativeUI is the cross-platform shell. Business operations stay in App/content.go/hugorun.go.
type nativeUI struct {
	a                                  *App
	win                                fyne.Window
	pages                              *fyne.Container
	nav                                []*widget.Button
	status                             *widget.Label
	posts                              []Post
	visible                            []Post
	list                               *widget.List
	editor                             *fyne.Container
	current                            string
	dirty                              bool
	hint                               *widget.Label
	search                             *widget.Entry
	title, section, bin, siteDir, port *widget.Entry
	includeDrafts                      *widget.Check
	postTitle, postDate, postSlug      *widget.Entry
	postTags, postCategories           *widget.Entry
	postDescription, postBody          *widget.Entry
	postDraft                          *widget.Check
	logs                               *widget.Entry
	siteLabel                          *widget.Label
	views                              []fyne.CanvasObject
}

func runNativeApp(a *App) {
	fyneApp := app.New()
	fyneApp.Settings().SetTheme(newIOSTheme())
	ui := &nativeUI{a: a}
	ui.win = fyneApp.NewWindow("Hugo 管理器")
	ui.win.Resize(fyne.NewSize(1180, 760))
	ui.status = widget.NewLabelWithStyle("正在读取站点状态…", fyne.TextAlignLeading, fyne.TextStyle{})

	ui.views = []fyne.CanvasObject{ui.overview(), ui.content(), ui.build(), ui.settings()}
	ui.pages = container.NewMax(ui.views[0])
	ui.nav = []*widget.Button{
		widget.NewButtonWithIcon("概览", theme.HomeIcon(), func() { ui.showPage(0) }),
		widget.NewButtonWithIcon("内容", theme.ListIcon(), func() { ui.showPage(1) }),
		widget.NewButtonWithIcon("构建与预览", theme.ViewRefreshIcon(), func() { ui.showPage(2) }),
		widget.NewButtonWithIcon("设置", theme.SettingsIcon(), func() { ui.showPage(3) }),
	}
	for _, button := range ui.nav {
		button.Alignment = widget.ButtonAlignLeading
		button.Importance = widget.LowImportance
	}
	ui.nav[0].Importance = widget.HighImportance

	brand := container.NewVBox(
		widget.NewLabelWithStyle("Hugo 管理器", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("本地内容工作台"),
	)
	ui.siteLabel = widget.NewLabel("未配置站点")
	navBox := container.NewVBox(widget.NewSeparator(), ui.nav[0], ui.nav[1], ui.nav[2], ui.nav[3])
	sidebarInner := container.NewBorder(brand, container.NewVBox(widget.NewSeparator(), ui.siteLabel), nil, nil, navBox)
	widthKeeper := canvas.NewRectangle(color.Transparent)
	widthKeeper.SetMinSize(fyne.NewSize(196, 1))
	sidebar := container.NewStack(canvas.NewRectangle(iosGroupBg), container.NewPadded(container.NewBorder(widthKeeper, nil, nil, nil, sidebarInner)))

	statusBar := container.NewBorder(widget.NewSeparator(), nil, nil, nil, container.NewPadded(ui.status))
	ui.win.SetContent(container.NewBorder(nil, statusBar, sidebar, nil, ui.pages))
	ui.bindEditorShortcuts()
	ui.refresh()
	ui.win.ShowAndRun()
}

func (ui *nativeUI) showPage(active int) {
	if active == 0 {
		ui.views[0] = ui.overview()
	}
	ui.pages.Objects = []fyne.CanvasObject{ui.views[active]}
	ui.pages.Refresh()
	for i, button := range ui.nav {
		if i == active {
			button.Importance = widget.HighImportance
		} else {
			button.Importance = widget.LowImportance
		}
		button.Refresh()
	}
}

func (ui *nativeUI) setStatus(format string, args ...any) {
	ui.status.SetText(fmt.Sprintf(format, args...))
}

// iosHeader renders an iOS style navigation bar: bold title left, actions right, hairline below.
func iosHeader(title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var right fyne.CanvasObject
	if len(actions) > 0 {
		right = container.NewHBox(actions...)
	}
	return container.NewBorder(nil, widget.NewSeparator(), nil, right, container.NewPadded(label))
}

// iosCard renders a rounded white grouped card with a bold section title, iOS settings style.
func iosCard(title string, objects ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(iosBg)
	bg.CornerRadius = 10
	head := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	rows := append([]fyne.CanvasObject{head, widget.NewSeparator()}, objects...)
	return container.NewStack(bg, container.NewPadded(container.NewVBox(rows...)))
}

func (ui *nativeUI) refresh() {
	cfg := ui.a.cfgSnapshot()
	if cfg.SiteDir == "" {
		ui.setStatus("尚未配置站点")
		if ui.siteLabel != nil {
			ui.siteLabel.SetText("未配置站点")
		}
	} else {
		posts, _ := listPosts(cfg.SiteDir)
		ui.posts = posts
		ui.applyFilter()
		if ui.siteLabel != nil {
			name, _ := siteInfo(cfg.SiteDir)
			ui.siteLabel.SetText(fallback(name, filepath.Base(cfg.SiteDir)))
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
	if cfg.SiteDir == "" {
		welcome := container.NewVBox(
			widget.NewLabelWithStyle("欢迎使用 Hugo 管理器", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("请先在「设置」中填写 Hugo 路径和站点目录。", fyne.TextAlignCenter, fyne.TextStyle{}),
			container.NewCenter(widget.NewButtonWithIcon("打开设置", theme.SettingsIcon(), func() { ui.showPage(3) })),
		)
		return container.NewCenter(welcome)
	}

	info := widget.NewLabel("")
	info.Wrapping = fyne.TextWrapWord
	update := func() {
		st := ui.a.buildState()
		info.SetText(fmt.Sprintf("站点标题：%v\nbaseURL：%v\n站点目录：%v\nHugo：%v\n内容：%v 篇（草稿 %v 篇）",
			st["siteTitle"], st["baseURL"], st["siteDir"], st["hugoVersion"], st["countTotal"], st["countDrafts"]))
	}
	update()
	newPost := widget.NewButtonWithIcon("新建文章", theme.ContentAddIcon(), func() { ui.showPage(1) })
	newPost.Importance = widget.HighImportance
	preview := widget.NewButtonWithIcon("启动预览", theme.MediaPlayIcon(), func() { ui.startServer() })
	build := widget.NewButtonWithIcon("构建站点", theme.ViewRefreshIcon(), func() { ui.buildSite() })
	return container.NewBorder(iosHeader("站点概览", newPost, preview, build), nil, nil, nil, container.NewPadded(info))
}

func (ui *nativeUI) content() fyne.CanvasObject {
	ui.section = widget.NewEntry()
	ui.section.SetPlaceHolder("板块，例如 posts")
	ui.title = widget.NewEntry()
	ui.title.SetPlaceHolder("新文章标题")
	newButton := widget.NewButtonWithIcon("新建", theme.ContentAddIcon(), func() { ui.newPost() })
	newButton.Importance = widget.HighImportance
	newBar := container.NewBorder(nil, nil, nil, newButton, container.NewGridWithColumns(2, ui.title, ui.section))

	ui.search = widget.NewEntry()
	ui.search.SetPlaceHolder("搜索标题或路径…")
	ui.search.OnChanged = func(string) { ui.applyFilter() }
	ui.list = widget.NewList(func() int { return len(ui.visible) }, func() fyne.CanvasObject {
		return widget.NewLabel("文章")
	}, func(id widget.ListItemID, obj fyne.CanvasObject) {
		p := ui.visible[id]
		label := obj.(*widget.Label)
		state := "已发布"
		if p.Draft {
			state = "草稿"
		}
		label.SetText(fmt.Sprintf("%s  [%s]\n%s", fallback(p.Title, "(无标题)"), state, p.Path))
		label.Wrapping = fyne.TextWrapWord
	})
	ui.list.OnSelected = func(id widget.ListItemID) {
		if id < len(ui.visible) {
			ui.openPost(ui.visible[id].Path)
		}
	}
	ui.editor = ui.makeEditor()
	top := container.NewVBox(iosHeader("内容列表"), container.NewPadded(ui.search))
	listCol := container.NewBorder(top, container.NewPadded(newBar), nil, nil, ui.list)
	split := container.NewHSplit(listCol, ui.editor)
	split.Offset = .32
	return split
}

// applyFilter filters posts by the search box text and refreshes the list.
func (ui *nativeUI) applyFilter() {
	q := ""
	if ui.search != nil {
		q = strings.ToLower(strings.TrimSpace(ui.search.Text))
	}
	ui.visible = ui.posts
	if q != "" {
		ui.visible = nil
		for _, p := range ui.posts {
			if strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(p.Path), q) {
				ui.visible = append(ui.visible, p)
			}
		}
	}
	if ui.list != nil {
		ui.list.Refresh()
	}
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
	ui.postDraft = widget.NewCheck("草稿", func(bool) { ui.dirty = true })
	ui.hint = widget.NewLabelWithStyle("选择左侧文章开始编辑 · Ctrl+B 粗体 I 斜体 U 下划线 1~3 标题 K 链接 E 代码 Q 引用 L 列表 T 表格 S 保存", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	markDirty := func(string) { ui.dirty = true }
	for _, e := range []*widget.Entry{ui.postTitle, ui.postDate, ui.postSlug, ui.postTags, ui.postCategories, ui.postDescription} {
		e.SetPlaceHolder("未设置")
		e.OnChanged = markDirty
	}
	ui.postBody.OnChanged = func(s string) {
		ui.dirty = true
		ui.hint.SetText(fmt.Sprintf("正文 %d 字", len([]rune(s))))
	}
	form := widget.NewForm(
		widget.NewFormItem("标题", ui.postTitle), widget.NewFormItem("日期", ui.postDate),
		widget.NewFormItem("Slug", ui.postSlug), widget.NewFormItem("标签", ui.postTags),
		widget.NewFormItem("分类", ui.postCategories), widget.NewFormItem("摘要", ui.postDescription),
		widget.NewFormItem("状态", ui.postDraft),
	)
	save := widget.NewButton("保存", func() { ui.savePost() })
	save.Importance = widget.HighImportance
	deleteBtn := widget.NewButton("删除", func() { ui.deletePost() })
	deleteBtn.Importance = widget.DangerImportance
	split := container.NewVSplit(container.NewVScroll(form), ui.postBody)
	split.Offset = 0.42
	statusLine := container.NewBorder(widget.NewSeparator(), nil, nil, nil, container.NewPadded(ui.hint))
	return container.NewBorder(iosHeader("编辑器", deleteBtn, save), statusLine, nil, nil, split)
}
func (ui *nativeUI) openPost(path string) {
	if ui.dirty && ui.current != "" && ui.current != path {
		dialog.ShowConfirm("未保存的更改", "当前文章有未保存的修改，切换后将丢失。仍要切换吗？", func(ok bool) {
			if ok {
				ui.dirty = false
				ui.openPost(path)
			}
		}, ui.win)
		return
	}
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
	ui.dirty = false
	ui.hint.SetText(fmt.Sprintf("正文 %d 字", len([]rune(ui.postBody.Text))))
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
	ui.dirty = false
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

	browseDir := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				ui.siteDir.SetText(uri.Path())
			}
		}, ui.win)
	})
	browseBin := widget.NewButtonWithIcon("", theme.FileApplicationIcon(), func() {
		dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
			if err == nil && file != nil {
				ui.bin.SetText(file.URI().Path())
				file.Close()
			}
		}, ui.win)
	})
	binRow := container.NewBorder(nil, nil, nil, browseBin, ui.bin)
	dirRow := container.NewBorder(nil, nil, nil, browseDir, ui.siteDir)

	save := widget.NewButton("保存设置", func() { ui.saveSettings() })
	save.Importance = widget.HighImportance
	detect := widget.NewButtonWithIcon("检测 Hugo", theme.SearchIcon(), func() { ui.detectHugo() })
	pageBg := canvas.NewRectangle(iosGroupBg)
	cards := container.NewVBox(
		iosCard("站点", widget.NewForm(widget.NewFormItem("站点目录", dirRow))),
		iosCard("Hugo", widget.NewForm(widget.NewFormItem("Hugo 路径", binRow))),
		iosCard("构建与预览", widget.NewForm(
			widget.NewFormItem("预览端口", ui.port),
			widget.NewFormItem("构建选项", ui.includeDrafts),
		)),
	)
	body := container.NewVScroll(container.NewPadded(cards))
	return container.NewBorder(iosHeader("设置", detect, save), nil, nil, nil, container.NewStack(pageBg, body))
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
	start := widget.NewButtonWithIcon("启动预览服务器", theme.MediaPlayIcon(), func() { ui.startServer() })
	start.Importance = widget.HighImportance
	stop := widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), func() {
		if err := ui.a.stopServer(); err != nil {
			ui.showError(err)
		}
		ui.refresh()
	})
	run := widget.NewButtonWithIcon("开始构建", theme.ViewRefreshIcon(), func() { ui.buildSite() })
	reload := widget.NewButtonWithIcon("刷新日志", theme.ContentClearIcon(), func() { ui.loadLogs() })
	open := widget.NewButtonWithIcon("打开浏览器", theme.ComputerIcon(), func() {
		openBrowser(fmt.Sprintf("http://localhost:%d", ui.a.cfgSnapshot().ServerPort))
	})
	return container.NewBorder(iosHeader("构建与预览", start, stop, run, reload, open), nil, nil, nil, container.NewPadded(ui.logs))
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
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
	text := strings.Join(ui.a.hub.historySnapshot(), "\n")
	ui.logs.SetText(text)
	ui.logs.CursorRow, ui.logs.CursorColumn = strings.Count(text, "\n"), 0 // 光标置末行，滚动到底
	ui.logs.Refresh()
}
func (ui *nativeUI) showError(err error) { dialog.ShowError(err, ui.win) }

// bindEditorShortcuts registers Typecho-style Markdown shortcuts on the window canvas.
func (ui *nativeUI) bindEditorShortcuts() {
	c := ui.win.Canvas()
	bind := func(key fyne.KeyName, fn func()) {
		c.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) {
			if c.Focused() != ui.postBody {
				return // body-only shortcuts,避免劫持其他输入框
			}
			fn()
		})
	}
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { ui.savePost() })
	bind(fyne.KeyB, func() { mdWrap(ui.postBody, "**", "**", "粗体文本") })
	bind(fyne.KeyI, func() { mdWrap(ui.postBody, "*", "*", "斜体文本") })
	bind(fyne.KeyU, func() { mdWrap(ui.postBody, "<u>", "</u>", "下划线文本") })
	bind(fyne.KeyK, func() { mdWrap(ui.postBody, "[", "](https://)", "链接文本") })
	bind(fyne.KeyE, func() { mdWrap(ui.postBody, "`", "`", "代码") })
	bind(fyne.KeyQ, func() { mdLinePrefix(ui.postBody, "> ") })
	bind(fyne.KeyL, func() { mdLinePrefix(ui.postBody, "- ") })
	bind(fyne.Key1, func() { mdHeading(ui.postBody, 1) })
	bind(fyne.Key2, func() { mdHeading(ui.postBody, 2) })
	bind(fyne.Key3, func() { mdHeading(ui.postBody, 3) })
	bind(fyne.KeyT, func() { mdInsertBlock(ui.postBody, "\n| 列1 | 列2 | 列3 |\n| --- | --- | --- |\n|  |  |  |\n") })
}

// cursorOffset returns the rune offset of the entry cursor.
func cursorOffset(e *widget.Entry) int {
	off := 0
	lines := strings.Split(e.Text, "\n")
	for i := 0; i < e.CursorRow && i < len(lines); i++ {
		off += len([]rune(lines[i])) + 1
	}
	return off + e.CursorColumn
}

// setEntryCursor moves the cursor to a rune offset.
func setEntryCursor(e *widget.Entry, off int) {
	r := []rune(e.Text)
	if off > len(r) {
		off = len(r)
	}
	row, col := 0, 0
	for i := 0; i < off; i++ {
		if r[i] == '\n' {
			row, col = row+1, 0
		} else {
			col++
		}
	}
	e.CursorRow, e.CursorColumn = row, col
	e.Refresh()
}

// mdWrap wraps the selection (or placeholder at cursor) with before/after markers.
func mdWrap(e *widget.Entry, before, after, placeholder string) {
	runes := []rune(e.Text)
	cur := cursorOffset(e)
	sel := []rune(e.SelectedText())
	start, end := cur, cur
	if len(sel) > 0 {
		switch {
		case cur+len(sel) <= len(runes) && string(runes[cur:cur+len(sel)]) == string(sel):
			end = cur + len(sel)
		case cur-len(sel) >= 0 && string(runes[cur-len(sel):cur]) == string(sel):
			start = cur - len(sel)
		default:
			sel = nil // 选区定位失败时按光标处插入处理
		}
	}
	if len(sel) == 0 {
		sel = []rune(placeholder)
	}
	var b strings.Builder
	b.WriteString(string(runes[:start]))
	b.WriteString(before)
	b.WriteString(string(sel))
	b.WriteString(after)
	b.WriteString(string(runes[end:]))
	e.SetText(b.String())
	setEntryCursor(e, start+len([]rune(before))+len(sel)+len([]rune(after)))
}

// mdLinePrefix inserts a block prefix (quote/list) at the start of the current line.
func mdLinePrefix(e *widget.Entry, prefix string) {
	r := []rune(e.Text)
	ls := cursorOffset(e)
	for ls > 0 && r[ls-1] != '\n' {
		ls--
	}
	e.SetText(string(r[:ls]) + prefix + string(r[ls:]))
	setEntryCursor(e, ls+len([]rune(prefix)))
}

// mdHeading sets the current line to the given heading level, replacing any existing # prefix.
func mdHeading(e *widget.Entry, level int) {
	r := []rune(e.Text)
	ls := cursorOffset(e)
	for ls > 0 && r[ls-1] != '\n' {
		ls--
	}
	le := ls
	for le < len(r) && r[le] != '\n' {
		le++
	}
	line := strings.TrimPrefix(strings.TrimLeft(string(r[ls:le]), "#"), " ")
	newLine := strings.Repeat("#", level) + " " + line
	e.SetText(string(r[:ls]) + newLine + string(r[le:]))
	setEntryCursor(e, ls+len([]rune(newLine)))
}

// mdInsertBlock inserts a snippet (e.g. table skeleton) at the cursor.
func mdInsertBlock(e *widget.Entry, snippet string) {
	r := []rune(e.Text)
	cur := cursorOffset(e)
	e.SetText(string(r[:cur]) + snippet + string(r[cur:]))
	setEntryCursor(e, cur+len([]rune(snippet)))
}

// iOS 10 flat light palette
var (
	iosBlue    = color.NRGBA{R: 0, G: 122, B: 255, A: 255}
	iosRed     = color.NRGBA{R: 255, G: 59, B: 48, A: 255}
	iosGreen   = color.NRGBA{R: 52, G: 199, B: 89, A: 255}
	iosOrange  = color.NRGBA{R: 255, G: 149, B: 0, A: 255}
	iosBg      = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	iosGroupBg = color.NRGBA{R: 242, G: 242, B: 247, A: 255} // sidebar / grouped background
	iosSep     = color.NRGBA{R: 198, G: 198, B: 200, A: 255}
	iosTextDim = color.NRGBA{R: 142, G: 142, B: 147, A: 255}
)

// iosTheme is a flat, always-light iOS 10 style theme wrapping the default theme.
type iosTheme struct{ fyne.Theme }

func newIOSTheme() fyne.Theme { return iosTheme{theme.DefaultTheme()} }

func (t iosTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return iosBlue
	case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
		return iosBg
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameMenuBackground,
		theme.ColorNameHeaderBackground, theme.ColorNameDisabledButton:
		return iosGroupBg
	case theme.ColorNameSeparator, theme.ColorNameInputBorder, theme.ColorNameScrollBarBackground:
		return iosSep
	case theme.ColorNameHover:
		return color.NRGBA{A: 12}
	case theme.ColorNamePressed:
		return color.NRGBA{A: 24}
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return iosTextDim
	case theme.ColorNameError:
		return iosRed
	case theme.ColorNameSuccess:
		return iosGreen
	case theme.ColorNameWarning:
		return iosOrange
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0, G: 122, B: 255, A: 60}
	case theme.ColorNameScrollBar:
		return color.NRGBA{A: 90}
	case theme.ColorNameShadow:
		return color.NRGBA{A: 40}
	}
	return t.Theme.Color(name, theme.VariantLight)
}

func (t iosTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInputRadius {
		return 6 // flat, subtle corners
	}
	return t.Theme.Size(name)
}

func (t iosTheme) Font(style fyne.TextStyle) fyne.Resource { return t.Theme.Font(style) }
func (t iosTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.Theme.Icon(name)
}
