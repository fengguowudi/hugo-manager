package core

// 本文件：主题识别与主题定制的 front matter 字段方案（FixIt 适配入口）。

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Field 描述一个可结构化编辑的 front matter 字段。
type Field struct {
	Key   string // front matter 键名（如 featured_image）
	Label string // 界面显示名
	Kind  string // text | bool | number | list | date
}

var (
	reThemeToml  = regexp.MustCompile(`(?m)^\s*theme\s*=\s*"([^"]+)"`)
	reThemeTomlL = regexp.MustCompile(`(?m)^\s*themes?\s*=\s*\[\s*"([^"]+)"`)
	reThemeYaml  = regexp.MustCompile(`(?m)^\s*theme\s*:\s*["']?([^"'\s]+)`)
)

// knownModules 把 hugo 模块路径映射为主题名（go.mod 方式安装的主题）。
var knownModules = map[string]string{
	"github.com/hugo-fixit/FixIt": "FixIt",
}

// DetectTheme 识别站点使用的主题名（如 "FixIt"）；未识别返回空串。
// 依次尝试：站点配置的 theme 键、go.mod 模块引用、themes/ 目录下的 theme.toml。
func DetectTheme(siteDir string) string {
	names := []string{"hugo.toml", "config.toml", "hugo.yaml", "hugo.yml", "config.yaml", "config.yml"}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(siteDir, n))
		if err != nil {
			continue
		}
		s := string(b)
		for _, re := range []*regexp.Regexp{reThemeToml, reThemeTomlL, reThemeYaml} {
			if m := re.FindStringSubmatch(s); m != nil {
				return filepath.Base(strings.TrimSpace(m[1]))
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(siteDir, "go.mod")); err == nil {
		for mod, name := range knownModules {
			if strings.Contains(string(b), mod) {
				return name
			}
		}
	}
	// themes/ 目录兜底：读取各子目录 theme.toml 的 name
	entries, err := os.ReadDir(filepath.Join(siteDir, "themes"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(siteDir, "themes", e.Name(), "theme.toml")); err == nil {
			if m := regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`).FindStringSubmatch(string(b)); m != nil {
				return m[1]
			}
		}
		return e.Name()
	}
	return ""
}

var reHugoVersion = regexp.MustCompile(`v([0-9]+\.[0-9]+\.[0-9]+)`)

// ParseHugoVersion 从 `hugo version` 输出提取语义版本，如 "0.165.0"。
func ParseHugoVersion(out string) string {
	if m := reHugoVersion.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// knownMinHugo 主题要求的最低 Hugo 版本兜底表（站点内读不到 theme.toml 时用，
// 值来自对应主题当前版本的 theme.toml）。
var knownMinHugo = map[string]string{"FixIt": "0.161.0"}

var reMinVersion = regexp.MustCompile(`(?m)^\s*min_version\s*=\s*"([^"]+)"`)

// ThemeMinHugo 返回主题要求的最低 Hugo 版本：优先站点 themes/<name>/theme.toml，
// 其次内置兜底表；未知返回空串。
func ThemeMinHugo(siteDir, theme string) string {
	if theme == "" {
		return ""
	}
	if b, err := os.ReadFile(filepath.Join(siteDir, "themes", theme, "theme.toml")); err == nil {
		if m := reMinVersion.FindStringSubmatch(string(b)); m != nil {
			return m[1]
		}
	}
	return knownMinHugo[theme]
}

// CompatWarning 当 hugo version 输出显示版本低于主题最低要求时返回中文警告，否则空串。
func CompatWarning(hugoVersionOut, minVer, theme string) string {
	v := ParseHugoVersion(hugoVersionOut)
	if v == "" || minVer == "" || theme == "" {
		return ""
	}
	if semverLess(v, minVer) {
		return "Hugo 版本 v" + v + " 低于主题 " + theme + " 要求的最低版本 " + minVer + "，构建可能失败，请升级 Hugo"
	}
	return ""
}

// semverLess 比较 x.y.z 版本号（数值逐段比较）。
func semverLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na < nb
		}
	}
	return len(pa) < len(pb)
}

// ---- 多语言站点（languages.*.contentDir）----
// 支持站点根目录的 TOML/YAML 配置；config/ 目录拆分配置需要时再扩展。

var reDefaultLang = regexp.MustCompile(`(?m)^\s*defaultContentLanguage\s*=\s*"([^"]+)"`)
var reContentDirKV = regexp.MustCompile(`(?m)^\s*contentDir\s*=\s*"([^"]+)"`)

func readSiteConfigFile(siteDir string) (name, content string) {
	for _, n := range []string{"hugo.toml", "config.toml", "hugo.yaml", "hugo.yml", "config.yaml", "config.yml"} {
		if b, err := os.ReadFile(filepath.Join(siteDir, n)); err == nil {
			return n, string(b)
		}
	}
	return "", ""
}

// tomlSection 返回 TOML 顶级 section 的正文；避免依赖 RE2 不支持的 lookahead。
func tomlSection(s, name string) string {
	target := "[" + name + "]"
	lines := strings.Split(s, "\n")
	active := false
	var body strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			active = trimmed == target
			continue
		}
		if active {
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	return body.String()
}

type yamlLanguageConfig struct {
	ContentDir string `yaml:"contentDir"`
}

