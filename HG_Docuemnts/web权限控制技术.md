MLC React Web IAM 权限控制技术规格文档（Tech Spec）
版本：v1.0 · 技术栈：React + JavaScript + Go + MySQL + Redis · 适用：MLC React Web 工程
一、设计目标
本方案基于现有的 MLC_GO IAM 权限体系，为 React Web 建立统一的前端权限控制框架。

采用 RBAC + Permission + Scope + ABAC 模型，实现用户登录后，根据角色、业务权限、数据作用域和资源状态，动态决定页面、菜单、模块、按钮和业务数据的可访问性。

覆盖以下业务模块：
- 平台管理：用户管理、角色管理、权限管理、IAM、MFA、安全审计。
- 运维管理：服务器监控、Kafka、Redis、任务调度、爬虫管理。
- 商城：商品管理、店铺管理、订单、支付、优惠券、会员卡。
- 视频：视频发布、审核、编辑、删除、评论、弹幕。
- 即时通信：聊天、消息中心、通知、客服。
- 个人中心：个人资料、账户安全、偏好设置。
1.1 必须实现的权限效果
控制对象	有权限	无权限
左侧菜单	正常展示	隐藏
页面路由	允许访问	403 页面
页面功能模块	展示	隐藏或无权限占位
操作按钮	正常可点击	灰色、不可执行业务
灰色按钮点击	—	Toast 提示无权限
表格行操作	按当前行权限展示	禁用或隐藏
API 请求	Go 后端执行	返回 403
数据列表	返回作用域内的数据	不返回越权数据

这里有一个关键区别：

灰色按钮不等于 HTML 的 disabled 属性。

如果使用：
<button disabled onClick={handleClick}>
  删除商品
</button>


浏览器不会触发该按钮的 onClick，因此无法直接弹出 Toast。

本方案使用 aria-disabled、灰色样式和事件拦截，实现：
1. 无权限时按钮显示灰色。
2. 鼠标悬停显示禁止操作光标。
3. 点击灰色按钮显示权限不足 Toast。
4. 不执行实际业务请求。
5. 键盘操作也不能绕过权限检查。
二、整体技术架构
                  MLC React Web
                        |
                   用户登录成功
                        |
                        v
             GET /api/iam/me/access
                        |
                        v
               IAM 权限快照
             Roles / Permissions
             Scopes / PolicyVersion
                        |
                        v
              IAMAccessProvider
                        |
                usePermission()
                        |
        +---------------+---------------+
        |               |               |
        v               v               v
    页面路由守卫     菜单/模块控制     按钮权限控制
        |               |               |
     RouteGuard     PermissionGate    AuthButton
        |               |               |
     允许/403        展示/隐藏       可用/灰色+Toast
        |               |               |
        +---------------+---------------+
                        |
                        v
                  API Request
                        |
                        v
                  Go IAM Middleware
                        |
             +----------+----------+
             |          |          |
             v          v          v
          JWT认证    Permission    Scope/ABAC
             |          |          |
             +----------+----------+
                        |
                        v
                   MySQL / Redis
                        |
                  业务数据返回


2.1 核心架构原则
前端负责用户体验，后端负责真正的权限安全。

例如，一个运营人员没有 product:delete 权限：
- React 将删除按钮置灰。
- 点击后显示“当前账号没有删除商品权限”。
- 即使用户通过浏览器开发者工具绕过前端限制，Go 后端仍返回 HTTP 403。
另外，角色不应直接等同于权限。React 应主要判断 product:delete 这样的权限码，而不是写死 role === "admin"。

这样后续新增 merchant_admin、shop_operator 等角色时，不需要到处修改 React 组件。
三、权限数据模型设计
3.1 四种核心概念
概念	作用	示例
Role	用户是什么身份	merchant
Permission	允许执行什么操作	product:update
Scope	可以操作哪些范围的数据	shop:shop_1001
ABAC	当前资源是否满足操作条件	商品属于当前店铺，且未锁定

