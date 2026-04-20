# Disk Space Incident Skill

`disk-space-incident` 是一个面向“磁盘空间不足/即将占满”场景的官方 skill 包。

目标：

- 先查时序监控，再决定是否需要上机
- 优先给出趋势、预计占满时间和增长来源判断
- 只有在监控和知识证据不足时，才进入主机侧排查
- 高风险清理动作仍然走审批

这个 skill 包当前以 `tars.marketplace/v1alpha1` 包格式保存，适合后续通过 `skill_source` / marketplace 导入。

对应包定义见：

- [package.yaml](package.yaml)
