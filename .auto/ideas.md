# Ideas backlog（FixIt 适配）

- [暂缓] ThemeSchema 增加 FixIt 页面级开关字段：math / toc / comment / lightgallery（theme.Params 均支持）。
  价值=编辑器多出几个开关；检查方式只能是"schema 含键"（弱检查），需要时直接往 fixit.go 的 FixIt 分支加。
- [已知限制] SavePost 语义"空值=不动该字段"，UI 无法清空已有标量字段（防误删 title）。
  若需要"清空即删除"，需显式协议（如 fields["__del__"]），改动 UI+core 两侧。
- [环境] 若重装 WSL，需恢复 C:\Windows\System32\bash-wsl-stub.exe → bash.exe。
- [跨平台] 基准目前 Windows 限定（hugo.exe、mklink /J、dart-sass windows）。移植其他平台需参数化。
