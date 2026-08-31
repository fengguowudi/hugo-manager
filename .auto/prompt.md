# Autoresearch: FixIt 主题适配（hugo-manager）

## Objective

让 hugo-manager（Hugo 本地 GUI 管理器）对 FixIt 主题（https://github.com/hugo-fixit/FixIt，已克隆在
`.auto/themes/FixIt`）做专门适配：识别 FixIt 站点、结构化编辑 FixIt 专有 front matter 字段、
新建文章套用 FixIt 原型字段，并保证管理器的写回在真实 FixIt 构建下不破坏站点。

基准站点：`.auto/fixtures/site/`（theme = "FixIt"，3 篇含 FixIt 专有 front matter 的文章）。

## Metrics

- **Primary**: `fixit_score`（无单位，越高越好，满分 20）— `core/fixit_score_test.go` 中
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

- （基线前）架构调整：逻辑拆到 `core/` 包（无 CGO），UI 留主包 —— 因本机无 gcc 无法编译 Fyne。
- 基线待测。预期失分项：site.detectTheme、save.subtitle/featuredImage/weightBareNumber/
  bareBool/password/collections、schema.fixit、fallback.fixitFields。
