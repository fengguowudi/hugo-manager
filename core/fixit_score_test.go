package core

// FixIt 主题适配基准：对 .auto/fixtures/site（真实 FixIt 主题站点）执行
// 行为级检查，输出 METRIC fixit_core=N。每个检查 1 分，共 fixitCoreTotal 分。
// 本测试永不失败（只计分）；正确性由 checks.sh 与检查项本身保证。

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixitCoreTotal = 52

type scoreKeeper struct {
	t     *testing.T
	score int
}

func (s *scoreKeeper) ck(name string, ok bool) {
	if ok {
		s.score++
	} else {
		s.t.Logf("MISS %s", name)
	}
}

func benchPaths(t *testing.T) (root, site, hugoBin, themesDir string) {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return root,
		filepath.Join(root, ".auto", "fixtures", "site"),
		filepath.Join(root, ".auto", "bin", "hugo.exe"),
		filepath.Join(root, ".auto", "themes")
}

// hugoBuild 用真实 FixIt 主题构建站点副本，返回是否成功。
func hugoBuild(t *testing.T, hugoBin, siteDir, themesDir, dst string) bool {
	t.Helper()
	dartSass := filepath.Join(filepath.Dir(hugoBin), "dart-sass")
	cmd := exec.Command(hugoBin, "-s", siteDir, "--themesDir", themesDir, "--destination", dst)
	sep := string(os.PathListSeparator)
	cmd.Env = append(os.Environ(), "PATH="+dartSass+sep+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("hugo build failed: %v\n%s", err, lastLines(string(out), 5))
		return false
	}
	return true
}

func lastLines(s string, n int) string {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(ls) > n {
		ls = ls[len(ls)-n:]
	}
	return strings.Join(ls, "\n")
}

// copySite 把 fixture 站点复制到临时目录，返回副本路径。
func copySite(t *testing.T, site string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "site")
	if err := os.CopyFS(dst, os.DirFS(site)); err != nil {
		t.Fatal(err)
	}
	return dst
}

