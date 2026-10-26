# 学习笔记

这里是**给自己看**的笔记。可以粗糙、可以有 TODO、可以写得很快 —— 但有两个底线要求：

1. **能独立看懂**。半年后回来读，不依赖当时的记忆。
2. **有证据**。结论性的说法要么引用源码（`file.go:func()`），要么引用 `labs/` 里的实测输出。

---

## 笔记索引

| # | 主题 | 状态 | 关键产出 |
|---|---|---|---|
| 00 | [术语表与心智模型](../reference/glossary.md) | ✅ | 四种 token 形态的边界 |
| 01 | [Token 基础：四种形态对比](01-token-fundamentals.md) | ✅ | 选型决策树 |
| 02 | JWT / RFC 7519 深入 | ⏳ | — |
| 03 | PASETO | ⏳ | — |
| 04 | Macaroon | ⏳ | — |
| 05 | Opaque Token | ⏳ | — |
| 06 | 签名算法性能与安全 | ⏳ | — |
| 07 | 密钥管理与轮换 | ⏳ | — |
| 08 | 攻击面 | ⏳ | — |
| 09 | 签发系统设计 | ⏳ | — |
| 10 | 校验系统设计 | ⏳ | — |
| 11 | JWKS 轮换 | ⏳ | — |
| 12 | 多租户隔离 | ⏳ | — |
| 13 | 可观测性 | ⏳ | — |
| 14 | 审计与合规 | ⏳ | — |

状态图例：✅ 已完成 · 🔄 进行中 · ⏳ 待开始

---

## 笔记模板

新增笔记请复制下面的模板。

```markdown
# NN. <主题>

> **状态**：🔄 进行中 | ✅ 完成
> **最后更新**：YYYY-MM-DD
> **相关实验**：[`labs/...`](../labs/...)

## 一句话结论

<用一句话说清这篇笔记要让人记住什么>

## 问题背景

<为什么需要研究这个？什么场景会触发这个问题？>

## 核心概念

<术语定义。链接到 ../reference/glossary.md 而不是重复定义>

## 机制拆解

<原理。要有图或流程步骤。关键处引用源码位置>

## 关键决策与取舍

<表格：选项 A / B / C，各自已知的优点、缺点、适用条件>

## 实测数据

<来自 labs/ 的真实输出，注明怎么复现>

## 陷阱与坑

<实际踩到的。写清楚"怎么发现的"和"怎么避免">

## 引用

- RFC 7519 — https://datatracker.ietf.org/doc/html/rfc7519
- `vendor/golang-jwt-jwt/parser.go:ParseWithClaims` — 源码位置
```

---

## 写作约定

**源码引用格式**：`路径:函数名` 或 `路径:行号`

- ✅ `vendor/golang-jwt-jwt/parser.go:ParseWithClaims`
- ✅ `labs/01-jwt-paseto-opaque/bench_test.go:TestBenchmarkSign`
- ❌ 「在 golang-jwt 里有个解析函数」

**实测数据格式**：贴真实输出，不要手写近似值

```go
func BenchmarkSignES256(b *testing.B) { ... }
```

```
$ go test -bench=. -benchmem ./...
BenchmarkSignES256-8   	 12043	    99714 ns/op	 34512 B/op	    41 allocs/op
```

**不确定的结论**：明确标注「待验证」并写下验证方法，不要把猜测写成事实。

**图表**：优先用 Mermaid（GitHub 原生支持渲染）。