type yamlSiteLanguageConfig struct {
	DefaultContentLanguage string                        `yaml:"defaultContentLanguage"`
	Languages              map[string]yamlLanguageConfig `yaml:"languages"`
}

// languageContentDirs 返回默认语言目录及所有显式语言目录。
func languageContentDirs(siteDir string) (string, []string) {
	name, s := readSiteConfigFile(siteDir)
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		var cfg yamlSiteLanguageConfig
		if yaml.Unmarshal([]byte(s), &cfg) == nil {
			defaultLang := cfg.DefaultContentLanguage
			if defaultLang == "" {
				defaultLang = "en" // Hugo 默认语言
			}
			defaultDir := "content"
			if lang, ok := cfg.Languages[defaultLang]; ok && strings.TrimSpace(lang.ContentDir) != "" {
				defaultDir = lang.ContentDir
			}
			var explicit []string
			for _, lang := range cfg.Languages {
				if strings.TrimSpace(lang.ContentDir) != "" {
					explicit = append(explicit, lang.ContentDir)
				}
			}
			return defaultDir, explicit
		}
	}

	defaultDir := "content"
	if m := reDefaultLang.FindStringSubmatch(s); m != nil {
		if body := tomlSection(s, "languages."+m[1]); body != "" {
			if cm := reContentDirKV.FindStringSubmatch(body); cm != nil {
				defaultDir = cm[1]
			}
		}
	}
	var explicit []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(trimmed, "[languages.") && strings.HasSuffix(trimmed, "]") {
			section := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
			if body := tomlSection(s, section); body != "" {
				if cm := reContentDirKV.FindStringSubmatch(body); cm != nil {
					explicit = append(explicit, cm[1])
				}
			}
		}
	}
	return defaultDir, explicit
}

func cleanContentDir(d string) string {
	d = filepath.ToSlash(strings.TrimSpace(d))
	return strings.Trim(d, "/")
}

// DefaultContentDir 返回默认语言的内容目录（单语言站点为 "content"）。
func DefaultContentDir(siteDir string) string {
	d, _ := languageContentDirs(siteDir)
	if d = cleanContentDir(d); d != "" {
		return d
	}
	return "content"
}

// ContentRoots 返回所有需扫描的内容目录：默认语言目录 + 各语言自定义 contentDir（去重）。
func ContentRoots(siteDir string) []string {
	defaultDir, explicit := languageContentDirs(siteDir)
	seen := map[string]bool{}
	var roots []string
	add := func(d string) {
		d = cleanContentDir(d)
		if d != "" && !seen[d] {
			seen[d] = true
			roots = append(roots, d)
		}
	}
	add(defaultDir)
	for _, d := range explicit {
		add(d)
	}
	return roots
}

// ThemeSchema 返回该主题推荐的可编辑字段；任何主题都至少有通用字段，
// FixIt 追加其专有字段（snake_case）。
func ThemeSchema(theme string) []Field {
	fields := []Field{
		{Key: "title", Label: "标题", Kind: "text"},
		{Key: "date", Label: "日期", Kind: "date"},
		{Key: "slug", Label: "Slug", Kind: "text"},
		{Key: "tags", Label: "标签", Kind: "list"},
		{Key: "categories", Label: "分类", Kind: "list"},
		{Key: "description", Label: "摘要", Kind: "text"},
		{Key: "draft", Label: "草稿", Kind: "bool"},
	}
	if theme == "FixIt" {
		fields = append(fields,
			Field{Key: "subtitle", Label: "副标题", Kind: "text"},
			Field{Key: "weight", Label: "置顶权重", Kind: "number"},
			Field{Key: "featured_image", Label: "头图", Kind: "text"},
			Field{Key: "featured_image_preview", Label: "列表预览图", Kind: "text"},
			Field{Key: "collections", Label: "合集", Kind: "list"},
			Field{Key: "keywords", Label: "关键词", Kind: "list"},
			Field{Key: "summary", Label: "小结", Kind: "text"},
			Field{Key: "hidden_from_home_page", Label: "首页隐藏", Kind: "bool"},
			Field{Key: "hidden_from_related", Label: "相关文章隐藏", Kind: "bool"},
			Field{Key: "password", Label: "访问密码", Kind: "text"},
			Field{Key: "message", Label: "密码提示语", Kind: "text"},
			// Hugo 内建日期（FixIt 的"最近更新"/过期提醒依赖 lastmod/expiryDate）
			Field{Key: "lastmod", Label: "更新日期", Kind: "date"},
			Field{Key: "expiryDate", Label: "过期日期", Kind: "date"},
			Field{Key: "publishDate", Label: "发布日期", Kind: "date"},
			// 页面级开关（FixIt 支持布尔简写，见 function/param.html）
			Field{Key: "toc", Label: "目录", Kind: "bool"},
			Field{Key: "math", Label: "数学公式", Kind: "bool"},
			Field{Key: "lightgallery", Label: "图片画廊", Kind: "bool"},
			Field{Key: "comment", Label: "评论", Kind: "bool"},
			Field{Key: "word_count", Label: "字数统计", Kind: "bool"},
			Field{Key: "reading_time", Label: "阅读时长", Kind: "bool"},
			Field{Key: "hidden_from_feed", Label: "订阅源隐藏", Kind: "bool"},
		)
	}
	return fields
}
