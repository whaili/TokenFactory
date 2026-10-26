# TokenFactory

> Token 生产技术栈的**调研、学习、实践与输出**仓库。
>
> 目标：把「Token 怎么生产出来的」这件事，从协议规范、密码学基础、主流开源实现，到自建签发/校验系统，完整走通一遍，留下可复现的笔记、可运行的代码、可发布的文章。

---

## 这个仓库解决什么问题

市面上的 token 方案教程大多停在「怎么用 `golang-jwt` 签一个 JWT」。但真正在生产环境里做 Token 生产，需要回答的是一整条链路的问题：

| 阶段 | 核心问题 |
|---|---|
| **协议选型** | JWT / PASETO / Macaroon / Opaque Token / SPIFFE？各自的信任边界和攻击面在哪？ |
| **密码学基座** | 签名算法选什么？密钥怎么管？轮换怎么做？RS256/ES256/EdDSA/HS256 的取舍？ |
| **Claim 设计** | 授权粒度、过期策略、撤销能力、audience 隔离 —— 声明了什么决定了后面能做什么 |
| **签发系统** | 密钥生命周期、注册中心、租户隔离、审计、限流 |
| **校验系统** | 本地验签 vs 远程 introspect？JWKS 缓存与轮换？失败模式分类 |
| **攻击面** | 算法混淆、kid 注入、claim 注入、JWKS 劫持、令牌混淆 |
| **可观测性** | 怎么知道 token 在被怎么用？发证速率、验签失败率、异常调用模式 |
| **合规** | 审计留痕、数据最小化、密钥托管、轮换周期 |

这个仓库把上述每一条都拆成**笔记（docs/）+ 可运行实验（labs/）+ 对外文章（articles/）** 三层。

---

## 目录结构

```
TokenFactory/
├── README.md              # 你在这里：技术栈总览 + 学习路线图
├── docs/
│   ├── notes/             # 学习笔记：每个组件/主题一篇
│   │   ├── README.md      #   笔记索引 + 阅读顺序建议
│   │   └── 00-*.md ...
│   └── reference/         # 规范与协议参考（RFC / 规范摘要 / 攻击面清单）
├── articles/              # 对外技术文章（从笔记提炼而成）
├── labs/                  # 可运行的演示与基准测试
│   ├── 01-jwt-paseto-opaque/   # 三种 token 形态的实现对比 + 基准
│   └── 02-macaroon/            # Macaroon 的委派模型实验
├── third_party/           # 通过 git subtree 引入的开源项目源码（供阅读/改造）
├── tools/                 # 辅助脚本（subtree 同步、笔记索引生成等）
└── .github/workflows/     # CI
```

### 三层产出的分工

| 目录 | 面向 | 写法要求 |
|---|---|---|
| `docs/notes/` | 自己 | 可以粗糙、可以留 TODO，但**必须能独立看懂**；要有源码引用 |
| `labs/` | 自己 + 评审者 | **必须能跑**（`go test ./...` / `make test` 要绿）；要有可复现的输出 |
| `articles/` | 外部读者 | **必须自洽**；不能引用"上一篇笔记说过"；要有读者能独立跟上的上下文 |

**核心约束**：`labs/` 里的代码是 `docs/notes/` 的**可执行证据**。笔记里写"JWT 的签名验证比查库快 N 倍"，那 `labs/` 里必须有能复现这个数字的 benchmark。

---

## 技术栈总览

### 协议层

| 方案 | 形态 | 特点 | 调研状态 |
|---|---|---|---|
| **JWT** (RFC 7519) | 自包含、有签名 | 生态最广；算法混淆是经典攻击面 | 🔍 深入 |
| **PASETO** | 自包含、有加密 | 设计上消除算法混淆；v4.local vs v4.public | 🔍 深入 |
| **Macaroon** | 委派令牌 | 携带 caveat 逐级收窄权限；bearer 假设的替代品 | 🔍 深入 |
| **Opaque Token** | 无状态引用 | 服务端存状态；天然可撤销、可即时封禁 | 🔍 深入 |
| **SPIFFE / SVID** | 工作负载身份 | 面向服务网格的 mTLS 身份，非终端用户场景 | ⏳ 待调研 |
| **DID / Verifiable Credentials** | 去中心化身份 | 自主权身份；签发方不可控是核心差异 | ⏳ 待调研 |

### 密码学与密钥

| 主题 | 关键点 |
|---|---|
| 签名算法 | RS256 / RS512 / ES256 / ES384 / EdDSA / HS256 —— 性能、安全模型、密钥长度差异 |
| 密钥管理 | KMS / Vault / 云 KMS；软硬分离；信封加密；HS256 的"共享密钥悖论" |
| 轮换 | JWKS 多 key 并存、kid 路由、灰度切换、紧急吊销（`jku` / `x5c` 风险） |
| 熵与随机 | CSPRNG 正确用法、UUIDv4 vs ULID vs KID 的碰撞与可枚举性 |

### 开源实现（`third_party/` + 依赖）

