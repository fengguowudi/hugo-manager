package core

// 本文件：hugo 二进制的进程管理 —— 版本探测、预览服务器 (hugo server)、
// 构建 (hugo)、新建文章 (hugo new)，以及日志推流。

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pipeLogs 把子进程 stdout/stderr 逐行推送到 hub（打上 [预览]/[构建] 标签）。
func pipeLogs(cmd *exec.Cmd, hub *Hub, label string) error {
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	errOut, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	scan := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			hub.Log("[" + label + "] " + sc.Text())
		}
	}
	go scan(out)
	go scan(errOut)
	return nil
}

// serverProc 跟踪 hugo server 子进程（同一时刻最多一个）。
type serverProc struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	port    int
}

func (s *serverProc) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// StartServer 启动 `hugo server -D --port N`（-D = 预览含草稿，方便写作）。
func (a *App) StartServer() error {
	cfg := a.ConfigSnapshot()
	if cfg.SiteDir == "" {
		return errors.New("请先在「设置」中填写站点目录")
	}
	bin, err := a.resolveHugo(cfg)
	if err != nil {
		return errors.New("未找到 hugo 可执行文件，请先在「设置」中配置")
	}
	s := a.srv
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("预览服务器已在运行")
	}
	cmd := exec.Command(bin, "server", "-D", "--port", strconv.Itoa(cfg.ServerPort))
	cmd.Dir = cfg.SiteDir
	if err := pipeLogs(cmd, a.hub, "预览"); err != nil {
		s.mu.Unlock()
		return err
	}
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("启动失败: %w", err)
	}
	s.cmd, s.running, s.port = cmd, true, cfg.ServerPort
	port := cfg.ServerPort
	s.mu.Unlock()

	a.hub.Log(fmt.Sprintf("预览服务器已启动: http://localhost:%d/", port))
	a.hub.State()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		s.cmd, s.running = nil, false
		s.mu.Unlock()
		if err != nil {
			a.hub.Log("预览服务器异常退出: " + err.Error())
		} else {
			a.hub.Log("预览服务器已停止。")
		}
		a.hub.State()
	}()
	return nil
}

func (a *App) StopServer() error {
	s := a.srv
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.cmd == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

// RunBuild 同步执行 `hugo [-D]`，日志实时推流，返回构建是否成功。
func (a *App) RunBuild() error {
	cfg := a.ConfigSnapshot()
	bin, err := a.resolveHugo(cfg)
	if err != nil {
		return errors.New("未找到 hugo 可执行文件，请先在「设置」中配置")
	}
	if a.srv.IsRunning() {
		return errors.New("请先停止预览服务器，再执行构建（避免构建锁冲突）")
	}
	args := []string{}
	if cfg.IncludeDrafts {
		args = append(args, "-D") // 构建时包含 draft: true 的文章
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = cfg.SiteDir
	if err := pipeLogs(cmd, a.hub, "构建"); err != nil {
		return err
	}
	a.hub.Log("$ hugo " + strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}
	err = cmd.Wait()
	if err != nil {
		a.hub.Log("构建失败: " + err.Error())
	} else {
		a.hub.Log("构建完成，输出目录: public/")
	}
	a.hub.State()
	return err
}

// NewContent 新建文章：优先 `hugo new`（自动套用站点原型 archetype），
// hugo 不可用时退化为直接写一个带最简 front matter 的文件。
func (a *App) NewContent(rel, kind, title string) error {
	cfg := a.ConfigSnapshot()
	if cfg.SiteDir == "" {
		return errors.New("请先在「设置」中填写站点目录")
	}
	if rel == "" || strings.Contains(rel, "..") { // 防目录穿越（hugo new 路径同样要守）
		return errors.New("非法路径")
	}
	bin, err := a.resolveHugo(cfg)
	if err == nil {
		contentRel := ContentRelPath(cfg.SiteDir, rel)
		args := []string{"new"}
		if kind != "" {
			args = append(args, "-k", kind)
		}
		args = append(args, contentRel)
		cmd := exec.Command(bin, args...)
		cmd.Dir = cfg.SiteDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("hugo new 失败: %s", strings.TrimSpace(string(out)))
		}
		if title != "" {
			p := filepath.Join(cfg.SiteDir, filepath.FromSlash(contentRel))
			fmFmt, _, _, body, _, err := ParsePost(p)
			if err != nil {
				return fmt.Errorf("读取新文章失败: %w", err)
			}
			if fmFmt == "" {
				return errors.New("hugo new 生成的文章缺少可识别的 front matter")
			}
			if err := SavePost(p, map[string]string{"title": title}, nil, body); err != nil {
				return fmt.Errorf("写入新文章标题失败: %w", err)
			}
		}
		return nil
	}
	return CreateFallbackPost(cfg.SiteDir, rel, title)
}

// CreateFallbackPost 无 hugo 二进制时的兜底：content/<rel> 写最简模板。
// 识别到主题（如 FixIt）时套用该主题的字段布局。
func CreateFallbackPost(siteDir, rel, title string) error {
	if rel == "" || strings.Contains(rel, "..") { // 与 SafeSitePath 同一规则，防目录穿越
		return errors.New("非法路径")
	}
	if title == "" {
		title = strings.TrimSuffix(rel, ".md")
		title = strings.NewReplacer("-", " ", "_", " ").Replace(title)
	}
	p := filepath.Join(siteDir, filepath.FromSlash(ContentRelPath(siteDir, rel)))
	if err := os.MkdirAll(dirOf(p), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	fixit := DetectTheme(siteDir) == "FixIt"
	if fixit { // 参照 FixIt archetypes/posts.md 的字段布局
		b.WriteString("subtitle: \n")
	}
	fmt.Fprintf(&b, "date: %s\ndraft: true\n", time.Now().Format("2006-01-02T15:04:05-07:00"))
	if fixit {
		b.WriteString("description: \nkeywords: []\nweight: 0\ntags: []\ncategories: []\ncollections: []\nfeatured_image: \nfeatured_image_preview: \n")
	}
	b.WriteString("---\n\n")
	return os.WriteFile(p, []byte(b.String()), 0o644)
}

func dirOf(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[:i]
	}
	return "."
}
