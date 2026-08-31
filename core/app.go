package core

// 应用核心：配置、状态汇总、路径安全。与 UI 解耦，便于无 CGO 环境下测试。

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config 持久化配置，保存为管理器可执行文件同目录下的 config.json。
type Config struct {
	HugoBin       string `json:"hugoBin"`       // [必填] hugo 可执行文件路径；留空 = 自动从 PATH 查找
	SiteDir       string `json:"siteDir"`       // [必填] 站点根目录（含 hugo.toml / config.toml 的目录）
	ServerPort    int    `json:"serverPort"`    // 预览服务器端口（hugo server --port），默认 1313
	IncludeDrafts bool   `json:"includeDrafts"` // 构建时是否包含草稿（-D）
}

type App struct {
	mu      sync.Mutex
	cfg     Config
	cfgPath string

	hugoVersion string // 探测到的 `hugo version` 输出（缓存）

	hub *Hub
	srv *serverProc
}

// NewApp 读取 cfgPath 处的配置（不存在则用零值），返回可用的 App。
func NewApp(cfgPath string) *App {
	a := &App{cfgPath: cfgPath, hub: NewHub(), srv: &serverProc{}}
	if b, err := os.ReadFile(cfgPath); err == nil {
		_ = json.Unmarshal(b, &a.cfg)
	}
	return a
}

func (a *App) ConfigSnapshot() Config { a.mu.Lock(); defer a.mu.Unlock(); return a.cfg }

// SetConfig 替换配置并持久化。
func (a *App) SetConfig(cfg Config) {
	a.mu.Lock()
	a.cfg = cfg
	a.mu.Unlock()
	a.SaveConfig()
}

// resolveHugo 返回可用的 hugo 命令（显式路径或 PATH 查找结果）。
func (a *App) resolveHugo(cfg Config) (string, error) {
	if cfg.HugoBin == "" {
		return exec.LookPath("hugo")
	}
	return cfg.HugoBin, nil
}

// Probe 探测 hugo 版本并缓存；失败则置空（界面会给出警告）。
func (a *App) Probe() {
	cfg := a.ConfigSnapshot()
	bin, err := a.resolveHugo(cfg)
	if err != nil {
		a.mu.Lock()
		a.hugoVersion = ""
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, bin, "version").Output()
	a.mu.Lock()
	a.hugoVersion = strings.TrimSpace(string(out))
	a.mu.Unlock()
}

func (a *App) HugoWarning() string {
	if a.HugoVersionSnapshot() != "" {
		return ""
	}
	return "未找到可用的 hugo 可执行文件：请安装 Hugo，或在设置中填写 hugo 路径"
}

func (a *App) HugoVersionSnapshot() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.hugoVersion
}

func (a *App) SaveConfig() {
	b, _ := json.MarshalIndent(a.cfg, "", "  ")
	_ = os.WriteFile(a.cfgPath, b, 0o644)
}

// Logs 返回进程日志历史快照。
func (a *App) Logs() []string { return a.hub.HistorySnapshot() }

// SafeSitePath 将站点内相对路径转为绝对路径，并阻止目录穿越。
func (a *App) SafeSitePath(rel string) (string, error) {
	cfg := a.ConfigSnapshot()
	if cfg.SiteDir == "" {
		return "", errors.New("请先在「设置」中填写站点目录")
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", errors.New("非法路径")
	}
	root := filepath.Clean(cfg.SiteDir)
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", errors.New("路径必须位于站点目录内")
	}
	return abs, nil
}

// BuildState 汇总前端所需的全部状态（含文章列表，供「内容」页使用）。
func (a *App) BuildState() map[string]any {
	cfg := a.ConfigSnapshot()
	bin, _ := a.resolveHugo(cfg)
	st := map[string]any{
		"hugoBin":         cfg.HugoBin,
		"hugoBinResolved": bin,
		"hugoVersion":     a.HugoVersionSnapshot(),
		"siteDir":         cfg.SiteDir,
		"serverPort":      cfg.ServerPort,
		"includeDrafts":   cfg.IncludeDrafts,
		"configured":      cfg.SiteDir != "",
		"serverRunning":   a.srv.IsRunning(),
	}
	if cfg.SiteDir == "" {
		return st
	}
	title, baseURL := SiteInfo(cfg.SiteDir)
	posts, _ := ListPosts(cfg.SiteDir)
	total, drafts, encrypted, expiredN, scheduledN := 0, 0, 0, 0, 0
	secs := map[string]bool{}
	for _, p := range posts {
		if p.Kind == "section" {
			continue // _index.md 是列表页，不计入文章数
		}
		total++
		if p.Draft {
			drafts++
		}
		if p.Encrypted {
			encrypted++
		}
		if p.Expired {
			expiredN++
		}
		if p.Scheduled {
			scheduledN++
		}
		if p.Section != "" {
			secs[p.Section] = true
		}
	}
	secList := make([]string, 0, len(secs))
	for s := range secs {
		secList = append(secList, s)
	}
	st["siteTitle"], st["baseURL"] = title, baseURL
	st["posts"] = posts
	st["countTotal"], st["countDrafts"], st["countEncrypted"], st["countExpired"], st["countScheduled"] = total, drafts, encrypted, expiredN, scheduledN
	st["sections"] = secList
	st["theme"] = DetectTheme(cfg.SiteDir)
	if w := CompatWarning(a.HugoVersionSnapshot(), ThemeMinHugo(cfg.SiteDir, st["theme"].(string)), st["theme"].(string)); w != "" {
		st["hugoCompat"] = w
	}
	return st
}

// SanitizeSection keeps quick-created content inside one content subdirectory.
func SanitizeSection(s string) string {
	s = strings.TrimSpace(s)
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return "posts"
		}
	}
	if s == "" {
		return "posts"
	}
	return s
}

func ConfigPath() string {
	exe, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exe), "config.json")
	}
	return "config.json"
}
