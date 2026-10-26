# 01. Token 基础：四种形态对比

> **状态**：✅ 完成
> **最后更新**：2026-10-26
> **相关实验**：[`labs/01-jwt-paseto-opaque`](../labs/01-jwt-paseto-opaque)

## 一句话结论

Token 形态的选择，本质上是在**「验证成本」和「可控性」**之间做取舍 —— 自包含形态（JWT/PASETO）用验证方的一次本地计算换取零网络开销，引用形态（Opaque）用一次网络往返换取即时撤销能力；**没有第三种选项**。

## 问题背景

选型讨论里最常见的两个错误：

1. **把 JWT 当成"唯一正确答案"** —— 因为它是默认选项，不是因为它适合当前场景。
2. **只比"功能清单"** —— 列一串特性做对比表，不看**每项特性背后的成本**（一次网络往返、一次状态查询、一次密钥分发）。

这篇笔记的目的：把三种主流形态**跑起来、量出来**，让选型有数据支撑而不是观点输出。

## 核心概念

见 [`docs/reference/glossary.md`](../reference/glossary.md)。这里只强调一个区分维度：

> **验证在哪里发生？**
> - 本地验签 → 验证方需要**公钥**（分发问题）
> - 远程查询 → 验证方需要**网络可达**（可用性问题）

## 三种形态的实现

实验代码：[`labs/01-jwt-paseto-opaque/issuer.go`](../labs/01-jwt-paseto-opaque/issuer.go)

| | JWT | PASETO v4.local | Opaque |
|---|---|---|---|
| 载荷 | base64url(JSON) 明文可见 | AEAD 加密 | 无载荷，服务端存 |
| 算法 | header 里带 `alg` 字段 | **格式自带算法**（`v4.local.`） | 无算法概念 |
| 密钥 | 非对称（Ed25519） | 对称（XChaCha20-Poly1305） | 无（随机引用） |
| 验证 | 本地验签 | 本地解密+认证 | **必须查服务端** |
| 撤销 | ❌ 靠短 TTL | ❌ 靠短 TTL | ✅ 立即生效 |

**关键实现细节**（都是实测中才暴露的）：

- **PASETO 的 nonce 是 24 字节不是 32**。XChaCha20 的扩展 nonce 是 192 位 = 24 字节（`chacha20poly1305.NonceSizeX`）。我第一版写成 32 字节，测试直接报 `nonce must be 24 bytes, got 32`。
- **JWT 的 header 是固定字符串**：`{"alg":"EdDSA","typ":"JWT"}` 的 base64url 是 `eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9`（测试里断言了它）。
- **PASETO 的 payload 里连 subject 都不该出现明文**（`TestJWTPayloadVisible` 验证了这一点，包括 base64 解码后也不含）。

## 实测数据

### 基准测试

环境：Intel Xeon E5-2630 v4 @ 2.20GHz，linux/amd64，Go 1.24.4

```bash
$ go test -bench=. -benchmem -run='^$' ./labs/...
```

```
BenchmarkIssue/jwt-40              23098     51936 ns/op     2413 B/op     20 allocs/op
BenchmarkIssue/paseto-40          290473      4137 ns/op     1643 B/op     11 allocs/op
BenchmarkIssue/opaque-40          560211      2703 ns/op      707 B/op      4 allocs/op

BenchmarkValidate/jwt-40          10000    116947 ns/op     1312 B/op     25 allocs/op
BenchmarkValidate/paseto-40      183841      6193 ns/op      960 B/op     18 allocs/op
BenchmarkValidate/opaque-40    11872508       106.0 ns/op        0 B/op      0 allocs/op

BenchmarkValidateParallel/jwt-40   74712     15817 ns/op     1313 B/op     25 allocs/op
BenchmarkValidateParallel/opaque-40 13066353    92.37 ns/op        0 B/op      0 allocs/op

BenchmarkTamperCheck-40            10000    114441 ns/op      690 B/op     12 allocs/op
```

### 体积

claim set 为 126 字节的 JSON（`sub` + `aud` + 3 个 `scope` + `exp`/`iat`/`iss`）：

| | 字节数 | 构成分析 |
|---|---|---|
| **opaque** | **46** | 32 字节随机数 + base64url(43) + `op_` 前缀 |
| **paseto** | **277** | base64url(24 nonce + 126 plaintext + 16 tag) = base64url(166) |
| **jwt** | **339** | base64url(header 36 + payload 168) + base64url(64 字节签名 86) |

---

## 三个反直觉的实测发现

### 发现 1：PASETO 比 JWT **更小**

第一版测试我断言 "PASETO 应该比 JWT 大（因为有 nonce + tag 开销）"，**测试直接把这个断言打红了**。

真实原因在于**编码膨胀的分布**：

```
JWT   = b64url(header) + b64url(payload) + b64url(64B 签名)
       = (4/3) × (36 + 126)  +  86   ≈  213 字节额外开销

PASETO = b64url(nonce + plaintext + tag)
       = (4/3) × (24 + 126 + 16)      ≈   53 字节额外开销
```

JWT 要把**签名本身**也 base64 编码（64 字节 → 86 字符），而 PASETO 的签名是 AEAD tag，**跟明文一起被加密**，只需要编码一次。

