package core

// 本文件：主题识别与主题定制的 front matter 字段方案（FixIt 适配入口）。

// Field 描述一个可结构化编辑的 front matter 字段。
type Field struct {
	Key   string // front matter 键名（如 featured_image）
	Label string // 界面显示名
	Kind  string // text | bool | number | list | date
}

// DetectTheme 识别站点使用的主题名（如 "FixIt"）；未识别返回空串。
// 检测途径：hugo.toml 的 theme 配置、themes/ 目录、go.mod 模块引用。
func DetectTheme(siteDir string) string { return "" } // TODO: 实现

// ThemeSchema 返回该主题推荐的可编辑字段；未知主题返回 nil。
func ThemeSchema(theme string) []Field { return nil } // TODO: 实现
