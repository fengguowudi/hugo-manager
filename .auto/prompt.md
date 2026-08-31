# Autoresearch: FixIt 主题适配（hugo-manager）

## Objective

让 hugo-manager（Hugo 本地 GUI 管理器）对 FixIt 主题（https://github.com/hugo-fixit/FixIt，已克隆在
`.auto/themes/FixIt`）做专门适配：识别 FixIt 站点、结构化编辑 FixIt 专有 front matter 字段、
新建文章套用 FixIt 原型字段，并保证管理器的写回在真实 FixIt 构建下不破坏站点。

基准站点：`.auto/fixtures/site/`（theme = "FixIt"，3 篇含 FixIt 专有 front matter 的文章）。

## Metrics

- **Primary**: `fixit_score`（无单位，越高越好，满分 31）— `core/fixit_score_test.go` 中
  TestFixItScore 的行为级检查通过数。检查项见测试源码注释：site.title / site.baseURL /
  site.detectTheme / parse.* / save.* / schema.fixit / fallback.fixitFields / e2e.*。
- **Secondary**: `fixit_total`（恒为 20，仅校验口径不变）。

## How to Run

`bash ./.auto/measure.sh` — 输出 `METRIC fixit_score=N`。
调试失败项：`go test -count=1 -v -run TestFixItScore ./core`（看 `MISS <name>` 日志）。

## 环境（已备好，勿重复下载）

- `.auto/bin/hugo.exe` — Hugo v0.165.0 extended（fixture 构建用；系统 PATH 上的 hugo 0.127 太老）。
- `.auto/bin/dart-sass/` — Dart Sass 1.103.1（FixIt 构建需要；测试里自动加 PATH）。
  注意：Hugo >=0.139 只认名为 `dart-sass`/`sass` 的二进制，`dart-sass-embedded` 已弃用。
- `.auto/themes/FixIt` — FixIt 主题源码（深度克隆，已去 .git）。**只读参考，勿修改**。

## Files in Scope

- `core/*.go` — 主要工作面：content.go（ParsePost/SavePost/ListPosts/SiteInfo）、
  fixit.go（DetectTheme/ThemeSchema，目前是桩）、hugorun.go（NewContent/CreateFallbackPost）、app.go。
- `native_ui.go` — 可编辑以接入 ThemeSchema（如编辑器按主题出字段、概览显示主题名），
  但**本机无 gcc，主包无法编译验证**（CGO_ENABLED=0 + Fyne）。改动保持简单机械，
  checks.sh 只做 gofmt 语法检查。
- `core/fixit_score_test.go` 与 `.auto/fixtures/` — **基准本身，禁止修改**（防过拟合）。

## Off Limits

- 不改基准测试、fixture 站点、`.auto/themes/FixIt` 主题源码。
- 不在产品代码里写死 fixture 路径或检查名 —— 必须走真实代码路径。
- 不新增第三方依赖。

## Constraints

- `go vet ./core`、`go build ./core`、`go test ./core` 必须通过（checks.sh）。
- SavePost 的"外科手术式"写回原则不变：未编辑字段逐字节保留，不破坏手工 front matter。
- YAML/TOML 两种 front matter 都要兼容（fixture 用 YAML；别把 TOML 路径改坏）。

## FixIt 页面级 front matter 参考（来自主题源码与原型 archetypes/posts.md）

snake_case 风格：`subtitle`、`weight`（数字，置顶排序）、`featured_image`、
`featured_image_preview`、`hidden_from_home_page`（布尔）、`hidden_from_related`、
`hidden_from_feed`、`password`、`message`、`keywords`（数组）、`collections`（数组）、
`repost`（嵌套 map: enable/url）、`summary`、`layout`、`comment`；
Hugo 内建 camelCase：`lastmod`、`expiryDate`、`publishDate`。
块式 YAML 列表（`tags:\n  - a`）很常见，写回时必须原样保留未编辑的块列表。

## What's Been Tried

基线 10/18 → 最终 31/31（基准逐步扩展到 31，每次扩展前先手动验证新检查 MISS 再实现）。

已实现（keep）：
- SavePost 泛化：任意标量/数组字段写回；布尔/数字/日期裸写；setLine/delLine 块感知
  （修复了真实 bug：编辑块式 tags 列表会残留 `- item` 行使 front matter 非法）
- DetectTheme：配置 theme 键 / go.mod 模块（hugo-fixit/FixIt）/ themes 目录 theme.toml
- ThemeSchema：通用 7 字段 + FixIt 11 专有字段（subtitle/weight/featured_image 等 snake_case）
- CreateFallbackPost：FixIt 站点套用原型字段布局
- BuildState 输出 theme；概览页显示主题名
- UI 编辑器：默认字段 + 按 ThemeSchema 动态生成的额外表单（bool→Check，其余→Entry）

原生通过（作为回归守卫加入）：TOML front matter 全路径、hugo new 自动匹配 posts 原型、CRLF/BOM 兼容。

环境备忘：
- pi runner 的 bash 曾解析到 WSL stub → 已将 C:\Windows\System32\bash.exe 改名 bash-wsl-stub.exe（可逆）
- measure.sh/checks.sh 显式 export /d/Git/usr/bin 与 scoop mingw 路径
- scoop 安装 mingw 后主包（Fyne/CGO）可编译验证；无 gcc 时 checks.sh 降级为 gofmt 语法检查
- Hugo >=0.139 弃用 dart-sass-embedded 二进制名，只认 dart-sass/sass（dart-sass 1.103.1 在 .auto/bin/）

已达基准上限 31/31，所有 FixIt 适配面经真实主题端到端构建验证。