| 项目 | Stars | 语言 | 定位 | 引入方式 |
|---|---|---|---|---|
| [golang-jwt/jwt](https://github.com/golang-jwt/jwt) | 9.2k | Go | JWT 最主流实现 | subtree（示例） |
| [lestrrat-go/jwx](https://github.com/lestrrat-go/jwx) | 2.4k | Go | 功能更全的 JWT/JWK/JWS 库 | subtree |
| [ory/fosite](https://github.com/ory/fosite) | 2.6k | Go | OAuth2 授权服务器框架（token 端点实现参考） | subtree |
| [ory/hydra](https://github.com/ory/hydra) | 17.6k | Go | OAuth2 / OIDC 授权服务器 | 阅读为主 |
| [go-macaroon/macaroon](https://github.com/go-macaroon/macaroon) | 194 | Go | Macaroon 实现 | subtree |
| [hashicorp/vault](https://github.com/hashicorp/vault) / [openbao/openbao](https://github.com/openbao/openbao) | 36k / 8.7k | Go | 密钥托管与 PKI | 阅读为主 |
| [smallstep/certificates](https://github.com/smallstep/certificates) | 9.0k | Go | 证书自动化（ACME），与工作负载身份相关 | 阅读为主 |
| [zitadel/zitadel](https://github.com/zitadel/zitadel) | 15.4k | Go | 完整 IAM / OIDC provider | 阅读为主 |
| [MicahParks/keyfunc](https://github.com/MicahParks/keyfunc) | 416 | Go | JWKS 缓存与轮换处理 | subtree |

> Stars / 语言数据随时间变化，以 `third_party/README.md` 中记录的时间点为准。

### 引入策略：为什么用 git subtree

本仓库对「需要读源码 / 需要改造」的开源项目使用 **git subtree** 引入到 `third_party/`：

```bash
# 引入
git subtree add --prefix=third_party/golang-jwt-jwt https://github.com/golang-jwt/jwt.git v5 --squash

# 同步上游更新
git subtree pull --prefix=third_party/golang-jwt-jwt https://github.com/golang-jwt/jwt.git v5 --squash
```

**为什么不用 submodule**：

| | submodule | subtree（本仓库的选择） |
|---|---|---|
| `git clone` | 需要 `--recursive`，否则目录为空 | **直接拿到全部内容** |
| GitHub 网页浏览 | 目录显示为灰色链接，点进去是上游仓库 | 正常显示、可点开、可搜索 |
| CI | 漏 `--recursive` 直接构建失败 | **无需任何额外配置** |
| 提交历史 | 你的 commit 引用上游 SHA，上游删 tag 会断 | `--squash` 后历史独立 |
| 上游同步 | `git submodule update --remote` | `git subtree pull` |

**subtree 的代价**：仓库体积更大（`--squash` 已压缩为单 commit），且上游更新需要手动 pull。这是可接受的代价 —— 本仓库的目标是**长期可阅读**，而不是永远跟得上上游。

> **命名说明**：`third_party/` 而不是 `vendor/` —— 因为 Go 工具链把 `vendor/` 保留给
> `go mod vendor` 管理的依赖目录。两者语义不同，不能混用。

**什么该进 `third_party/`，什么不该**：

- ✅ 进 `third_party/`：需要**读源码**、需要**改造**、需要**跑起来做实验**的项目
- ❌ 不进 `third_party/`：只是想**调用**的库 —— 直接走 `go.mod` 依赖，版本由 `go.sum` 锁定

---

## 学习路线图

按依赖顺序推进，每个阶段产出笔记 + 实验。

### 阶段 0 · 基础（Token 是什么）

- [ ] 文档笔记：[`docs/notes/01-token-fundamentals.md`](docs/notes/01-token-fundamentals.md) — 什么是 token，四种形态的边界
- [ ] 参考：[`docs/reference/glossary.md`](docs/reference/glossary.md) — 术语表

### 阶段 1 · 协议深入

- [ ] 文档笔记：`02-jwt-rfc7519.md` — header/claim/signature 三段结构，JWS vs JWE
- [ ] 文档笔记：`03-paseto.md` — 为什么 PASETO 认为 JWT 的设计有错
- [ ] 文档笔记：`04-macaroon.md` — 委派与 caveat 机制
- [ ] 文档笔记：`05-opaque-token.md` — 状态与撤销的权衡
- [ ] 参考：[`docs/reference/rfc-index.md`](docs/reference/rfc-index.md) — 相关 RFC 索引

### 阶段 2 · 密码学基座

- [ ] 文档笔记：`06-signing-algorithms.md` — 各算法实测性能与安全模型
- [ ] 文档笔记：`07-key-management.md` — 密钥生命周期、轮换、吊销
- [ ] 文档笔记：`08-attack-surface.md` — 算法混淆 / kid 注入 / JWKS 劫持等
- [ ] 实验：`labs/01-jwt-paseto-opaque` — 三形态实现 + 基准测试

### 阶段 3 · 签发与校验系统

- [ ] 文档笔记：`09-issuer-design.md` — 签发服务的分层设计
- [ ] 文档笔记：`10-validator-design.md` — 本地验签 vs 远程 introspect
- [ ] 文档笔记：`11-jwks-rotation.md` — JWKS 缓存、轮换、多 key 并存
- [ ] 文档笔记：`12-multi-tenancy.md` — 租户隔离、audience、scope

### 阶段 4 · 可观测性与合规

- [ ] 文档笔记：`13-observability.md` — 发证/验签指标、异常检测
- [ ] 文档笔记：`14-audit-compliance.md` — 审计留痕、数据最小化

### 阶段 5 · 输出

- [ ] 文章：[`articles/`](articles/) — 从笔记提炼

---

## 快速开始

```bash
# 拉取（含 subtree 内容，无需 --recursive）
git clone https://github.com/whaili/TokenFactory.git
cd TokenFactory

# 跑全部实验
make test

# 跑基准测试
make bench
```

## 本地开发

```bash
# Go 环境
export PATH=$PATH:/usr/local/go/bin

# 同步 third_party/ 下的上游（见 third_party/README.md）
make vendor-sync
```

## 贡献

笔记和实验代码都欢迎补充。新增笔记请遵循 `docs/notes/README.md` 里的模板约定。

## License

本仓库的**原创内容**（笔记、实验代码、文章）采用 [MIT License](LICENSE)。

`third_party/` 下的第三方源码保留各自原有的许可证，详见各目录下的 `LICENSE` 文件与 `third_party/README.md` 的记录。
