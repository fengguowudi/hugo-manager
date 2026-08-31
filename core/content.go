package core

// 本文件：content/ 目录扫描、front matter 的读取与"外科手术式"写回。
// 写回策略：只替换被编辑字段的所在行，其余内容（含未知自定义字段）逐字节保留，
// 因此不会破坏用户手工维护的 front matter。仅支持 YAML(---) 与 TOML(+++)，
// JSON front matter 只能整体查看（不做结构化编辑）。

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Post 列表页需要的文章摘要信息。
type Post struct {
	Path      string `json:"path"` // 相对站点根目录，如 content/posts/hello.md
	Title     string `json:"title"`
	Section   string `json:"section"` // content/ 下第一级目录名
	Draft     bool   `json:"draft"`
	Encrypted bool   `json:"encrypted"` // 含 password（FixIt 内容加密，构建后需 @hugo-fixit/encrypt）
	Expired   bool   `json:"expired"`   // expiryDate 已过（Hugo 构建时会静默跳过）
	Scheduled bool   `json:"scheduled"` // publishDate 在未来（到时间前 Hugo 不发布）
	Date      string `json:"date"`      // front matter 原始字符串（ISO 排序友好）
	Kind      string `json:"kind"`      // page=普通文章 section=列表页(_index.md) bundle=页面包(index.md)
}

// parseFMDate 解析 front matter 日期（RFC3339 或纯日期）；失败返回 false。
func parseFMDate(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false // 解析不了不算数，不误报
}

// expired 判断 expiryDate 是否已过（Hugo 构建时静默跳过）。
func expired(v string) bool {
	t, ok := parseFMDate(v)
	return ok && t.Before(time.Now())
}

// scheduled 判断 publishDate 是否在未来（到时间前 Hugo 不发布）。
func scheduled(v string) bool {
	t, ok := parseFMDate(v)
	return ok && t.After(time.Now())
}

var (
	reYamlKV = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)\s*:\s*(.*)$`)
	reTomlKV = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*(.*)$`)
)

// ParsePost 解析一个内容文件：返回 front matter 格式、标量字段、数组字段、正文、原始全文。
// 没有 front matter 时 format 为空、body 即全文。
func ParsePost(p string) (fmFmt string, fields map[string]string, arrays map[string][]string, body, raw string, err error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	raw = strings.TrimPrefix(string(b), "\uFEFF") // 去掉可能的 UTF-8 BOM
	lines := strings.Split(raw, "\n")
	fields, arrays = map[string]string{}, map[string][]string{}

	first := strings.TrimRight(lines[0], "\r")
	var delim string
	switch first {
	case "---":
		fmFmt, delim = "yaml", "---"
	case "+++":
		fmFmt, delim = "toml", "+++"
	}
	if fmFmt == "" {
		body = raw
		return
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == delim {
			end = i
			break
		}
	}
	if end == -1 { // 只有开分隔符、没有闭分隔符 → 当作无 front matter
		fmFmt, body = "", raw
		return
	}
	re := reYamlKV
	if fmFmt == "toml" {
		re = reTomlKV
	}
	for i := 1; i < end; i++ {
		if fmFmt == "toml" && isTOMLTableHeader(lines[i]) {
			break // table 之后的键属于嵌套对象，不是页面根字段
		}
		m := re.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		k := m[1]
		v, _ := splitInlineComment(strings.TrimSpace(m[2]), fmFmt == "toml")
		if v == "" { // 块式列表（YAML）：key: 后跟若干 - item 行
			// ponytail: 只认缩进的 "- " 项（最常见的写法）；不处理顶格列表项或 - key: value 映射项
			var items []string
			for i+1 < end {
				t, _ := splitInlineComment(strings.TrimSpace(strings.TrimRight(lines[i+1], "\r")), false)
				if t != "-" && !strings.HasPrefix(t, "- ") {
					break
				}
				i++
				if item := strings.TrimSpace(strings.TrimPrefix(t, "-")); item != "" {
					items = append(items, unquote(item))
				}
			}
			if len(items) > 0 {
				arrays[k] = items
				continue
			}
		}
		if strings.HasPrefix(v, "[") && !strings.HasSuffix(v, "]") {
			// 多行行内数组（TOML 常见）：累积到 ] 闭合
			for i+1 < end {
				i++
				part, _ := splitInlineComment(strings.TrimSpace(strings.TrimRight(lines[i], "\r")), fmFmt == "toml")
				v += part
				if strings.HasSuffix(v, "]") {
					break
				}
			}
		}
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			arrays[k] = splitList(v[1 : len(v)-1])
			continue
		}
		fields[k] = unquote(v)
	}
	body = strings.Join(lines[end+1:], "\n")
	return
}

