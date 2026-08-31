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

const fixitCoreTotal = 24

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
	_, site, hugoBin, themesDir := benchPaths(t)
	s := &scoreKeeper{t: t}

	// ---- 站点级 ----
	title, baseURL := SiteInfo(site)
	s.ck("site.title", title == "Fixture Blog")
	s.ck("site.baseURL", baseURL == "https://fixture.example.com/")
	s.ck("site.detectTheme", DetectTheme(site) == "FixIt")
	a := NewApp(filepath.Join(t.TempDir(), "config.json"))
	a.SetConfig(Config{SiteDir: site})
	s.ck("state.theme", a.BuildState()["theme"] == "FixIt")

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
	// ---- 端到端：真实主题构建 ----
	s.ck("e2e.buildPristine", hugoBuild(t, hugoBin, copySite(t, site), themesDir, filepath.Join(t.TempDir(), "public")))

	editSite := copySite(t, site)
	if err := SavePost(filepath.Join(editSite, helloRel),
		map[string]string{"title": "端到端编辑", "subtitle": "端到端副标题"},
		map[string][]string{"tags": {"hugo", "fixit", "e2e"}},
		"端到端正文。\n"); err != nil {
		t.Fatal(err)
	}
	s.ck("e2e.buildAfterEdit", hugoBuild(t, hugoBin, editSite, themesDir, filepath.Join(t.TempDir(), "public")))

	fmt.Printf("METRIC fixit_core=%d\n", s.score)
	fmt.Printf("METRIC fixit_core_total=%d\n", fixitCoreTotal)
}
