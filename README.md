# TokenFactory

> Token 生产技术栈的调研、学习、实践与输出。

---

## 这个仓库做什么

把「Token 怎么生产出来的」完整走一遍：协议规范、密码学基础、主流开源实现、自建签发/校验系统，留下**可复现的笔记、可运行的实验、可发布的文章**。

不限定语言与技术栈 —— 调研涉及什么就用什么。

## 目录结构

```
TokenFactory/
├── docs/
│   ├── notes/        # 学习笔记：每个主题一篇
│   └── reference/    # 规范、术语表、攻击面清单等参考材料
├── articles/         # 对外技术文章（从笔记提炼）
├── labs/             # 可运行的演示项目与基准测试
├── third_party/      # 通过 git subtree 引入的开源项目源码
├── tools/            # 辅助脚本
└── .github/workflows # CI
```

### 三层产出的分工

| 目录 | 面向 | 要求 |
|---|---|---|
| `docs/notes/` | 自己 | 能独立看懂，有源码/实测证据 |
| `labs/` | 自己 + 评审者 | 必须能跑，结果可复现 |
| `articles/` | 外部读者 | 必须自洽，能独立跟上 |

**核心约束**：`labs/` 是 `docs/notes/` 的**可执行证据**。笔记里写"X 比 Y 快 N 倍"，`labs/` 里就要有能复现这个数字的基准测试。

## 第三方源码的引入策略

对需要读源码 / 改造的开源项目，使用 **git subtree** 引入到 `third_party/`：

```bash
# 引入
git subtree add --prefix=third_party/<name> <upstream-url> <ref> --squash

# 同步上游
git subtree pull --prefix=third_party/<name> <upstream-url> <ref> --squash
```

**为什么不用 submodule**：

| | submodule | subtree |
|---|---|---|
| `git clone` | 需 `--recursive`，否则目录为空 | 直接拿到全部内容 |
| GitHub 网页 | 灰色链接，点不进上游内容 | 正常显示、可搜索 |
| CI | 漏 `--recursive` 直接失败 | 无需额外配置 |
| 上游同步 | `git submodule update --remote` | `git subtree pull` |

**代价**：仓库体积更大，上游更新需手动 pull。对"长期可阅读"的仓库可接受。

**什么该进 `third_party/`**：需要读源码、需要改造、需要跑起来做实验的项目。
**什么不该进**：只是想调用的库 —— 走各语言的包管理器，版本由 lock 文件锁定。

> 命名用 `third_party/` 而非 `vendor/`，是因为 Go 等工具链把 `vendor/` 保留给依赖 vendoring，两者语义不同。

## 学习路线图

| 阶段 | 主题 |
|---|---|
| 0 · 基础 | Token 是什么；四种形态（JWT / PASETO / Macaroon / Opaque）的边界 |
| 1 · 协议 | JWT (RFC 7519)、PASETO、Macaroon、Opaque Token |
| 2 · 密码学 | 签名算法取舍、密钥管理与轮换、攻击面 |
| 3 · 系统 | 签发服务设计、校验服务设计、JWKS 轮换、多租户隔离 |
| 4 · 运维 | 可观测性、审计与合规 |
| 5 · 输出 | 从笔记提炼对外文章 |

## License

原创内容（笔记、实验代码、文章）采用 [MIT License](LICENSE)。

`third_party/` 下的第三方源码保留各自原有许可证。
