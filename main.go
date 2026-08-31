// hugo-manager —— Hugo 博客本地可视化管理器（独立客户端）。
// 不修改 Hugo 本体，通过调用已安装的 hugo 二进制工作：
//
//	hugo new（建文章，套用原型）、hugo server -D（预览）、hugo（构建）、hugo version。
//
// 仅监听本机回环地址、无鉴权 —— 只限本机个人使用，请勿暴露到网络。
package main

import (
	"hugo-manager/core"
)

// Native desktop app; no local HTTP listener is required.

func main() {
	a := core.NewApp(core.ConfigPath())
	a.Probe()
	runNativeApp(a)
}
