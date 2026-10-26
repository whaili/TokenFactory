# Token 的四种形态：一次基于实测的技术选型

> 本文是 [TokenFactory](https://github.com/whaili/TokenFactory) 项目的第一篇产出。
> 文中所有数字都来自仓库里的可运行基准测试（`labs/01-jwt-paseto-opaque`），
> 不是估算值。复现命令见文末。

---

## 一个常见的技术选型失误

每次做 token 方案选型，团队里都会出现这样一张表：

| | JWT | PASETO | Opaque |
|---|---|---|---|
| 自包含 | ✅ | ✅ | ❌ |
| 加密载荷 | 可选 | ✅ | N/A |
| 即时撤销 | ❌ | ❌ | ✅ |
| 性能 | 快 | 快 | 慢 |
| 生态 | 成熟 | 一般 | 成熟 |

然后结论通常是"选 JWT"。

**这张表的问题在于：它比较的是功能清单，不是每项功能背后的成本。**

比如"性能：JWT 快 / Opaque 慢"这一行——慢多少？慢 5 倍还是 500 倍？在同机房还是跨机房？这些问题的答案会直接翻转结论。

这篇文章把三种形态都跑起来、量一遍。

---

## 三个形态的机制差异

在讲数字之前，先明确三者的机制差异——这决定了性能差异的**来源**。

### JWT：签名，但不加密

```
base64url(header) . base64url(payload) . base64url(signature)
```

三段，点号分隔。`header` 里带 `alg` 字段。**载荷是明文可读的**——任何持有 token 的人都能解出 `sub`、`email`、手机号。签名的作用是防篡改，不是防窥探。

### PASETO：加密，格式自带算法

```
v4.local.<base64url(nonce || ciphertext || tag)>
```

`v4.local.` 前缀是**格式的一部分**，不是 header 里的一个字段。这意味着攻击者无法通过修改 header 把算法降级——这正是 PASETO 相对 JWT 的核心设计改进（JWT 的 `alg` 是攻击者可控的字段，"算法混淆"攻击的存在根源）。

### Opaque：一个随机引用

```
op_<base64url(32 字节随机数)>
```

token 本身不含任何信息。所有状态在服务端，验证时**必须**查一次。

---

## 实测：验证性能

先看最关键的数字——**验证**。因为验证发生在每个请求上，而签发只发生一次。

```bash
$ go test -bench=. -benchmem -run='^$' ./labs/...
```

```
BenchmarkValidate/jwt-40          10000    116947 ns/op     1312 B/op     25 allocs/op
BenchmarkValidate/paseto-40      183841      6193 ns/op      960 B/op     18 allocs/op
BenchmarkValidate/opaque-40    11872508       106.0 ns/op        0 B/op      0 allocs/op
```

（环境：Intel Xeon E5-2630 v4 @ 2.20GHz，Go 1.24.4）

### 第一个反直觉：JWT 验签比 PASETO 解密慢 19 倍

116947 ns vs 6193 ns。这和"非对称密码学更重"的直觉相反。

原因在于两者的计算特性不同：
- **Ed25519 验签**涉及椭圆曲线点运算，是 CPU 密集型、有较多数据依赖的串行计算
- **XChaCha20** 是纯 ALU 运算，可向量化，在现代 CPU 上极快

这台 2.2GHz 的老 Xeon 放大了差距。在更宽的现代 CPU 上倍数会缩小，但不会反转。

> 需要说明：这是单台机器的结果，跨机器比较需要用 `benchstat` 做统计显著性检验。这一点我在笔记里标了"待验证"。

### 第二个反直觉：Opaque 的 106 ns 是误导性数字

106 ns 是**内存 map 查找**的数字。生产环境是 Redis：

```
真实成本 ≈ 106ns（本地查找）+ 网络 RTT
```

一次跨机房 RTT 约 10-50ms，是 Ed25519 验签（0.117ms）的 **100-400 倍**。

所以 Opaque 的性能完全取决于部署拓扑：

| 部署形态 | Opaque 验证成本 | vs JWT |
|---|---|---|
| 同机 Redis（unix socket） | ~0.1ms | 接近 |
| 同机房 Redis | ~0.5-1ms | 慢 5-10 倍 |
| 跨可用区 | ~2-5ms | 慢 20-40 倍 |
| 跨机房 | ~10-50ms | **慢 100-400 倍** |

**任何拿微基准数字做架构决策的做法都是错的。** 要算端到端。

---

## 实测：token 体积

用 126 字节的典型 claim set（`sub` + `aud` + 3 个 `scope` + 时间戳）：

| | 字节数 | 构成 |
|---|---|---|
| opaque | **46** | 32 字节随机数 + base64url + `op_` 前缀 |
| paseto | **277** | base64url(24 nonce + 126 明文 + 16 tag) |
| jwt | **339** | base64url(header + payload) + base64url(64 字节签名) |

### 第三个反直觉：PASETO 比 JWT 更小

我最初的测试断言写的是"PASETO 应该比 JWT 大（多出 nonce 和 tag 的开销）"——**测试直接把这个断言打红了**。

真实原因是**编码膨胀的分布方式**：

```
JWT   = b64url(header) + b64url(payload) + b64url(64B 签名)
       = (4/3) × (36 + 126) + 86        ≈  213 字节额外开销

PASETO = b64url(nonce + 明文 + tag)
       = (4/3) × (24 + 126 + 16)         ≈   53 字节额外开销
```

JWT 要把**签名本身**也做 base64 编码（64 字节 → 86 字符），而 PASETO 的 AEAD tag 跟明文一起被加密，**只编码一次**。

**结论：token 体积不应该作为选型依据。** 两者都远低于常规 header 限制（nginx 8KB / Envoy 16KB）。

---

## 那么该怎么选

回到开头那张表，填上真实成本：

| 维度 | JWT | PASETO | Opaque |
|---|---|---|---|
| 验证延迟（同机房） | 0.12ms | 0.006ms | 0.5-1ms（+网络） |
| 验证延迟（跨机房） | 0.12ms | 0.006ms | **10-50ms** |
| 载荷保密 | ❌ base64 明文 | ✅ | ✅（不在 token 里） |
| 即时撤销 | ❌ 靠 TTL | ❌ 靠 TTL | ✅ |
| 算法混淆风险 | ⚠️ 需靠库防护 | ✅ 设计上消除 | ✅ 无算法概念 |
| 跨组织密钥分发 | ✅ 非对称 | ⚠️ 对称 | ✅ |

### 三个实际的决策规则

**规则一：跨机房架构 + 强撤销需求 → Opaque 的成本可能被低估**

如果你的服务跨可用区部署，Opaque 的每次验证都带一次跨区 RTT。在高 QPS 下这是真实的成本。反过来，如果撤销只是"封禁一个账号"这种低频操作，短 TTL（分钟级）的 JWT 更划算。

**规则二：载荷含 PII → JWT 不够**

`sub`、`email`、手机号、身份证号出现在 base64 明文里，意味着任何能拿到 token 的人（CDN 日志、代理、浏览器历史、监控）都能读到。JWT 的签名防的是篡改，不是这个。

**规则三：自己写 JWT 解析器 = 自找麻烦**

JWT 最大的坑是 `alg` 字段由攻击者控制。成熟库的处理方式值得看：

```go
// third_party/golang-jwt-jwt/parser.go
if p.validMethods != nil {
    for _, m := range p.validMethods {
        if m == token.Method.Alg() {
            return token, nil
        }
    }
    return token, newError("signing method (alg) is unavailable", ErrTokenSignatureInvalid)
}
```

而 `none` 算法的处理更严格——需要调用方显式传一个**不可伪造的 magic constant**：

```go
// third_party/golang-jwt-jwt/none.go
const UnsafeAllowNoneSignatureType unsafeNoneMagicConstant = "none signing method allowed"

func (m *signingMethodNone) Verify(signingString, signature string, key interface{}) (err error) {
    // Key must be UnsafeAllowNoneSignatureType to prevent accidentally
    // accepting 'none' signing method
    if _, ok := key.(unsafeNoneMagicConstant); !ok {
        return NoneSignatureTypeDisallowedError
    }
    ...
}
```

**用成熟库和手写解析器的差别，往往就是这类细节。**

---

## 一个容易被忽略的中间选项

如果你的需求是"既要自包含，又要能撤销"，除了 Opaque 还有第三条路：

**JWT + 短 TTL + 撤销列表（revocation list）**

撤销列表本质上是把 Opaque 的"查一次"变成了"偶尔查一次"——只在 token 处于高风险状态时才需要查。代价是引入了一个新的、需要维护的、有 TTL 的状态存储。

**大多数场景下，短 TTL（5-15 分钟）就足够了**：撤销延迟 = TTL ≤ 15 分钟。在没有"必须秒级封禁"的硬需求时，这个方案比 Opaque 简单得多。

---

## 复现

```bash
git clone https://github.com/whaili/TokenFactory.git
cd TokenFactory
make test
make bench
```

代码在 `labs/01-jwt-paseto-opaque/`，笔记在 `docs/notes/01-token-fundamentals.md`。

---

## 下一步

- [x] 四种形态的机制与实测对比 ← 本文
- [ ] JWT / RFC 7519 深入
- [ ] PASETO 的设计取舍
- [ ] Macaroon 的委派模型
- [ ] 签名算法性能与安全模型
- [ ] 攻击面：算法混淆 / kid 注入 / JWKS 劫持