// copyPost 把单篇文章复制到临时文件，返回路径。
func copyPost(t *testing.T, site, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(site, rel))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), filepath.Base(rel))
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFixItScore(t *testing.T) {
	root, site, hugoBin, themesDir := benchPaths(t)
	s := &scoreKeeper{t: t}

	// ---- 站点级 ----
	title, baseURL := SiteInfo(site)
	s.ck("site.title", title == "Fixture Blog")
	s.ck("site.baseURL", baseURL == "https://fixture.example.com/")
	s.ck("site.detectTheme", DetectTheme(site) == "FixIt")
	a := NewApp(filepath.Join(t.TempDir(), "config.json"))
	a.SetConfig(Config{SiteDir: site})
	s.ck("state.theme", a.BuildState()["theme"] == "FixIt")
	s.ck("state.encryptedCount", a.BuildState()["countEncrypted"] == 1) // locked-weighted.md 有 password
	s.ck("state.expiredCount", a.BuildState()["countExpired"] == 1)     // expired.md 的 expiryDate 已过
	s.ck("state.scheduledCount", a.BuildState()["countScheduled"] == 1) // scheduled.md 的 publishDate 在未来

	// ---- Hugo 版本与主题兼容性（FixIt 要求 hugo >= theme.toml min_version）----
	s.ck("parse.hugoVersion", ParseHugoVersion("hugo v0.165.0-76a5e18+extended windows/amd64 BuildDate=2026-08-12") == "0.165.0")

	old := NewApp(filepath.Join(t.TempDir(), "config.json"))
	old.SetConfig(Config{SiteDir: site, HugoBin: filepath.Join(root, ".auto", "fixtures", "fakehugo", "hugo.bat")})
	old.Probe() // 假 hugo 报告 v0.127.0
	s.ck("state.hugoCompatOld", strings.Contains(fmt.Sprint(old.BuildState()["hugoCompat"]), "0.161.0"))

	a.SetConfig(Config{SiteDir: site, HugoBin: hugoBin})
	a.Probe() // 真 hugo v0.165.0
	_, hasWarn := a.BuildState()["hugoCompat"]
	s.ck("state.hugoCompatNew", !hasWarn)

	// ---- FixIt 友链（data/friends.yml，供 layout: friends 页面使用）----
	friends, ferr := ListFriends(site)
	s.ck("friends.parse", ferr == nil && len(friends) == 2 && friends[0].Nickname == "示例友链" && friends[1].URL == "https://fixit.lruihao.cn")

	fs := copySite(t, site)
	baddies := AddFriend(fs, Friend{Nickname: "无链接"}) == nil ||
		AddFriend(fs, Friend{Nickname: "坏协议", URL: "ftp://x"}) == nil
	okAdd := AddFriend(fs, Friend{Nickname: "新友链", URL: "https://new.example.com", Description: "d"}) == nil
	friends2, _ := ListFriends(fs)
	s.ck("friends.addValidate", !baddies && okAdd && len(friends2) == 3 && friends2[2].Nickname == "新友链")

	s.ck("state.friendsCount", a.BuildState()["countFriends"] == 2)

	// ---- 多语言站点（languages.*.contentDir，FixIt 是 i18n 重度主题）----
	allPosts, _ := ListPosts(site)
	foundEN := false
	for _, p := range allPosts {
		if p.Title == "English Post" && p.Section == "posts" { // content/en 下的文章应归入 posts 板块而非 "en"
			foundEN = true
		}
	}
	s.ck("list.multiLang", foundEN)
	seenPaths := map[string]bool{}
	duplicatePath := false
	for _, p := range allPosts {
		if seenPaths[p.Path] {
			duplicatePath = true
		}
		seenPaths[p.Path] = true
	}
	s.ck("list.noDuplicateRoots", !duplicatePath)

	// YAML 配置同样支持独立语言 contentDir
	yamlSite := t.TempDir()
	for _, d := range []string{"content/zh-cn/posts", "content/en/posts", "archetypes"} {
		if err := os.MkdirAll(filepath.Join(yamlSite, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	yamlConfig := "title: YAML multilingual\ndefaultContentLanguage: zh-cn\nlanguages:\n  zh-cn:\n    contentDir: content/zh-cn\n  en:\n    contentDir: content/en\n"
	if err := os.WriteFile(filepath.Join(yamlSite, "hugo.yaml"), []byte(yamlConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, title := range map[string]string{"content/zh-cn/posts/zh.md": "中文 YAML", "content/en/posts/en.md": "English YAML"} {
		body := "---\ntitle: " + title + "\ndate: 2024-01-01T00:00:00+08:00\n---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(yamlSite, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archetype := "---\ntitle: \"{{ .File.ContentBaseName }}\"\ndraft: true\n---\n"
	if err := os.WriteFile(filepath.Join(yamlSite, "archetypes", "default.md"), []byte(archetype), 0o644); err != nil {
		t.Fatal(err)
	}
	yamlPosts, _ := ListPosts(yamlSite)
	yamlFound := map[string]bool{}
	for _, p := range yamlPosts {
		if p.Section == "posts" {
			yamlFound[p.Title] = true
		}
	}
	s.ck("list.multiLangYAML", DefaultContentDir(yamlSite) == "content/zh-cn" && yamlFound["中文 YAML"] && yamlFound["English YAML"])

	yamlApp := NewApp(filepath.Join(t.TempDir(), "config.json"))
	yamlApp.SetConfig(Config{SiteDir: yamlSite, HugoBin: hugoBin})
	yamlNewErr := yamlApp.NewContent("posts/yaml-new.md", "", "YAML New")
	_, yamlDefaultErr := os.Stat(filepath.Join(yamlSite, "content", "zh-cn", "posts", "yaml-new.md"))
	_, yamlWrongErr := os.Stat(filepath.Join(yamlSite, "content", "en", "posts", "yaml-new.md"))
	s.ck("newcontent.yamlDefaultDir", yamlNewErr == nil && yamlDefaultErr == nil && os.IsNotExist(yamlWrongErr))

	// ---- 解析 ----
	helloRel := filepath.Join("content", "posts", "hello-fixit.md")
	_, fields, _, _, _, err := ParsePost(filepath.Join(site, helloRel))
	if err != nil {
		t.Fatal(err)
	}
	s.ck("parse.subtitle", fields["subtitle"] == "第一篇文章")
	s.ck("parse.weight", fields["weight"] == "0")

	lockedRel := filepath.Join("content", "posts", "locked-weighted.md")
	_, lf, la, _, _, err := ParsePost(filepath.Join(site, lockedRel))
	if err != nil {
		t.Fatal(err)
	}
	s.ck("parse.inlineTags", strings.Join(la["tags"], ",") == "secret,pinned")
	s.ck("parse.hiddenFromHomePage", lf["hidden_from_home_page"] == "true")

	// p5: YAML/TOML 行尾注释不能污染字段语义，尤其不能把 draft:true 变成 false
	commentSite := t.TempDir()
	commentDir := filepath.Join(commentSite, "content", "posts")
	if err := os.MkdirAll(commentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yamlCommentPath := filepath.Join(commentDir, "commented-draft.md")
	yamlComment := "---\ntitle: \"Secret #1\" # editorial note\ndate: 2024-01-01T00:00:00+08:00\ndraft: true # must stay private\ntags: [\"internal\", \"tag#1\"] # classification\ndescription: https://example.com/#anchor\n---\n\nSECRET\n"
	if err := os.WriteFile(yamlCommentPath, []byte(yamlComment), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ycf, yca, _, _, err := ParsePost(yamlCommentPath)
	if err != nil {
		t.Fatal(err)
	}
	s.ck("parse.inlineComments", ycf["title"] == "Secret #1" && ycf["draft"] == "true" &&
		ycf["description"] == "https://example.com/#anchor" && strings.Join(yca["tags"], ",") == "internal,tag#1")

	draftValue := "false"
	if ycf["draft"] == "true" {
		draftValue = "true"
	}
	if err := SavePost(yamlCommentPath, map[string]string{"title": ycf["title"], "date": ycf["date"], "draft": draftValue, "description": ycf["description"]}, map[string][]string{"tags": yca["tags"]}, "UPDATED\n"); err != nil {
		t.Fatal(err)
	}
	yamlSaved := readFile(t, yamlCommentPath)
	_, ycf2, yca2, _, _, err := ParsePost(yamlCommentPath)
	if err != nil {
		t.Fatal(err)
	}
	commentPosts, _ := ListPosts(commentSite)
	stillDraft := len(commentPosts) == 1 && commentPosts[0].Draft
	s.ck("save.inlineComments", stillDraft && ycf2["title"] == "Secret #1" && ycf2["draft"] == "true" &&
		strings.Join(yca2["tags"], ",") == "internal,tag#1" && strings.Contains(yamlSaved, "# must stay private") &&
		strings.Contains(yamlSaved, "# editorial note") && strings.Contains(yamlSaved, "# classification"))

	tomlCommentPath := filepath.Join(commentDir, "commented-toml.md")
	tomlComment := "+++\ntitle = \"Secret #2\" # editorial note\ndate = 2024-01-02T00:00:00+08:00\ndraft = true # must stay private\ntags = [\"internal\", \"tag#2\"] # classification\n+++\n\nSECRET TOML\n"
	if err := os.WriteFile(tomlCommentPath, []byte(tomlComment), 0o644); err != nil {
		t.Fatal(err)
	}
	_, tcf, tca, _, _, err := ParsePost(tomlCommentPath)
	if err != nil {
		t.Fatal(err)
	}
	s.ck("toml.inlineComments", tcf["title"] == "Secret #2" && tcf["draft"] == "true" && strings.Join(tca["tags"], ",") == "internal,tag#2")

	// p5: YAML 块式列表（FixIt 原型默认写法）应解析为数组
	_, _, ha, _, _, err := ParsePost(filepath.Join(site, helloRel))
	if err != nil {
		t.Fatal(err)
	}
	s.ck("parse.blockList", strings.Join(ha["tags"], ",") == "hugo,fixit" &&
		strings.Join(ha["keywords"], ",") == "Hugo,FixIt" && strings.Join(ha["collections"], ",") == "随笔")

	// p6: 块式列表解析→写回往返不丢数据（转为行内是文档化行为）
	bp := copyPost(t, site, helloRel)
	if err := SavePost(bp, nil, map[string][]string{"tags": ha["tags"], "keywords": ha["keywords"]}, "正文不变。\n"); err != nil {
		t.Fatal(err)
	}
	bgot := readFile(t, bp)
	_, _, ra, _, _, err := ParsePost(bp)
	if err != nil {
		t.Fatal(err)
	}
	s.ck("save.blockListRoundTrip", strings.Contains(bgot, `tags: ["hugo", "fixit"]`) &&
		strings.Contains(bgot, `keywords: ["Hugo", "FixIt"]`) && !strings.Contains(bgot, "  - hugo") &&
		strings.Join(ra["tags"], ",") == "hugo,fixit")

	// ---- 写回（外科手术式）----
	// r1: 改标题，不动其余字段
	p := copyPost(t, site, helloRel)
	if err := SavePost(p, map[string]string{"title": "改名后"}, nil, "正文不变。\n"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, p)
	s.ck("save.keepOthers", strings.Contains(got, `title: "改名后"`) && strings.Contains(got, "subtitle: 第一篇文章"))
	s.ck("save.keepNestedMap", strings.Contains(got, "repost:\n  enable: false"))
	s.ck("save.keepBlockList", strings.Contains(got, "keywords:\n  - Hugo\n  - FixIt"))

	// r1b: 显式传空字符串 = 删除该字段（编辑器清空语义）
	p = copyPost(t, site, helloRel)
	if err := SavePost(p, map[string]string{"subtitle": ""}, nil, "正文不变。\n"); err != nil {
		t.Fatal(err)
	}
	got = readFile(t, p)
	s.ck("save.clearScalar", !strings.Contains(got, "subtitle") && strings.Contains(got, "date: 2024-01-15T10:00:00+08:00"))

	p = copyPost(t, site, lockedRel)
	if err := SavePost(p, map[string]string{"password": "", "message": ""}, nil, "正文不变。\n"); err != nil {
		t.Fatal(err)
	}
	got = readFile(t, p)
	s.ck("save.clearPassword", !strings.Contains(got, "password:") && !strings.Contains(got, "message:") && strings.Contains(got, "weight: 5"))

	// r2: 编辑 FixIt 专有字段（通用标量）
	p = copyPost(t, site, helloRel)
	err = SavePost(p, map[string]string{
		"subtitle":       "新副标题",
		"featured_image": "/images/new.jpg",
		"weight":         "9",
	}, nil, "正文不变。\n")
	if err != nil {
		t.Fatal(err)
	}
	got = readFile(t, p)
	s.ck("save.subtitle", strings.Contains(got, `subtitle: "新副标题"`))
	s.ck("save.featuredImage", strings.Contains(got, `featured_image: "/images/new.jpg"`))
	s.ck("save.weightBareNumber", strings.Contains(got, "weight: 9\n"))

	// r2b: 日期类字段裸写（lastmod 等，YAML 时间戳不应加引号）
	s.ck("save.lastmodBare", func() bool {
		p := copyPost(t, site, helloRel)
		if err := SavePost(p, map[string]string{"lastmod": "2024-06-01T00:00:00+08:00"}, nil, "正文不变。\n"); err != nil {
			t.Fatal(err)
		}
		return strings.Contains(readFile(t, p), "lastmod: 2024-06-01T00:00:00+08:00\n")
	}())

	// r3: 布尔与数组字段
	p = copyPost(t, site, lockedRel)
	err = SavePost(p,
		map[string]string{"hidden_from_home_page": "false", "password": "newpass"},
		map[string][]string{"collections": {"合集A", "合集B"}},
		"正文不变。\n")
	if err != nil {
		t.Fatal(err)
	}
	got = readFile(t, p)
	s.ck("save.bareBool", strings.Contains(got, "hidden_from_home_page: false\n"))
	s.ck("save.password", strings.Contains(got, `password: "newpass"`))
	s.ck("save.collections", strings.Contains(got, `collections: ["合集A", "合集B"]`))

	// ---- 新建与方案 ----
	schema := ThemeSchema("FixIt")
	wantKeys := []string{"subtitle", "weight", "featured_image", "collections", "hidden_from_home_page"}
	seen := map[string]bool{}
	for _, f := range schema {
		seen[f.Key] = true
	}
	schemaOK := len(schema) > 0
	for _, k := range wantKeys {
		schemaOK = schemaOK && seen[k]
	}
	s.ck("schema.fixit", schemaOK)

	tmpSite := copySite(t, site)
	if err := CreateFallbackPost(tmpSite, "posts/fallback-check.md", "兜底文章"); err != nil {
		t.Fatal(err)
	}
	fb := readFile(t, filepath.Join(tmpSite, "content", "posts", "fallback-check.md"))
	s.ck("fallback.fixitFields", strings.Contains(fb, "subtitle:") && strings.Contains(fb, "featured_image:"))

	// ---- 路径安全（新建文章的 section 输入是信任边界）----
	s.ck("safe.sanitizeSection", SanitizeSection("../etc") == "posts" && SanitizeSection("ok-name_1") == "ok-name_1")

	base := t.TempDir()
	safeSite := filepath.Join(base, "site")
	if err := os.MkdirAll(safeSite, 0o755); err != nil {
		t.Fatal(err)
	}
	err = CreateFallbackPost(safeSite, "../evil.md", "x")
	_, statErr := os.Stat(filepath.Join(base, "evil.md"))
	s.ck("safe.fallbackNoEscape", err != nil && os.IsNotExist(statErr))

	// ---- TOML front matter（FixIt 同样支持）----
	tomlRel := filepath.Join("content", "posts", "toml-post.md")
	fmt2, tf, ta, _, _, err := ParsePost(filepath.Join(site, tomlRel))
	if err != nil {
		t.Fatal(err)
	}
	s.ck("toml.parse", fmt2 == "toml" && tf["subtitle"] == "toml 副标题" && tf["weight"] == "2" && strings.Join(ta["collections"], ",") == "TOML集")

	p = copyPost(t, site, tomlRel)
	err = SavePost(p, map[string]string{"title": "TOML 改名", "subtitle": "新副标", "weight": "8"}, nil, "TOML 正文。\n")
	if err != nil {
		t.Fatal(err)
	}
	got = readFile(t, p)
	s.ck("toml.saveScalar", strings.Contains(got, `subtitle = "新副标"`) && strings.Contains(got, "weight = 8\n") && strings.Contains(got, "date = 2024-02-01T09:00:00+08:00"))
	s.ck("toml.keepTable", strings.Contains(got, "[repost]\nenable = false\nurl = \"\""))

	// ---- TOML 多行行内数组 ----
	mlRel := filepath.Join("content", "posts", "toml-multiline.md")
	_, _, ma, _, _, err := ParsePost(filepath.Join(site, mlRel))
	if err != nil {
		t.Fatal(err)
	}
	s.ck("toml.parseMultiLineArray", strings.Join(ma["tags"], ",") == "长标签甲,长标签乙")

	p = copyPost(t, site, mlRel)
	if err := SavePost(p, nil, map[string][]string{"tags": {"新标签"}}, "正文。\n"); err != nil {
		t.Fatal(err)
	}
	got = readFile(t, p)
	noOrphan := true
	for _, ln := range strings.Split(got, "\n") {
		if strings.TrimSpace(ln) == "]" { // 孤立的 ] = 旧数组没删干净
			noOrphan = false
		}
	}
	_, _, ma2, _, _, err := ParsePost(p)
	if err != nil {
		t.Fatal(err)
	}
	s.ck("toml.saveMultiLineArray", noOrphan && strings.Contains(got, `tags = ["新标签"]`) && strings.Join(ma2["tags"], ",") == "新标签")
	// ---- 编码健壮性（SavePost 声称兼容 CRLF / BOM，做回归守卫）----
	crlf := strings.ReplaceAll(readFile(t, filepath.Join(site, helloRel)), "\n", "\r\n")
	p2 := filepath.Join(t.TempDir(), "crlf.md")
	if err := os.WriteFile(p2, []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SavePost(p2, map[string]string{"subtitle": "CRLF 副标题"}, nil, "CRLF 正文。"); err != nil {
		t.Fatal(err)
	}
	got = readFile(t, p2)
	s.ck("save.crlf", strings.Contains(got, "subtitle: \"CRLF 副标题\"\r\n") && !strings.Contains(got, "\r\r"))

	bom := "\uFEFF" + readFile(t, filepath.Join(site, helloRel))
	p2 = filepath.Join(t.TempDir(), "bom.md")
	if err := os.WriteFile(p2, []byte(bom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SavePost(p2, map[string]string{"subtitle": "BOM 副标题"}, nil, "BOM 正文。"); err != nil {
		t.Fatal(err)
	}
	s.ck("save.bom", strings.HasPrefix(readFile(t, p2), "\uFEFF---"))

	// ---- hugo new 原型（FixIt 站点应默认用 posts 原型）----
	newSite := copySite(t, site)
	if out, err := exec.Command("cmd", "/c", "mklink", "/J",
		filepath.Join(newSite, "themes"), themesDir).CombinedOutput(); err != nil {
		t.Logf("mklink: %v %s", err, out)
		s.ck("newcontent.fixitKind", false)
	} else {
		a2 := NewApp(filepath.Join(t.TempDir(), "config.json"))
		a2.SetConfig(Config{SiteDir: newSite, HugoBin: hugoBin})
		if err := a2.NewContent("posts/kind-check.md", "", "原型检查"); err != nil {
			t.Logf("NewContent: %v", err)
			s.ck("newcontent.fixitKind", false)
		} else {
			nc := readFile(t, filepath.Join(newSite, "content", "posts", "kind-check.md"))
			s.ck("newcontent.fixitKind", strings.Contains(nc, "featured_image:"))
		}
	}

	// ---- 端到端：真实主题构建 ----
	s.ck("e2e.buildPristine", hugoBuild(t, hugoBin, copySite(t, site), themesDir, filepath.Join(t.TempDir(), "public")))

	editSite := copySite(t, site)
	if err := SavePost(filepath.Join(editSite, helloRel),
		map[string]string{"title": "端到端编辑", "subtitle": "端到端副标题", "toc": "false", "math": "true", "comment": "false"},
		map[string][]string{"tags": {"hugo", "fixit", "e2e"}},
		"端到端正文。\n"); err != nil {
		t.Fatal(err)
	}
	s.ck("e2e.buildAfterEdit", hugoBuild(t, hugoBin, editSite, themesDir, filepath.Join(t.TempDir(), "public")))

	fmt.Printf("METRIC fixit_core=%d\n", s.score)
	fmt.Printf("METRIC fixit_core_total=%d\n", fixitCoreTotal)
}
