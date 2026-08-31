# Ideas backlog（FixIt 适配）

- [暂缓] ThemeSchema 增加 FixIt 页面级开关字段：math / toc / comment / lightgallery（theme.Params 均支持）。
  价值=编辑器多出几个开关；检查方式只能是"schema 含键"（弱检查），需要时直接往 fixit.go 的 FixIt 分支加。
- [已知限制] SavePost 语义"空值=不动该字段"，UI 无法清空已有标量字段（防误删 title）。