举例：用户拥有 merchant 角色，并获得 product:update 权限，但仅能管理店铺 shop_1001 的商品。

那么：
- 编辑 shop_1001 商品：允许。
- 编辑 shop_2002 商品：拒绝。
- 编辑 shop_1001 中已锁定的商品：根据 ABAC 策略拒绝。
权限计算不是简单判断用户是否拥有某个角色，而是：
允许操作 =
    身份有效
    AND 权限有效
    AND 作用域匹配
    AND 资源属性满足策略
    AND 不存在优先级更高的拒绝规则


3.2 推荐权限码规范
统一采用：

资源:操作

例如 product:create、product:update、product:delete。

建议先建立以下权限字典，具体是否授予由 Go IAM 决定。
模块	权限码
用户	user:read, user:update, user:delete
IAM	iam:role:read, iam:role:assign, iam:permission:manage
商品	product:read, product:create, product:update, product:delete
店铺	shop:read, shop:update
订单	order:read, order:refund
视频	video:read, video:create, video:update, video:delete, video:review
评论	comment:read, comment:delete
爬虫	crawler:read, crawler:execute, crawler:manage
运维	ops:read, ops:restart, ops:config:update
聊天	chat:read, chat:send, chat:delete
客服	support:session:read, support:session:reply
消息中心	notification:read, notification:manage
个人中心	profile:read, profile:update

注意：个人用户修改自己的资料，不建议依赖全局 user:update 权限，应使用 profile:update 并由后端限定为当前用户。
3.3 作用域定义
建议 Go 后端统一维护 scope_type 字典。
scope_type	含义	scope_id
global	平台全局	空
tenant	租户	tenant ID
shop	店铺	shop ID
department	部门	department ID
project	项目	project ID
self	用户本人	当前 user ID
resource	特定资源	资源类型与 ID 的组合

例如：
{
  "role": "merchant",
  "permission": "product:update",
  "scope_type": "shop",
  "scope_id": "shop_1001"
}


这里建议把 scope_type 和 scope_id 作为 一次授权绑定的范围，而不是单独作为全局权限。一个用户拥有多个店铺授权时，应分别计算其有效范围。

对于 resource，实际落库应同时包含 resource_type，避免商品 ID 和订单 ID 发生语义冲突。
四、Go IAM 与 React 的接口协议
4.1 获取当前用户的权限快照
新增或复用接口：

GET /api/iam/me/access

必须携带当前用户的认证凭据。

返回示例：
{
  "code": 0,
  "data": {
    "user_id": "user_1001",
    "roles": [
      "merchant",
      "creator"
    ],
    "grants": [
      {
        "permission": "product:read",
        "scope_type": "shop",
        "scope_id": "shop_1001"
      },
      {
        "permission": "product:update",
        "scope_type": "shop",
        "scope_id": "shop_1001"
      },
      {
        "permission": "video:create",
        "scope_type": "self",
        "scope_id": "user_1001"
      }
    ],
    "policy_version": 18,
    "expires_at": "2026-11-20T08:00:00Z"
  }
}


这里的 grants 是一个简化后的有效授权投影，假设后端已经处理角色继承、权限分配、拒绝规则及授权有效期。

不要把所有角色权限简单合并成一个全局 permissions 数组。 否则容易出现作用域扩大问题。

例如：
角色 A：product:update + shop_1001
角色 B：product:read   + shop_2002

错误计算：
product:update + 所有店铺

正确计算：
product:update 仅 shop_1001
product:read   仅 shop_2002


4.2 资源级权限检查
新增接口：

POST /api/iam/authorize

请求：
{
  "permission": "product:delete",
  "resource": {
    "type": "product",
    "id": "product_9001"
  }
}


响应：
{
  "code": 0,
  "data": {
    "allowed": false,
    "reason_code": "RESOURCE_LOCKED",
    "message": "该商品已锁定，无法删除"
  }
}


