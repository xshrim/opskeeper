# Git 与远端仓库

## 仓库信息

- 默认分支：`main`
- 远端名称：`origin`
- GitHub 仓库：`https://github.com/xshrim/opskeeper.git`
- 当前 SSH 远端：`ssh://git@ssh.github.com:443/xshrim/opskeeper.git`

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

当前网络不能直接连接 GitHub SSH 端口。已经在 GitHub 添加本机 `~/.ssh/id_rsa.pub` 时，可以让 SSH 通过本机 HTTP 代理连接 GitHub 官方 SSH 443 入口：

```bash
GIT_SSH_COMMAND="ssh -o IdentitiesOnly=yes -i $HOME/.ssh/id_rsa \
  -o 'ProxyCommand=nc -X connect -x 127.0.0.1:7890 %h %p'" \
git push origin main
```

`GIT_SSH_COMMAND` 仅对当前命令生效，不修改用户级 SSH 配置。首次连接时应核对并接受 `ssh.github.com` 的官方主机指纹。

## 临时代理

使用 HTTPS 远端且无法直接连接 GitHub 时，只对当前命令临时设置本机代理：

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
