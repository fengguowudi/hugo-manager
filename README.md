# Hugo 管理器（hugo-manager）

给**已安装的 hugo 二进制**用的本地可视化管理客户端：不改 Hugo 源码，
通过 `hugo new` / `hugo server` / `hugo` / `hugo version` 命令驱动一个 Hugo 博客。
界面使用 Go 原生跨平台 GUI（Fyne），不依赖浏览器、Chrome、Edge 或 WebView。后端和界面编译进同一个可执行文件；运行时只调用用户已安装的 Hugo 二进制。

## 快速开始

```sh
go build -ldflags "-H=windowsgui" -o hugo-manager.exe .  # Windows，不显示控制台窗口
go build -o hugo-manager .                              # Linux
go build -o hugo-manager .                              # macOS（建议在 macOS 上构建）
```

## 必填配置（首次启动在「设置」页完成）

| 配置 | 说明 | 默认 |
|---|---|---|
| hugo 路径 `*` | hugo 可执行文件完整路径；**留空 = 自动从 PATH 检测**（点「检测」可验证） | 自动检测 |
| 站点目录 `*` | 博客根目录，即包含 `hugo.toml` / `config.toml` 的目录 | — |
| 预览端口 | `hugo server --port` | 1313 |
| 含草稿构建 | 构建 (`hugo -D`) 时是否包含 `draft: true` 的文章 | 关 |

配置持久化在管理器可执行文件同目录的 `config.json`。应用本身不监听 HTTP 端口。

## 功能

- **概览**：站点标题 / baseURL / Hugo 版本 / 文章与草稿统计 / 快捷操作
- **内容**：扫描 `content/`，按日期列出文章并标记草稿；选择文章后直接编辑常用 front matter 字段（标题、日期、slug、草稿、标签、分类、摘要）和 Markdown 正文；未知 front matter 字段原样保留
- **快捷创建 Markdown**：填写标题和可选板块即可创建；优先调用 `hugo new` 套用原型，Hugo 不可用时写入最小模板
- **构建与预览**：启停 `hugo server -D`，执行 `hugo` 构建到 `public/`，在应用内查看进程日志
- **设置**：配置 Hugo 路径、站点目录、预览端口和是否构建草稿

## 桌面平台

- Windows、macOS、Linux 使用同一套 Fyne 原生桌面界面
- 每个平台分别构建自己的单文件程序；不依赖浏览器或 WebView
- Windows 构建需要 GCC/MinGW，macOS 需要 Xcode Command Line Tools，Linux 需要 GCC 和 OpenGL/X11 开发包

## 边界（v1 明确不做 / 简化）

- 不做主题管理、媒体库上传、多语言专用视图
- front matter 仅支持 YAML(`---`) 与 TOML(`+++`) 的常用字段结构化编辑；JSON front matter 只读展示
- 标签/分类以行内数组写回（块式列表会被转为行内）
- 编辑动作限定在站点目录内（服务端做路径穿越校验）

## FixIt 主题适配

识别到站点使用 [FixIt](https://github.com/hugo-fixit/FixIt) 主题时（hugo.toml 的 `theme`、go.mod 模块或 themes/ 目录）：

- **概览**页显示主题名
- **编辑器**自动追加 FixIt 专有 front matter 字段：副标题、置顶权重、头图、列表预览图、合集、关键词、小结、首页隐藏、相关文章隐藏、访问密码、密码提示语
- **新建文章**的兜底模板（无 hugo 二进制时）套用 FixIt 原型字段
- 写回保持外科手术式：未编辑字段（含 `repost` 嵌套表、块式列表）逐字节保留；布尔/数字/日期裸写，CRLF/BOM 保持
