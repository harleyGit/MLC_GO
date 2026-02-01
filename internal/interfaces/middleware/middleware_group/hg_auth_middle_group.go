/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-26 18:10:07
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 16:43:26

* @FilePath: /MLC_GO/internal/interfaces/middleware/middleware_group/hg_tid_middle_group.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* Auth 无关接口
*/
package HGMiddlewareGroupPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	HGServerPackage "MLC_GO/server"
	"net/http"
)

/* Auth 无关接口【登录、验证码】， 子路由只写相对路径 */
func AuthMiddlewareGoup(userHandler *UserHandlerPackage.UserHandler) http.Handler {

	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/send_code", userHandler.SendCode)
	publicMux.HandleFunc("/register", userHandler.RegisterHandlerV3)
	publicMux.HandleFunc("/login", userHandler.Login)

	// 🔥 Method Guard（最外层）
	guard := HGMiddlewarePackage.NewAPIGuard(
		HGServerPackage.PublicAPIRules(),
	)

	// 正确的执行顺序（请求进入时）
	// 最外层，最先执行前置逻辑：RecoverMiddleware
	// 最内层，最后执行前置逻辑：JSONHeaderMiddleware
	recoverHandler := HGMiddlewarePackage.RecoverMiddleware(publicMux)
	loggerHander := HGMiddlewarePackage.LoggerMiddleware(recoverHandler)
	// 响应返回时的顺序,最先执行后置逻辑的是：JSONHeaderMiddleware
	// 最后执行后置逻辑的是：RecoverMiddleware
	// 即：外层中间件先执行进入逻辑，内层中间件后执行进入逻辑。
	// 统一： JOSN + TID【不加Auth】
	// 添加追踪中间件（如 TID）
	withTracing := HGMiddlewarePackage.TIDMiddleware(loggerHander)
	// 添加通用响应头（如 JSON）
	jsonHandler := HGMiddlewarePackage.JSONHeaderMiddleware(withTracing)

	// 🔥 包在最外层
	return guard.MethodGuardMiddlewareV3(jsonHandler)
}