Go 后端必须自行查询商品所属店铺、所有者、锁定状态等可信属性，不得相信 React 提交的 owner_id 或 shop_id。

该接口用于按钮的预判和说明，不代替实际删除 API 的再次鉴权。
4.3 权限错误标准化
HTTP 状态	reason_code	前端行为
401	UNAUTHENTICATED	尝试刷新登录态，失败后登录
403	PERMISSION_DENIED	Toast：没有操作权限
403	SCOPE_DENIED	Toast：不在可操作范围
403	RESOURCE_LOCKED	Toast：资源已锁定
403	MFA_REQUIRED	引导完成 MFA
403	POLICY_CHANGED	重新获取权限快照

MFA_REQUIRED 可作为项目约定的业务错误码，但敏感操作是否必须 MFA，必须由 Go 后端判定。
五、React 工程目录设计
推荐独立建立 src/iam 模块，不要把权限判断散落到每一个页面。
src/
├── iam/
│   ├── api/
│   │   └── iamApi.js
│   ├── core/
│   │   ├── accessEvaluator.js
│   │   └── permissionCodes.js
│   ├── context/
│   │   └── IAMAccessProvider.jsx
│   ├── hooks/
│   │   └── usePermission.js
│   ├── components/
│   │   ├── PermissionGate.jsx
│   │   ├── AuthButton.jsx
│   │   ├── RouteGuard.jsx
│   │   └── AccessDenied.jsx
│   └── tests/
│       └── accessEvaluator.test.js
├── components/
│   └── toast/
│       └── ToastProvider.jsx
├── config/
│   └── menuConfig.js
├── modules/
│   ├── mall/
│   ├── video/
│   ├── chat/
│   ├── notification/
│   ├── operations/
│   ├── iam-admin/
│   └── profile/
└── App.jsx


依赖选择： React Context、Hooks、Fetch API、原生 JavaScript、CSS 即可实现权限核心。路由部分复用现有 React Router；如果工程已经安装状态管理库，不需要为了 IAM 再安装一套。
六、核心权限引擎实现
6.1 accessEvaluator.js
这个文件只负责纯 JavaScript 权限判断，不依赖 React，方便单元测试。

const VALID_SCOPES = new Set([
  "global", "tenant", "shop",
  "department", "project", "self", "resource"
]);

export function evaluateAccess(snapshot, requirement) {
  if (!snapshot || !requirement?.permission) {
    return { allowed: false, reason: "权限尚未加载" };
  }

  if (!Array.isArray(snapshot.grants)) {
    return { allowed: false, reason: "权限数据无效" };
  }

  if (!snapshot.expires_at ||
      Date.parse(snapshot.expires_at) <= Date.now()) {
    return { allowed: false, reason: "权限已过期" };
  }

  const {
    permission,
    scopeType,
    scopeId
  } = requirement;

  const grants = snapshot.grants.filter(
    g => g.permission === permission &&
      VALID_SCOPES.has(g.scope_type)
  );

  if (!grants.length) {
    return { allowed: false, reason: "没有该操作权限" };
  }

  // 没有指定资源范围时，只允许全局授权直接通过。
  // 其他作用域授权必须提供具体资源上下文。
  const matched = grants.some(g => {
    if (g.scope_type === "global") return true;

    return scopeType === g.scope_type &&
      Boolean(scopeId) &&
      scopeId === g.scope_id;
  });

  return matched
    ? { allowed: true, reason: "" }
    : { allowed: false, reason: "不在授权作用域内" };
}



该函数是前端快速预判器，适用于作用域明确、授权已由后端展开的场景。

它刻意不支持自动推断店铺归属、跨租户继承、资源所有者、拒绝规则及 ABAC 条件。这些必须通过 Go IAM 或资源级授权接口完成。

尤其注意：对于一个具体商品，不能仅因为 React 收到了 shopId 就认为它可信。服务端仍需根据商品 ID 重新确定归属。
6.2 IAMAccessProvider.jsx
统一加载权限快照，并向整个 React 应用提供权限状态。

