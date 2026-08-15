# OpsKeeper 文档索引

本文档是 OpsKeeper 设计、工程规范、操作指南、实施计划和验收记录的统一入口。文档按职责分类，每项信息只在一个权威位置维护；其他文档通过链接引用，不复制完整规则。

## 文档分类

| 目录          | 内容                                 | 使用场景                   |
| ------------- | ------------------------------------ | -------------------------- |
| `design/`     | 当前系统设计、领域模型和架构边界     | 理解系统是什么以及如何设计 |
| `standards/`  | 必须遵守的编码、Git 和工程约束       | 开发、重构和代码评审       |
| `guides/`     | 本地开发、测试、迁移和自动化发布步骤 | 执行具体工程操作           |
| `planning/`   | 阶段规划、任务拆分和审批状态         | 确认当前进度和后续范围     |
| `acceptance/` | 已完成阶段的验收步骤、结果和证据     | 复核已交付能力             |
| `adr/`        | 重要技术决策的背景、选项和结论       | 追溯为什么采用当前方案     |
| `runbooks/`   | 生产故障处置、恢复和巡检操作         | 处理运行事件               |

## 设计

- [总体架构设计](design/architecture.md)
- [三级权限与安全设计](design/authorization.md)
- [组织、资源与拓扑模型](design/resource-model.md)
- [Kubernetes 导入、AI 诊断与自动巡检](design/operations-workflows.md)

## 工程规范

- [Go 编码与工程组织通用规范](standards/go-coding-conventions.md)
- [Git 与远端仓库](standards/version-control.md)

## 操作指南

- [本地开发](guides/development.md)
- [自动化发布](guides/delivery.md)

## 计划与验收

- [分阶段实施任务书](planning/implementation-tasks.md)
- [T01 工程初始化与质量基线验收记录](acceptance/t01.md)
- [T02 Scope 与组织模型验收记录](acceptance/t02.md)

## 维护原则

1. 当前设计事实写入 `design/`，强制性工程规则写入 `standards/`。
2. 可执行步骤写入 `guides/`，未完成任务及状态写入 `planning/`。
3. 验收记录保存已经执行的步骤和结果，不用于描述后续计划。
4. 影响长期技术方向的重要决定写入编号 ADR；ADR 记录决策历史，不替代当前设计文档。
5. 生产事件的诊断和恢复步骤写入 `runbooks/`，不与自动化发布流程混写。
6. 移动或重命名文档时，必须同步更新本索引和仓库内的全部引用。