/*
你提出的问题非常好，涉及 **Go 中间件执行顺序** 的核心理解。

---

## ✅ 先说结论：

你列出的执行顺序（从外到内）：

```
TID
 → Logger
   → RateLimit
     → CircuitBreaker
       → Retry
         → Auth
           → Handler
```

**这是错误的。**

### 正确的**请求进入时**的中间件执行顺序是：

```
Trace
 → Retry
   → CircuitBreaker
     → RateLimit
       → Auth (if enabled)
         → Logger
           → TID
             → Handler
```

> 换句话说：**代码中最后包裹的中间件（`TIDMiddleware`）最靠近 handler，最先被调用（进入时最后执行，但逻辑上最贴近业务）？不！等等——我们来澄清这个常见误区。**

---

## 🔍 关键原理回顾

在 Go 中，中间件通常这样定义：

```go
func Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 前置逻辑
        next.ServeHTTP(w, r)  // 调用下一个
        // 后置逻辑
    })
}
```

当你这样链式包裹：

```go
h = A(B(C(handler)))
```

实际调用栈是：

1. 请求先到达 `A`
2. `A` 执行前置逻辑 → 调用 `B`
3. `B` 执行前置逻辑 → 调用 `C`
4. `C` 执行前置逻辑 → 调用 `handler`
5. `handler` 返回
6. `C` 执行后置逻辑
7. `B` 执行后置逻辑
8. `A` 执行后置逻辑

所以：**代码中“最后应用”的中间件，最靠近 handler，在请求进入时最后执行前置逻辑，但最先执行后置逻辑。**

---

## 🧩 分析你的代码

```go
var h http.Handler = r.Handle

h = middleware.TraceMiddleware(r.Span)(h)               // 1️⃣ 最外层
h = middleware.RetryMiddleware(2, 100*time.Millisecond)(h) // 2️⃣
h = middleware.CircuitBreakerMiddleware(...)(h)         // 3️⃣
h = middleware.RateLimitMiddleware(...)(h)              // 4️⃣

if r.Auth {
    h = middleware.AuthMiddleware(h)                    // 5️⃣（条件）
}

h = middleware.LoggerMiddleware(h)                      // 6️⃣
h = middleware.TIDMiddleware(h)                         // 7️⃣ 最内层（最靠近 handler）
```

注意：每次都是 **把当前 `h` 传给新中间件，返回新的 `h`**，所以：

- `TIDMiddleware` 是**最后一个包裹**的 → 它是最内层 → **最接近原始 handler**
- `TraceMiddleware` 是**第一个包裹**的 → 它是最外层 → **最先接收到请求**

---

## ✅ 正确的执行顺序（请求进入时）

从前到后（即从外到内）：

```
1. TraceMiddleware      （最外层，最先执行前置逻辑）
2. RetryMiddleware
3. CircuitBreakerMiddleware
4. RateLimitMiddleware
5. AuthMiddleware       （如果启用）
6. LoggerMiddleware
7. TIDMiddleware        （最内层，最后执行前置逻辑）
8. r.Handle             （业务 handler）
```

> 所以你原来画的顺序是反的！你把“包裹顺序”和“执行顺序”搞反了。

---

## 🔄 响应返回时的顺序（后置逻辑）

从内到外：

```
1. TIDMiddleware        （最先执行后置逻辑）
2. LoggerMiddleware
3. AuthMiddleware
4. RateLimitMiddleware
5. CircuitBreakerMiddleware
6. RetryMiddleware
7. TraceMiddleware      （最后执行后置逻辑）
```

---

## ❓那为什么你觉得 `TraceMiddleware` 应该最后执行？

可能是因为你认为“追踪”应该包含所有中间件的行为，所以它应该在外层 —— **这其实是对的！**

✅ **`TraceMiddleware` 放在最外层是正确的设计**，因为它要测量整个请求链路（包括重试、熔断、限流等）的耗时和行为。

所以你的**代码顺序是对的**，但你对执行顺序的理解反了。

---

## ✅ 生产级建议的中间件顺序（从外到内）

| 层级 | 中间件                | 说明 |
|------|------------------------|------|
| 1    | **Trace / Context**    | 注入 trace ID、span、context，贯穿全程 |
| 2    | **Retry**              | 在熔断/限流之后重试？通常放较外层，但需谨慎 |
| 3    | **Circuit Breaker**    | 防止雪崩，通常在重试之后 |
| 4    | **Rate Limit**         | 限流，保护系统 |
| 5    | **Auth / ACL**         | 认证鉴权（避免对非法请求做昂贵操作）|
| 6    | **Logger**             | 记录请求日志（含 TID、状态码、耗时等）|
| 7    | **TID / Request ID**   | 生成或提取请求唯一 ID（其实常和 Trace 合并）|
| 8    | **业务 Handler**       | 实际逻辑 |

> ⚠️ 注意：`TIDMiddleware` 如果只是设置响应头中的 Request-ID，通常可以和 Trace 合并。如果它依赖 Trace 上下文，那它应该在 Trace 之后。

---

## 🔧 建议调整（更合理）

如果你的 `TIDMiddleware` 只是生成一个请求 ID 并注入响应头，**其实它应该放在最外层或与 Trace 合并**，而不是最内层。

否则：
- Logger 没有 TID（因为 TID 还没生成）
- Trace 也无法关联 TID

所以更合理的顺序可能是：

```go
h = TIDMiddleware(h)            // 生成 request ID
h = TraceMiddleware(span)(h)    // 基于 TID 创建 span
h = LoggerMiddleware(h)         // 日志可记录 TID + trace
h = RateLimitMiddleware(...)(h)
// ... 其他
```

或者更好的做法：**在 TraceMiddleware 内部统一处理 TID 和 Span**。

---

## ✅ 总结

- 你代码中的中间件**包裹顺序**决定了**执行顺序**。
- **先包裹的中间件（如 Trace）在外层，最先执行（进入时）**。
- 你画的执行顺序是反的。
- `TraceMiddleware` 应该最先执行（最外层），这样才能覆盖整个调用链 —— **你的代码是对的，理解错了**。
- `TIDMiddleware` 放在最内层可能导致日志、追踪无法使用 TID，建议调整位置。

如有需要，我可以帮你重构这个注册函数以符合最佳实践。
*/