import {
  createContext, useCallback,
  useContext, useEffect, useState
} from "react";

const IAMContext = createContext(null);

export function IAMAccessProvider({ children }) {
  const [snapshot, setSnapshot] = useState(null);
  const [status, setStatus] = useState("loading");

  const refresh = useCallback(async () => {
    setStatus("loading");
    setSnapshot(null);

    try {
      const response = await fetch("/api/iam/me/access", {
        credentials: "include",
        cache: "no-store"
      });

      if (!response.ok) {
        throw new Error(`IAM HTTP ${response.status}`);
      }

      const body = await response.json();

      if (body.code !== 0 || !body.data?.grants) {
        throw new Error("Invalid IAM response");
      }

      setSnapshot(body.data);
      setStatus("ready");
    } catch {
      setSnapshot(null);
      setStatus("error");
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <IAMContext.Provider
      value={{ snapshot, status, refresh }}
    >
      {children}
    </IAMContext.Provider>
  );
}

export function useIAM() {
  const context = useContext(IAMContext);
  if (!context) {
    throw new Error("IAMAccessProvider missing");
  }
  return context;
}



这里假设认证凭据通过 HttpOnly Cookie 发送。如果当前 MLC_GO 使用 Bearer Token，则应改为调用现有的统一 API Client 注入 Access Token，不建议将 Token 放入普通 localStorage。

生产环境还需在 Provider 中接入登录、退出、切换账号和权限版本变更事件。refresh() 必须具备并发请求防护，防止旧账号的请求结果覆盖新账号权限。
6.3 usePermission.js

import { useIAM } from "../context/IAMAccessProvider";
import { evaluateAccess } from "../core/accessEvaluator";

export function usePermission(requirement) {
  const { snapshot, status } = useIAM();

  if (status !== "ready") {
    return {
      allowed: false,
      loading: status === "loading",
      reason: "权限尚未就绪"
    };
  }

  return {
    ...evaluateAccess(snapshot, requirement),
    loading: false
  };
}



使用方式：
const access = usePermission({
  permission: "product:update",
  scopeType: "shop",
  scopeId: "shop_1001"
});

if (access.allowed) {
  // 可以展示编辑功能
}


七、权限按钮：灰色 + Toast + 阻止业务执行
7.1 预期 UI 效果
下面是交互示意，可直接点击灰色按钮体验无权限提示。
商品管理权限演示
商家 merchant


商品：无线蓝牙耳机

店铺：shop_1001 · 状态：在售

已上架

编辑商品删除商品


已通过前端编辑权限检查（演示）


仅模拟角色权限的前端交互；正式系统还需检查真实授权作用域及后端策略。
7.2 AuthButton.jsx
import { usePermission } from "../hooks/usePermission";
import { useToast } from "../../components/toast/ToastProvider";
import "./AuthButton.css";

export default function AuthButton({
  permission,
  scopeType,
  scopeId,
  onClick,
  children,
  disabled = false,
  ...props
}) {
  const access = usePermission({
    permission, scopeType, scopeId
  });
  const toast = useToast();

  const blocked = !access.allowed || disabled;

  function handleClick(event) {
    if (blocked) {
      event.preventDefault();
      event.stopPropagation();

      toast.warning(
        !access.allowed
          ? access.reason
          : "当前操作暂不可用"
      );
      return;
    }

    onClick?.(event);
  }

  return (
    <button
      {...props}
      type={props.type || "button"}
      aria-disabled={blocked}
      className={[
        "auth-button",
        blocked ? "auth-button--blocked" : "",
        props.className || ""
      ].join(" ")}
      onClick={handleClick}
    >
      {children}
    </button>
  );
}


AuthButton.css：
.auth-button {
  padding: 9px 16px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: #438BFA;
  color: #fff;
  cursor: pointer;
}

.auth-button--blocked {
  background: #E5E7EB;
  border-color: #D1D5DB;
  color: #9CA3AF;
  cursor: not-allowed;
}

/* 不要使用 pointer-events: none */


aria-disabled 仅表达禁用语义，不会自动阻止事件，所以必须保留 handleClick 中的拦截逻辑。

MDN+1

业务页面使用：
<AuthButton
  permission="product:delete"
  scopeType="shop"
  scopeId={product.shopId}
  onClick={() => deleteProduct(product.id)}
>
  删除商品
</AuthButton>


对于删除、退款、角色授权等高风险操作，建议使用后端资源级授权结果决定按钮是否可用；点击后实际 API 仍需重新鉴权。
八、Toast 通知组件实现
不额外安装第三方 SDK，直接使用 React Context 实现全局 Toast。

src/components/toast/ToastProvider.jsx

import {
  createContext, useCallback,
  useContext, useEffect,
  useRef, useState
} from "react";

const ToastContext = createContext(null);

export function ToastProvider({ children }) {
  const [message, setMessage] = useState("");
  const timerRef = useRef(null);

  const warning = useCallback((text) => {
    clearTimeout(timerRef.current);
    setMessage(text);
    timerRef.current = setTimeout(() => {
      setMessage("");
    }, 3000);
  }, []);

  useEffect(() => {
    return () => clearTimeout(timerRef.current);
  }, []);

  return (
    <ToastContext.Provider value={{ warning }}>
      {children}
      {message && (
        <div
          className="mlc-toast"
          role="status"
          aria-live="polite"
        >
          {message}
        </div>
      )}
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("ToastProvider missing");
  }
  return ctx;
}



对应 CSS：
.mlc-toast {
  position: fixed;
  top: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  padding: 12px 20px;
  background: #172B4D;
  color: #FFFFFF;
  border-radius: 10px;
  box-shadow: 0 8px 24px #172B4D22;
  max-width: min(90vw, 480px);
}


正式工程建议支持 success、warning、error、info 四种类型，并增加消息去重，避免连续点击禁用按钮产生大量 Toast。
九、模块展示与隐藏
9.1 PermissionGate.jsx
统一控制某个业务模块是否展示。
import { usePermission } from "../hooks/usePermission";

export default function PermissionGate({
  permission,
  scopeType,
  scopeId,
  fallback = null,
  children
}) {
  const access = usePermission({
    permission,
    scopeType,
    scopeId
  });

  return access.allowed ? children : fallback;
}


使用示例：
<PermissionGate
  permission="crawler:read"
>
  <CrawlerDashboard />
</PermissionGate>


上述示例仅在拥有全局 crawler:read 授权时展示。如果爬虫权限限定在某个项目，需要传入对应 scopeType 和 scopeId。
9.2 页面多模块权限
例如运维管理页面：
function OperationsPage() {
  return (
    <main>
      <PermissionGate permission="ops:read">
        <ServerMetrics />
      </PermissionGate>

      <PermissionGate permission="crawler:read">
        <CrawlerTasks />
      </PermissionGate>

      <PermissionGate permission="iam:role:read">
        <IAMRoleOverview />
      </PermissionGate>
    </main>
  );
}


对于没有任何权限的模块，不请求对应的敏感数据，避免出现页面隐藏但网络接口仍自动请求的情况。
十、菜单与路由权限控制
10.1 菜单配置化
src/config/menuConfig.js
export const menuConfig = [
  {
    key: "mall",
    title: "商城管理",
    path: "/mall",
    permission: "product:read"
  },
  {
    key: "video",
    title: "视频管理",
    path: "/video",
    permission: "video:read"
  },
  {
    key: "operations",
    title: "运维管理",
    path: "/operations",
    permission: "ops:read"
  },
  {
    key: "iam",
    title: "权限管理",
    path: "/iam",
    permission: "iam:role:read"
  }
];


菜单项的权限可以扩展为 anyOf 和 allOf：
- anyOf：拥有任意一个权限即可展示。
- allOf：必须同时拥有所有指定权限。
但是必须使用同一资源上下文检查多个权限，不能将不同店铺的授权组合成一个虚假的完整权限。
10.2 RouteGuard.jsx
以下示例假设工程使用 React Router v6 或兼容 API。
import { Navigate } from "react-router-dom";
import { usePermission } from "../hooks/usePermission";

export default function RouteGuard({
  permission,
  children
}) {
  const access = usePermission({ permission });

  if (access.loading) {
    return <div>正在验证权限...</div>;
  }

  if (!access.allowed) {
    return <Navigate to="/403" replace />;
  }

  return children;
}


路由注册：
<Route
  path="/operations"
  element={
    <RouteGuard permission="ops:read">
      <OperationsPage />
    </RouteGuard>
  }
/>


生产版本还要区分权限加载失败、未登录和权限不足：
状态	行为
loading	显示加载状态
unauthenticated	跳转登录
ready + allowed	展示页面
ready + denied	展示 403
error	展示加载失败和重试入口

不能把网络请求失败直接当成用户没有权限。 网络失败应该拒绝展示受保护数据，但同时允许用户重试。
十一、复杂作用域与 ABAC 的实际处理
这是整个方案最重要的部分。

对于普通菜单，React 可以通过权限快照快速判断。

但是对于具体商品、订单、视频、用户资料等资源，不能只依靠本地权限码。
11.1 商品列表示例
假设商家有如下授权：
权限	作用域
product:read	shop_1001
product:update	shop_1001
product:delete	无
order:read	shop_1001

展示商品列表：
商品管理
merchant · shop_1001

商品	操作权限
蓝牙耳机 shop_1001	可编辑 禁止删除
智能手表 shop_2002	无权管理

示例数据，实际商品列表应由 Go 后端按照授权作用域过滤。
建议商品列表接口直接返回资源级可执行操作：
{
  "id": "product_1001",
  "title": "蓝牙耳机",
  "shop_id": "shop_1001",
  "allowed_actions": {
    "read": true,
    "update": true,
    "delete": false
  },
  "denied_reasons": {
    "delete": "没有商品删除权限"
  }
}


这样有三个优势：
1. React 无需对列表中每个商品单独调用一次 /api/iam/authorize。
2. Go 后端可以一次性处理角色、作用域和 ABAC。
3. 前端按钮状态与后端授权决策更加一致。
注意： allowed_actions 是当前资源状态下的权限快照，不能作为后续操作的最终授权依据。
11.2 列表按钮组件
function ProductRow({ product }) {
  const toast = useToast();

  const canDelete =
    product.allowed_actions?.delete === true;

  function handleDelete() {
    if (!canDelete) {
      toast.warning(
        product.denied_reasons?.delete ||
        "没有删除商品权限"
      );
      return;
    }

    deleteProduct(product.id);
  }

  return (
    <button
      type="button"
      aria-disabled={!canDelete}
      className={
        canDelete
          ? "auth-button"
          : "auth-button auth-button--blocked"
      }
      onClick={handleDelete}
    >
      删除商品
    </button>
  );
}


对于列表中的资源级权限，优先使用后端返回的 allowed_actions，不要让前端自行推断资源所有权。
十二、权限动态更新机制
管理员可能在用户正在操作时修改角色或权限。

例如：
管理员撤销 product:delete
          |
          v
Go IAM 更新授权记录
          |
          v
policy_version 18 -> 19
          |
          v
发布权限变更事件
          |
          v
Centrifugo WebSocket
          |
          v
React 收到 iam.access.changed
          |
          v
重新获取 /api/iam/me/access
          |
          v
刷新菜单、模块、按钮状态


结合现有 Centrifugo 实时通信架构，建议采用：

WebSocket 变更通知 + HTTP 拉取最新权限快照。

事件示例：
{
  "event": "iam.access.changed",
  "data": {
    "user_id": "user_1001",
    "policy_version": 19
  }
}


前端收到事件后：
if (event.event === "iam.access.changed") {
  await refreshIAMAccess();
}


权限事件只携带版本号，不建议在 WebSocket 事件中直接下发完整授权信息。

需要处理的边界情况：
- 权限事件重复发送：根据版本号去重。
- WebSocket 断线：重连后重新同步权限。
- 多浏览器标签页：可使用原生 BroadcastChannel 通知其他标签页刷新。
- 用户退出：清除权限快照和用户相关缓存。
- 权限撤销：Go 服务端立即按最新有效策略拒绝请求，不能等待前端刷新。
- 权限快照过期：进入受保护操作的默认拒绝状态，重新拉取。
建议增加页面重新获得焦点时的版本校验，作为 WebSocket 通知丢失时的补偿机制。
十三、Go 后端必须配套实现的能力
前端的所有权限判断最终都依赖后端 IAM。

建议后端按以下流程执行：
HTTP Request
     |
     v
JWT Authentication
     |
     v
Resolve Current User
     |
     v
Resolve Effective Grants
     |
     v
Permission Check
     |
     v
Load Trusted Resource Attributes
     |
     v
Scope + ABAC Evaluation
     |
     +----------+
     |          |
   Allow       Deny
     |          |
     v          v
 Business     HTTP 403
 Execution


13.1 推荐 IAM 数据关系
iam_users / users
       |
       v
iam_user_role_bindings
       |
       v
iam_roles
       |
       v
iam_role_permissions
       |
       v
iam_permissions


这里的表名是推荐的逻辑结构，不要求重复创建现有表。

如果当前 iam_user_role_bindings 已经包含 scope_type 和 scope_id，可以继续复用。

但需要特别注意：角色绑定范围和单个权限的授权范围可能不同。

例如，一个用户的 merchant 角色限定在店铺 A，角色中的所有权限都只能在店铺 A 生效。后端应在展开有效授权时保留这种绑定关系。

如果未来支持权限级独立作用域、显式拒绝和复杂条件，再扩展策略表，不建议在 React 中重新实现这些数据库关系。
13.2 服务端权限判断要求
后端至少保证：
- 每个受保护 API 都有权限声明。
- 默认拒绝未声明授权的请求。
- 资源级操作必须验证资源归属。
- 不能相信客户端提交的作用域。
- 数据查询必须在 SQL 或业务查询层限制可见范围。
- 权限变更必须使相关缓存失效。
- 高风险权限操作记录审计日志。
- super_admin 的特殊授权规则在 Go IAM 统一处理，React 不写死超级管理员绕过逻辑。
OWASP 也明确建议采用默认拒绝、最小权限及服务端逐请求鉴权。

OWASP Cheat Sheet Series+1

十四、API 请求统一处理
React 中建议使用统一的 API Client。
export async function apiRequest(url, options = {}) {
  const response = await fetch(url, {
    ...options,
    credentials: "include"
  });

  if (response.status === 401) {
    throw new Error("UNAUTHENTICATED");
  }

  if (response.status === 403) {
    const body = await response.json().catch(() => ({}));

    const error = new Error(
      body.message || "没有执行该操作的权限"
    );

    error.code = body.reason_code || "PERMISSION_DENIED";
    error.status = 403;

    throw error;
  }

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  return response.json();
}


这个示例仅展示统一错误处理。生产工程需要复用现有 Access Token 刷新、请求取消、幂等、超时和错误响应规范。

业务页面统一捕获错误并调用 Toast，而不是在每个模块重复实现权限错误逻辑。
十五、自动化测试与验收标准
15.1 权限测试矩阵
测试场景	预期
用户未登录	不能访问受保护页面
用户无模块权限	菜单隐藏，直达 URL 返回 403
用户无按钮权限	按钮灰色，点击 Toast
用户有权限但 Scope 不匹配	禁止操作
用户有权限但资源已锁定	禁止操作
用户切换角色	权限状态刷新
管理员撤销权限	旧权限不能继续执行业务
用户伪造前端权限数据	Go API 仍拒绝
用户修改商品 ID	不能跨店铺操作
WebSocket 断线重连	权限重新同步
权限接口超时	默认拒绝并支持重试
多标签页退出登录	所有标签页权限失效

15.2 单元测试
建议使用现有测试框架。若当前工程采用 Vitest：
import { describe, expect, it } from "vitest";
import { evaluateAccess } from "../core/accessEvaluator";

describe("IAM access evaluator", () => {
  const snapshot = {
    expires_at: "2099-01-01T00:00:00Z",
    grants: [
      {
        permission: "product:update",
        scope_type: "shop",
        scope_id: "shop_1001"
      }
    ]
  };

  it("允许操作授权店铺", () => {
    expect(evaluateAccess(snapshot, {
      permission: "product:update",
      scopeType: "shop",
      scopeId: "shop_1001"
    }).allowed).toBe(true);
  });

  it("拒绝跨店铺操作", () => {
    expect(evaluateAccess(snapshot, {
      permission: "product:update",
      scopeType: "shop",
      scopeId: "shop_2002"
    }).allowed).toBe(false);
  });

  it("拒绝未授权的删除操作", () => {
    expect(evaluateAccess(snapshot, {
      permission: "product:delete",
      scopeType: "shop",
      scopeId: "shop_1001"
    }).allowed).toBe(false);
  });
});


另外必须增加端到端测试：验证灰色按钮点击后只产生 Toast，不会发起删除请求。
十六、开发实施顺序
建议分四个阶段实施，避免一次性改动所有模块。

阶段一：IAM 协议统一

完成 Go 权限快照、权限码字典、Scope 规则、资源授权检查接口和 403 错误规范。

验收：后端能够正确区分不同角色、店铺和资源的权限。




阶段二：React IAM 基础框架

完成 IAMAccessProvider、usePermission、PermissionGate、RouteGuard、AuthButton 和 Toast。

验收：任意页面可以通过统一组件控制显示、隐藏和禁用。




阶段三：业务模块接入

按 IAM 管理、商城、视频、运维、消息中心、聊天、个人中心逐步接入。

验收：菜单、路由、表格行操作和敏感 API 均受控。




阶段四：动态权限与安全验收

接入 Centrifugo 权限事件、缓存失效、审计日志、权限测试矩阵和跨账号测试。

验收：权限撤销后，前端及时更新，后端立即拒绝越权操作。



十七、最终工程规范
建议把以下要求作为 MLC React Web 的强制开发规范：
1. 所有业务权限码由统一文件管理，禁止在页面中随意拼接。
2. 不允许通过 role === "admin" 直接决定敏感操作权限。
3. 菜单使用配置化权限控制。
4. 页面使用 RouteGuard，模块使用 PermissionGate。
5. 普通操作按钮使用 AuthButton。
6. 资源级操作优先使用 Go 返回的 allowed_actions。
7. 灰色权限按钮必须阻止操作并提供 Toast。
8. 业务 API 必须由 Go IAM 再次鉴权。
9. 用户退出、权限撤销、账号切换时清理权限状态。
10. 所有权限判断默认拒绝，只有明确允许才能执行。
最终建议
对于 MLC_GO + React 这套工程，推荐采用：

Go IAM 作为唯一权威授权决策中心，React IAM 作为统一权限展示与交互层。

前端只需维护 usePermission、PermissionGate、RouteGuard、AuthButton 等基础能力，后续无论增加商城模块、视频审核、运维管理还是客服系统，都不需要重新开发一套权限控制逻辑。

需要特别区分两类权限：菜单和模块级权限适合通过权限快照判断；具体资源的修改、删除、退款、审批等操作，应由 Go 后端返回资源级决策并在执行时再次校验。

这能兼顾代码复用、前端交互体验，以及大规模多角色、多租户、多作用域场景下的安全性。