// splitList 拆分行内数组，只在引号外的逗号处分隔。
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var parts []string
	var item strings.Builder
	var quote byte
	escaped := false
	flush := func() {
		if v := unquote(strings.TrimSpace(item.String())); v != "" {
			parts = append(parts, v)
		}
		item.Reset()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			item.WriteByte(c)
			escaped = false
			continue
		}
		if quote != 0 {
			item.WriteByte(c)
			if quote == '"' && c == '\\' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
			item.WriteByte(c)
		case ',':
			flush()
		default:
			item.WriteByte(c)
		}
	}
	flush()
	return parts
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		inner := v[1 : len(v)-1]
		return strings.NewReplacer(`\"`, `"`, `\'`, `'`, `\\`, `\`).Replace(inner)
	}
	return v
}

// splitInlineComment 分离 front matter 值与引号外的 # 行尾注释。
// YAML 的 # 仅在行首或前有空白时视为注释（保留 URL#fragment / C#）；TOML 在引号外即为注释。
func splitInlineComment(s string, toml bool) (value, comment string) {
	var quote byte
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '#' && (toml || i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			start := i
			for start > 0 && (s[start-1] == ' ' || s[start-1] == '\t') {
				start--
			}
			return strings.TrimSpace(s[:start]), s[start:]
		}
	}
	return strings.TrimSpace(s), ""
}

func inlineCommentSuffix(line string) string {
	i := strings.IndexAny(line, ":=")
	if i < 0 {
		return ""
	}
	_, comment := splitInlineComment(line[i+1:], line[i] == '=')
	return comment
}

// SavePost 结构化保存：仅改写被编辑字段所在行，正文整体替换，其余保留。
// 兼容 CRLF 文件：统一先归一为 LF 再按原换行风格重组，避免 \r 翻倍。
func SavePost(p string, fields map[string]string, arrays map[string][]string, body string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	orig := string(b)
	bom := strings.HasPrefix(orig, "\uFEFF")
	raw := strings.TrimPrefix(orig, "\uFEFF")
	eol := "\n"
	if strings.Contains(raw, "\r\n") {
		eol = "\r\n" // 保持原有换行风格
	}
	lines := strings.Split(raw, "\n")
	first := strings.TrimRight(lines[0], "\r")
	var delim, fmFmt string
	switch first {
	case "---":
		fmFmt, delim = "yaml", "---"
	case "+++":
		fmFmt, delim = "toml", "+++"
	}
	if fmFmt == "" {
		return errors.New("该文件没有 front matter，暂不支持结构化编辑")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == delim {
			end = i
			break
		}
	}
	if end == -1 {
		return errors.New("front matter 格式不完整（缺少结束分隔符）")
	}
	toml := fmFmt == "toml"
	lineFor := func(k, v string) string {
		if toml {
			return k + " = " + v
		}
		return k + ": " + v
	}
	quote := func(s string) string { return strconv.Quote(s) } // YAML/TOML 双引号字符串通用

	fm := make([]string, 0, end)
	for _, ln := range lines[1:end] {
		fm = append(fm, strings.TrimRight(ln, "\r")) // 行尾 \r 归一，之后统一按 eol 重组
	}
	// 标量字段：布尔 / 数字 / 日期时间裸写，其余加引号；空值 = 删除该字段（清空语义）
	for _, k := range sortedKeys(fields) {
		v := fields[k]
		if v == "" {
			fm = delLine(fm, k, toml)
			continue
		}
		val := quote(v)
		if bareValue(v) {
			val = v
		}
		fm = setLine(fm, k, lineFor(k, val), toml)
	}
	// 数组字段：空数组 = 删除该字段（含块式列表的残留行）
	for _, k := range sortedKeys(arrays) {
		v := arrays[k]
		if len(v) == 0 {
			fm = delLine(fm, k, toml)
			continue
		}
		parts := make([]string, len(v))
		for i, t := range v {
			parts[i] = quote(t)
		}
		fm = setLine(fm, k, lineFor(k, "["+strings.Join(parts, ", ")+"]"), toml)
	}

	newLines := append([]string{first}, fm...)
	newLines = append(newLines, delim)
	newLines = append(newLines, strings.Split(normalizeEOL(body), "\n")...)
	out := strings.Join(newLines, "\n")
	out = strings.ReplaceAll(out, "\n", eol)
	if !strings.HasSuffix(out, eol) {
		out += eol
	}
	if bom {
		out = "\uFEFF" + out
	}
	return os.WriteFile(p, []byte(out), 0o644)
}

// normalizeEOL 把 CRLF / 孤立 CR 统一为 LF。
func normalizeEOL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func isTOMLTableHeader(line string) bool {
	v, _ := splitInlineComment(strings.TrimSpace(strings.TrimRight(line, "\r")), true)
	return strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]")
}

func keyLineRe(key string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `\s*[:=]`)
}

// sortedKeys 返回 map 键的排序副本，保证写回顺序稳定。
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// bareValue 判断标量是否可裸写（不加引号）：布尔、数字、日期时间。
var reBare = regexp.MustCompile(`^(true|false|-?[0-9]+(\.[0-9]+)?|[0-9]{4}-[0-9]{2}-[0-9]{2}([T ][0-9]{2}:[0-9]{2}.*)?)$`)

func bareValue(v string) bool { return reBare.MatchString(strings.TrimSpace(v)) }

// blockEnd 返回 key 行所属块的结束下标（不含）：
// 其后所有缩进行（块式列表项 / 嵌套 map）都属于这个键；
// 若键行以 [ 开头且方括号未闭合（多行行内数组），继续吞到闭合——TOML 允许 ] 顶格。
func blockEnd(lines []string, i int) int {
	j := i + 1
	for j < len(lines) && (strings.HasPrefix(lines[j], " ") || strings.HasPrefix(lines[j], "\t")) {
		j++
	}
	kv := strings.IndexAny(lines[i], ":=")
	if kv < 0 || !strings.HasPrefix(strings.TrimSpace(lines[i][kv+1:]), "[") {
		return j
	}
	depth := 0
	for k := i; k < j; k++ {
		depth += bracketDepth(lines[k])
	}
	for j < len(lines) && depth > 0 {
		depth += bracketDepth(lines[j])
		j++
	}
	return j
}

// bracketDepth 统计引号外 [ 与 ] 的净差；字符串字面量和 # 注释内括号不参与结构。
func bracketDepth(s string) int {
	d := 0
	var quote byte
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && c == '\\' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '#':
			return d
		case '[':
			d++
		case ']':
			d--
		}
	}
	return d
}

func rootFieldLimit(lines []string, toml bool) int {
	if toml {
		for i, line := range lines {
			if isTOMLTableHeader(line) {
				return i
			}
		}
	}
	return len(lines)
}

// setLine 替换页面根 key 所在行（连同块式内容）。
// TOML 不存在该 key 时插入首个 table 前，避免新键落入最后一个 table。
func setLine(lines []string, key, line string, toml bool) []string {
	re := keyLineRe(key)
	limit := rootFieldLimit(lines, toml)
	for i := 0; i < limit; i++ {
		if re.MatchString(lines[i]) {
			if suffix := inlineCommentSuffix(lines[i]); suffix != "" {
				line += suffix
			}
			return append(lines[:i], append([]string{line}, lines[blockEnd(lines, i):]...)...)
		}
	}
	if toml && limit < len(lines) {
		insertAt := limit
		for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:insertAt]...)
		out = append(out, line)
		out = append(out, lines[insertAt:]...)
		return out
	}
	return append(lines, line)
}

// delLine 删除页面根 key 所在行及其块式内容；TOML table 内同名键不动。
func delLine(lines []string, key string, toml bool) []string {
	re := keyLineRe(key)
	limit := rootFieldLimit(lines, toml)
	for i := 0; i < limit; i++ {
		if re.MatchString(lines[i]) {
			return append(lines[:i], lines[blockEnd(lines, i):]...)
		}
	}
	return lines
}

// ListPosts 扫描各语言内容目录（ContentRoots）下全部 Markdown 文件（跳过隐藏目录）。
func ListPosts(siteDir string) ([]Post, error) {
	roots := ContentRoots(siteDir)
	var posts []Post
	for _, rootRel := range roots {
		posts = append(posts, listPostsIn(siteDir, rootRel, roots)...)
	}
	sortPosts(posts)
	return posts, nil
}

// listPostsIn 扫描单个内容根；Section 取相对于该内容根的第一级目录。
// 扫描父根时跳过另一个已声明的嵌套根，避免同一文章重复读取。
func listPostsIn(siteDir, rootRel string, allRoots []string) []Post {
	root := filepath.Join(siteDir, filepath.FromSlash(rootRel))
	var posts []Post
	if _, err := os.Stat(root); err != nil {
		return posts // 无此内容目录 → 空列表
	}
	nestedRoots := map[string]bool{}
	for _, other := range allRoots {
		if other == rootRel {
			continue
		}
		otherPath := filepath.Join(siteDir, filepath.FromSlash(other))
		rel, err := filepath.Rel(root, otherPath)
		if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			nestedRoots[filepath.Clean(otherPath)] = true
		}
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if nestedRoots[filepath.Clean(p)] {
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".markdown") {
			return nil
		}
		rel, _ := filepath.Rel(siteDir, p)
		rel = filepath.ToSlash(rel)
		_, fields, _, _, _, _ := ParsePost(p)
		title := fields["title"]
		if title == "" { // 无标题时用文件名兜底
			base := strings.TrimSuffix(name, filepath.Ext(name))
			title = strings.NewReplacer("-", " ", "_", " ").Replace(base)
		}
		post := Post{
			Path: rel, Title: title,
			Draft: fields["draft"] == "true", Encrypted: fields["password"] != "",
			Expired:   expired(fields["expiryDate"]),
			Scheduled: scheduled(fields["publishDate"]),
			Date:      fields["date"],
		}
		if sub := strings.TrimPrefix(rel, rootRel+"/"); sub != rel {
			parts := strings.Split(sub, "/")
			if len(parts) > 1 {
				post.Section = parts[0]
			}
		}
		switch name {
		case "_index.md":
			post.Kind = "section"
		case "index.md":
			post.Kind = "bundle"
		}
		posts = append(posts, post)
		return nil
	})
	return posts
}

func sortPosts(posts []Post) {
	sort.Slice(posts, func(i, j int) bool {
		di, dj := posts[i].Date, posts[j].Date
		if di == "" {
			return false
		}
		if dj == "" {
			return true
		}
		return di > dj // 日期倒序，最新在前
	})
}

// SiteInfo 从站点配置文件（hugo.toml/config.toml/...）尽力读出标题与 baseURL。
func SiteInfo(siteDir string) (title, baseURL string) {
	names := []string{"hugo.toml", "hugo.yaml", "hugo.yml", "config.toml", "config.yaml", "config.yml", "hugo.json", "config.json"}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(siteDir, n))
		if err != nil {
			continue
		}
		s := string(b)
		title = pick(s, []string{
			`(?m)^\s*title\s*=\s*"([^"]*)"`,
			`(?m)^\s*"title"\s*:\s*"([^"]*)"`,
			`(?m)^\s*title\s*:\s*["']?([^"'\r\n#]+?)["']?\s*$`,
		})
		baseURL = pick(s, []string{
			`(?m)^\s*baseURL\s*=\s*"([^"]*)"`,
			`(?m)^\s*"baseURL"\s*:\s*"([^"]*)"`,
			`(?m)^\s*baseURL\s*:\s*["']?([^"'\r\n#]+?)["']?\s*$`,
		})
		return
	}
	return
}

func pick(s string, patterns []string) string {
	for _, pat := range patterns {
		if m := regexp.MustCompile(pat).FindStringSubmatch(s); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}
