# Ideas backlog（FixIt 适配）

- [高优先 bug] 多语言 UI 新建后打开路径错误：NewContent 已写入 DefaultContentDir，但 native_ui.newPost 仍固定 openPost("content/"+rel)；content/zh-cn 站点会新建成功后立即报文件不存在。下一轮应先修并做 UI 行为检查。
- [候选] 友链添加 UI：core.AddFriend / ValidateFriend 已完成，概览卡片已可见；只有明确需要时再加 dialog 表单。
- [需独立度量] content/ 扫描性能：当前 ListPosts 会读取每篇文章完整内容；如切换为性能目标，应重新 init_experiment，以扫描耗时/内存为主指标，不把阈值硬塞进功能分。
- [已知边界] 多语言配置支持站点根目录的 TOML/YAML；尚未合并 config/ 目录拆分配置和环境配置层，收益不足时不要扩面。
- [低优先] expiryDate 到期前预警；当前已覆盖已过期/定时未到的确定状态。
