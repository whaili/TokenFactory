# Vendor — 通过 git subtree 引入的开源项目

本目录下的代码**全部来自第三方开源项目**，通过 `git subtree` 引入，保留各自原始许可证。

> ⚠️ **这些代码是「学习材料」和「实验基座」**，不是本仓库的原创产品。
> 如果某个项目只是**被调用**而不需要读源码/改造，它**不应该**出现在这里 —— 应该走 `go.mod` 依赖。

## 引入的约定

| 约定 | 说明 |
|---|---|
| 目录名 | `<org>-<repo>`，小写，连字符分隔 |
| 引入方式 | `git subtree add --prefix=vendor/<name> <url> <ref> --squash` |
| 同步方式 | `git subtree pull --prefix=vendor/<name> <url> <ref> --squash` |
| 记录 | 每个目录在下方表格登记：来源、引入时间、引入时的 commit、许可证、**引入它的原因** |

**为什么用 `--squash`**：保留上游完整历史会让仓库体积膨胀数倍，而本仓库的目标是「长期可阅读 + 可做实验」，不需要追溯上游每一笔提交。同步时用 `git subtree pull --squash` 即可把上游变更压成单个 commit。

---

## 登记表

| 目录 | 上游 | 引入时间 | 引入时 commit | 许可证 | 引入原因 |
|---|---|---|---|---|---|
| `golang-jwt-jwt` | https://github.com/golang-jwt/jwt | 2026-10-26 | _(待填)_ | MIT | JWT 最主流 Go 实现；作为算法性能实测与 RS256/ES256/EdDSA/HS256 对比的基线 |

---

## 待引入清单（调研计划）

按学习路线图推进，逐步引入：

- [ ] `lestrrat-go-jwx` — 功能更全的 JWT/JWK/JWS 库，用于对比 `golang-jwt` 的 API 设计取舍
- [ ] `go-macaroon-macaroon` — Macaroon 的 Go 实现，用于委派模型实验（`labs/02-macaroon`）
- [ ] `ory-fosite` — OAuth2 授权服务器框架，用于研究 token 端点的完整实现（授权码、refresh、PKCE）
- [ ] `MicahParks-keyfunc` — JWKS 缓存与轮换处理，用于 JWKS 轮换实验
- [ ] `hashicorp-vault`（阅读为主）— 密钥托管参考实现
- [ ] `smallstep-certificates`（阅读为主）— ACME / 工作负载身份

---

## 同步脚本

```bash
# 查看所有 vendor 项目及其上游 URL（从 git config 读取）
make vendor-list

# 同步单个项目
make vendor-sync PROJECT=golang-jwt-jwt REF=v5

# 同步全部（见 tools/sync-vendor.sh）
make vendor-sync-all
```

脚本实现见 [`tools/sync-vendor.sh`](../tools/sync-vendor.sh)。