**结论**：token 体积**不应该**作为选型依据 —— 两者都远低于常规 header 限制（nginx 8KB / Envoy 16KB）。

### 发现 2：JWT 验签比 PASETO 解密**慢约 19 倍**

`116947 ns` vs `6193 ns`。这与直觉相反 —— Ed25519 验签通常被认为比对称加密快。

**原因分析**（待进一步验证）：Ed25519 验签涉及椭圆曲线点运算，是 CPU 密集型；而 XChaCha20 是纯 ALU 运算，在现代 CPU 上极快且可向量化。**这台 2.2GHz 的老 Xeon 放大了这个差距** —— 在更新、更宽的 CPU 上差距会缩小，但不会反转。

> ⚠️ **待验证**：这个差距在现代 CPU（如 AMD Genoa / Apple M3）上的具体倍数。应该用 `benchstat` + 不同机器复测。

### 发现 3：Opaque 的"106 ns"是**误导性数字**

内存 map 查找 106ns，比 JWT 快 1000 倍 —— 但**生产环境是 Redis**，要加一次网络往返。

```
真实成本 ≈ 106ns (本地) + 网络 RTT (0.5ms ~ 5ms)
```

**一次跨机房 RTT 约 0.5-5ms，是 Ed25519 验签（0.117ms）的 4-40 倍。**

所以 Opaque 的性能优势**完全取决于部署拓扑**：

| 部署形态 | Opaque 验证成本 | vs JWT |
|---|---|---|
| 同机房 Redis | ~0.5-1ms | 慢 5-10 倍 |
| 同机 Redis（unix socket） | ~0.1ms | 接近 |
| 跨可用区 | ~2-5ms | **慢 20-40 倍** |
| 跨机房 | ~10-50ms | **慢 100-400 倍** |

**这才是 Opaque 的真实代价**，也是为什么"用 Opaque 换即时撤销"在多区域架构里可能不可接受。

---

## 关键决策与取舍

| 决策点 | 选项 | 取舍 |
|---|---|---|
| **是否需要即时撤销** | 短 TTL 的 JWT / Opaque | 短 TTL 把"撤销延迟"从**永久**降到**TTL**，通常可接受；只有强监管/风控场景才值得付 Opaque 的网络成本 |
| **载荷是否需要保密** | JWT(JWS) / PASETO | JWT 载荷是 base64url **不是加密**，任何持有者都能读。含手机号/身份证/PII 时必须用 PASETO 或 JWE |
| **验证方能否联网** | 自包含 / 引用 | 离线/边缘/内网隔离场景**只能**用自包含 |
| **算法混淆风险** | JWT / PASETO | JWT 的 `alg` 是 header 里的**攻击者可控字段**；PASETO 把算法编码进格式本身，攻击者无法降级。**必须用成熟库**（`golang-jwt` 强制 `alg` 白名单） |
| **密钥分发** | 对称 / 非对称 | HS256 的共享密钥悖论：验证方能验签就能**伪造**。跨组织必须用非对称 |
| **上游实现成熟度** | JWT 生态 / PASETO 生态 | JWT 生态压倒性成熟（Auth0/Okta/各云厂商都支持）；PASETO 库少，运维工具链弱 |

## 陷阱与坑

### 坑 1：把 JWT 的 base64 当加密

```go
// ❌ 错误理解：以为 payload 被保护了
token := "eyJhbGciOi..."  // 任何人都能解出 sub/email/手机号
```

base64url 是**编码**，不是加密。签名的作用是**防篡改**，不是**防窥探**。

### 坑 2：HS256 的共享密钥悖论

对称签名下，签发方和验证方持有**同一把**密钥。**验证方因此也能签发有效 token** —— 权限边界从"签发方说了算"退化成"持有密钥的人说了算"。

跨组织场景（你的服务验证别人签的 token）**必须**用 RS256/ES256/EdDSA。

### 坑 3：nonce 长度想当然

PASETO 实现时我按 32 字节写 nonce，测试报 `nonce must be 24 bytes`。**XChaCha20 的扩展 nonce 是 24 字节（192 位），不是 32 字节。**

这类错误在真实项目里会被"加密库刚好也接受 32 字节"掩盖 —— 但如果实现里 nonce 被截断，安全性就悄悄降级了。

### 坑 4：Opaque token 的性能数字不能信

`106 ns` 是内存 map 的数字。真实部署是 Redis + 网络。**任何拿微基准数字做架构决策的做法都是错的** —— 要算端到端。

## 引用

- RFC 7519 — https://datatracker.ietf.org/doc/html/rfc7519
- RFC 7515 — https://datatracker.ietf.org/doc/html/rfc7515
- PASETO v4.local spec — https://github.com/paseto-standard/paseto-spec/blob/master/docs/v4.local.md
- `vendor/golang-jwt-jwt/parser.go:ParseWithClaims` — 成熟库如何强制 alg 白名单
- `labs/01-jwt-paseto-opaque/issuer.go:JWTIssuer.Issue` — JWT 三段式构造
- `labs/01-jwt-paseto-opaque/issuer.go:PASETOIssuer.Issue` — PASETO v4.local 构造
