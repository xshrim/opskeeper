# Git 与远端仓库

本文档规定 OpsKeeper 的 Git 分支、提交、验收、合并和发布方式。项目采用**轻量级主干开发**：只有 `main` 是长期分支，具体任务使用短生命周期分支，完成开发和验收后才合并回 `main`。

## 1. 仓库信息

- 默认分支：`main`
- 远端名称：`origin`
- GitHub 仓库：`https://github.com/xshrim/opskeeper.git`
- 当前 SSH 远端：`ssh://git@ssh.github.com:443/xshrim/opskeeper.git`

检查当前分支和远端：

```bash
git branch --show-current
git remote -v
```

`main` 应始终保持可构建、可测试和可部署，不在 `main` 上直接开展尚未验收的任务开发。

## 2. 分支模型

| 分支 | 生命周期 | 用途 |
|---|---:|---|
| `main` | 长期 | 唯一主干，保存已通过检查和验收的代码 |
| `feat/<task>-<name>` | 短期 | 功能或阶段任务，例如 `feat/t02-organization` |
| `fix/<name>` | 短期 | 普通缺陷修复 |
| `docs/<name>` | 短期 | 纯文档修改 |
| `refactor/<name>` | 短期 | 不改变外部行为的重构 |
| `chore/<name>` | 短期 | 依赖、构建、Compose、CI 等维护工作 |
| `hotfix/<version>-<name>` | 短期 | 已发布版本的紧急修复 |
| `release/<version>` | 必要时才使用 | 版本稳定期间与下一版本并行开发 |

不设置长期 `develop` 分支。`release/*` 只在确实需要并行稳定一个版本时创建，不能作为日常开发分支。

任务编号是实施和验收标识，不是长期分支层级。一个纵向任务可以同时包含后端、前端、数据库迁移、测试和文档；不因为文件类型不同就拆成多个长期分支。

## 3. 任务分支生命周期

### 3.1 创建分支

从最新 `main` 创建任务分支：

```bash
git switch main
git pull --ff-only
git switch -c feat/t03-scope-rbac
```

当前仓库的 T02 变更使用：

```text
feat/t02-organization
```

任务尚未验收前，代码、测试和文档修改都留在该分支，不直接写入 `main`。

### 3.2 开发和提交

按可解释的逻辑单元提交，提交信息使用 Conventional Commits：

```text
feat(organization): add team and project APIs
fix(migrations): make rollback transactional
docs(development): reorganize local setup guide
test(organization): cover concurrent team creation
```

推荐的提交类型为 `feat`、`fix`、`docs`、`test`、`refactor`、`perf`、`chore`、`ci` 和 `revert`。作用域使用稳定模块名，例如 `organization`、`authorization`、`migrations`、`http`、`frontend`、`compose` 和 `docs`。

不要使用无法说明行为的提交信息，例如 `update`、`fix bug` 或 `T02 changes`。

### 3.3 推送和 Pull Request

任务分支首次推送：

```bash
git push --set-upstream origin feat/t03-scope-rbac
```

开发过程中需要同步主干时：

```bash
git fetch origin
git rebase origin/main
```

任务分支通过 Pull Request 合并，不直接执行 `git push origin main`。默认使用 Squash merge，使一个已验收任务在 `main` 上形成一个清晰的交付提交；分支提交已经整理且各提交具有独立价值时，可以使用 Rebase merge。

### 3.4 验收和合并门禁

任务状态按以下顺序推进：

```text
实施中 -> 待验收 -> 用户验收通过 -> 合并 main -> 已完成
```

只有用户明确验收当前任务后，才允许合并到 `main`。验收前可以创建并更新 Pull Request，但必须保持未合并状态。

合并前至少满足：

1. 分支基于最新 `main`，没有未解决冲突。
2. `make quality` 通过。
3. 涉及 PostgreSQL 时，PostgreSQL 16 集成测试通过。
4. 涉及迁移时，验证 `up -> down -> up`、重复 `up` 幂等和并发迁移互斥，并说明 Expand/Contract 兼容性。
5. 涉及 HTTP API 时，覆盖路由、输入校验、错误映射和服务行为。
6. 涉及前端时，完成构建、测试和主要页面检查。
7. 配置或启动方式变化同步更新示例配置和开发文档。
8. Pull Request 描述包含变更范围、验证命令、数据库影响和回滚方式。

