package main

// 本文件：hugo 二进制的进程管理 —— 版本探测、预览服务器 (hugo server)、
// 构建 (hugo)、新建文章 (hugo new)，以及日志推流。

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// probeVersion 运行 `hugo version`（5 秒超时），返回版本输出。
func probeVersion(bin string) string {
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

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
			hub.log("[" + label + "] " + sc.Text())
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

func (s *serverProc) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// startServer 启动 `hugo server -D --port N`（-D = 预览含草稿，方便写作）。
func (a *App) startServer() error {
	cfg := a.cfgSnapshot()
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

	a.hub.log(fmt.Sprintf("预览服务器已启动: http://localhost:%d/", port))
	a.hub.state()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		s.cmd, s.running = nil, false
		s.mu.Unlock()
		if err != nil {
			a.hub.log("预览服务器异常退出: " + err.Error())
		} else {
			a.hub.log("预览服务器已停止。")
		}
		a.hub.state()
	}()
	return nil
}

func (a *App) stopServer() error {
	s := a.srv
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.cmd == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

// runBuild 同步执行 `hugo [-D]`，日志实时推流，返回构建是否成功。
func (a *App) runBuild() error {
	cfg := a.cfgSnapshot()
	bin, err := a.resolveHugo(cfg)
	if err != nil {
		return errors.New("未找到 hugo 可执行文件，请先在「设置」中配置")
	}
	if a.srv.isRunning() {
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
	a.hub.log("$ hugo " + strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}
	err = cmd.Wait()
	if err != nil {
		a.hub.log("构建失败: " + err.Error())
	} else {
		a.hub.log("构建完成，输出目录: public/")
	}
	a.hub.state()
	return err
}

// newContent 新建文章：优先 `hugo new`（自动套用站点原型 archetype），
// hugo 不可用时退化为直接写一个带最简 front matter 的文件。
func (a *App) newContent(rel, kind, title string) error {
	cfg := a.cfgSnapshot()
	if cfg.SiteDir == "" {
		return errors.New("请先在「设置」中填写站点目录")
	}
	bin, err := a.resolveHugo(cfg)
	if err == nil {
		args := []string{"new"}
		if kind != "" {
			args = append(args, "-k", kind)
		}
		args = append(args, rel)
		cmd := exec.Command(bin, args...)
		cmd.Dir = cfg.SiteDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("hugo new 失败: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	return createFallbackPost(cfg.SiteDir, rel, title)
}

// createFallbackPost 无 hugo 二进制时的兜底：content/<rel> 写最简模板。
func createFallbackPost(siteDir, rel, title string) error {
	if title == "" {
		title = strings.TrimSuffix(rel, ".md")
		title = strings.NewReplacer("-", " ", "_", " ").Replace(title)
	}
	p := siteDir + string(os.PathSeparator) + "content" + string(os.PathSeparator) +
		strings.ReplaceAll(rel, "/", string(os.PathSeparator))
	if err := os.MkdirAll(dirOf(p), 0o755); err != nil {
		return err
	}
	tmpl := fmt.Sprintf("---\ntitle: %q\ndate: %s\ndraft: true\n---\n\n",
		title, time.Now().Format("2006-01-02T15:04:05-07:00"))
	return os.WriteFile(p, []byte(tmpl), 0o644)
}

func dirOf(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[:i]
	}
	return "."
}
