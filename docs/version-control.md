# Git 与远端仓库

## 仓库信息

- 默认分支：`main`
- 远端名称：`origin`
- 远端地址：`https://github.com/xshrim/opskeeper.git`

检查当前配置：

```bash
git branch --show-current
git remote -v
```

## 正常推送

```bash
git push origin main
```

首次推送或本地分支尚未设置上游时：

```bash
git push --set-upstream origin main
```

## 临时代理

无法直接连接 GitHub 时，只对当前命令临时设置本机代理：

```bash
http_proxy=http://127.0.0.1:7890 \
https_proxy=http://127.0.0.1:7890 \
git push origin main
```

读取远端或拉取代码时可以使用相同方式：

```bash
http_proxy=http://127.0.0.1:7890 \
https_proxy=http://127.0.0.1:7890 \
git fetch origin
```

代理变量仅作用于对应命令，不写入仓库或用户级 Git 配置。不要使用 `git config --global http.proxy`，避免影响其他项目和无代理网络环境。

## 提交前检查

```bash
make quality
git status --short
git diff --check
```

禁止提交 `.env`、真实凭据、`node_modules`、前端构建产物及本地缓存。相关规则维护在仓库根目录的 `.gitignore` 中。