用户验收通过后，先在任务分支中将任务状态更新为 `已完成` 并记录验收结果，再完成最后一轮检查和合并。合并完成后删除任务分支：

```bash
git switch main
git pull --ff-only
git branch -d feat/t03-scope-rbac
```

## 4. `main` 保护规则

GitHub 上的 `main` 应启用以下保护规则：

- 禁止直接 push。
- 必须通过 Pull Request 合并。
- 必须通过 CI 和质量检查。
- 必须解决所有 review conversation。
- 禁止 force push 和删除 `main`。
- 优先保持线性历史，禁止无必要的 merge commit。
- 小团队至少需要一名批准者；单人开发可以由本人合并，但不能跳过 CI 和验收门禁。

## 5. 数据库迁移分支规则

数据库迁移必须遵守比普通代码更严格的历史规则：

- 每个迁移版本号在仓库中全局唯一。
- 前滚 SQL 和同版本 `.down.sql` 必须在同一个 Pull Request 中提交。
- 已经合并或部署的迁移文件不得修改，修复必须新增迁移。
- 并行分支新增迁移时，后合并者必须基于最新 `main` 重新检查版本号并执行迁移测试。
- 迁移由单一发布任务执行，API 副本不自动执行 Schema 变更。
- 滚动发布优先采用“先扩展 Schema、再部署代码、最后清理旧字段”的向后兼容顺序。
- 应用回滚不自动执行数据库 `down`；生产问题优先新增前滚修复迁移。
- 发布分支中的迁移二进制和应用必须由同一提交构建，并使用同一个不可变镜像 digest。

自动化 Migration Job、失败处理和权限边界见[数据库与应用自动化发布](delivery.md)。

例如，`0001_scope_organization` 进入 `main` 后，不能直接改写该文件来增加新表，应新增 `0002_...` 迁移。

## 6. 版本发布和 Hotfix

使用语义化版本标签：

```text
v0.1.0
v0.2.0
v0.2.1
v1.0.0
```

新阶段或新增能力通常提升次版本，兼容性缺陷修复提升修订版本，不兼容变更提升主版本。T01、T02 等是实施任务编号，不替代产品版本号。

从已经验收并合并的 `main` 创建发布标签：

```bash
git switch main
git pull --ff-only
git tag -a v0.2.0 -m "OpsKeeper v0.2.0"
git push origin v0.2.0
```

如果已发布版本需要紧急修复，而 `main` 已包含尚未发布的新功能，则从对应版本标签创建 Hotfix 分支：

```bash
git switch -c hotfix/v0.2.1-database-connection v0.2.0
```

Hotfix 通过 Pull Request 发布修订版本后，必须将同一修复同步回 `main`。不重复手工实现两份修复代码，优先 cherry-pick 已审核的修复提交。

## 7. SSH 远端和代理

远端使用 GitHub SSH over HTTPS 443：

```text
ssh://git@ssh.github.com:443/xshrim/opskeeper.git
```

SSH 使用本机密钥，并通过 HTTP 代理连接 `ssh.github.com:443`。建议将代理和密钥配置在用户级 `~/.ssh/config`，使拉取、推送和 PR 辅助命令统一生效，而不是为每次 Git 命令临时设置 `GIT_SSH_COMMAND`：

```sshconfig
Host ssh.github.com
    HostName ssh.github.com
    Port 443
    User git
    IdentityFile ~/.ssh/id_rsa
    IdentitiesOnly yes
    ProxyCommand ncat --proxy 127.0.0.1:7890 --proxy-type http %h %p
```

配置生效后检查：

```bash
git remote -v
ssh -T git@ssh.github.com
```

首次连接时应核对并接受 `ssh.github.com` 的官方主机指纹。不要使用 `git config --global http.proxy` 代替 SSH 代理，也不要把真实密钥、代理密码或个人配置提交到仓库。

## 8. 提交前检查

```bash
make quality
git status --short
git diff --check
```

禁止提交 `.env`、真实凭据、`node_modules`、前端构建产物和本地缓存。相关规则维护在仓库根目录的 `.gitignore` 中。
