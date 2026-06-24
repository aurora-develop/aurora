# /reverse-sentinel Skill 设计文档

> **触发词**: `/reverse-sentinel` — 当 ChatGPT 网页端 sentinel 指纹协议更新后，引导式半自动化逆向新版协议并修复 aurora 项目

## 目标

将 sentinel 指纹协议逆向流程固化为可复用的 Claude Code Skill，使得每次 ChatGPT 网页端风控更新后，通过单一指令 + CDP 浏览器 MCP 工具即可完成逆向分析和代码修复，减少手动重复劳动和遗漏风险。

## 架构

```
Phase 1: 侦察          Phase 2: 网络抓包         Phase 3: SDK 分析
┌────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│ CDP 导航       │    │ XHR 断点         │    │ 下载新版 sdk.js  │
│ 提取 build-id  │───▶│ /sentinel/* 抓包  │───▶│ 搜索指纹函数     │
│ 提取 SDK URL   │    │ 解码 base64 cfg  │    │ 对比旧版差异     │
│ 检查 cookie    │    │ 提取 25 元素数组 │    │ 检查 turnstile/so│
└────────────────┘    └──────────────────┘    └──────────────────┘
                                                        │
Phase 4: 字段映射        Phase 5: 代码修复       Phase 6: 验证
┌──────────────────┐    ┌──────────────────┐    ┌───────────────┐
│ 模式匹配自动标注 │    │ fingerprint.go   │    │ go build      │
│ JS 浏览器交叉验证│───▶│ prooftoken.go    │───▶│ go vet        │
│ 未知字段人工确认 │    │ request.go       │    │ go test       │
│ 输出映射表       │    │ turnstile.go     │    │ 输出变更摘要  │
│                  │    │ so.go            │    │               │
│                  │    │ browserfp.go     │    │               │
└──────────────────┘    └──────────────────┘    └───────────────┘
```

## 触发条件

以下任一症状:
- aurora 请求被路由到 mini 池 (模型降级)
- sentinel prepare/req/finalize 返回 4xx / force_login
- ChatGPT 网页端已知有 UI/功能更新
- 用户手动调用 `/reverse-sentinel`

## 前置检查 (Phase 0)

1. 检查 CDP 浏览器 MCP 工具可用 (`js-reverse` + `cloakbrowser`)
2. 检查 ChatGPT 会话凭证 (`access_token` 或 `__Secure-next-auth.session-token`)，无则提示用户先登录
3. 创建 `docs/superpowers/reverse-sessions/YYYY-MM-DD/` 资产目录
4. `git stash` 保护当前工作区

## Phase 1: 侦察

**工具**: js-reverse `new_page` / `navigate_page` / `list_scripts` / `get_script_source`

**步骤**:
1. 导航到 `https://chatgpt.com/`，等待页面加载完毕
2. 提取 `<html data-build="...">` 中的 `data-build` 值 → 新版 BuildID
3. 列出所有 `<script src>`，找到 sentinel SDK URL (匹配 `/sentinel/` 或 `sdk.js`)
4. 保存首页 HTML 到资产目录 `chatgpt-page.html`
5. 提取新版 User-Agent (从 `navigator.userAgent`)
6. 提取新版 Chrome 版本号 → 更新 `Sec-Ch-Ua` 头

**人工确认**: 新版 BuildID 和 SDK URL

## Phase 2: 网络抓包

**工具**: js-reverse `break_on_xhr` / `list_network_requests` / `get_request_initiator` / `evaluate_script`

**步骤**:
1. 在 chatgpt.com 页面，设置 XHR 断点 `break_on_xhr("sentinel")` 捕获所有 `/sentinel/*` 请求
2. 触发一次对话 (发送简单消息) — 让浏览器自然触发 sentinel 流程
3. 收集以下请求的 request body:
   - `POST /sentinel/chat-requirements/prepare` — 提取 `p` 字段 (requirements token)
   - `POST /sentinel/req` — 提取 `p` 字段 (sentinel req token)
   - `POST /sentinel/chat-requirements/finalize` — 提取完整 payload
4. 对每个 token 解码: 去掉 `gAAAAAC`/`gAAAAAB` 前缀和 `~S` 后缀 → base64 decode → JSON parse → 得到 25 元素数组
5. 保存原始 payload 和解码结果到资产目录
6. 确定新的数组长度 (如果不再是 25)

**人工确认**: 网络请求捕获成功，数组元素数量确认

## Phase 3: SDK 脚本分析

**工具**: js-reverse `save_script_source` / `search_in_sources` / `list_scripts`

**步骤**:
1. 下载新版 sentinel SDK JS 到资产目录 `sdk.js`
2. 同时下载 turnstile DX 脚本和 SO collector/snapshot 脚本 (如果有独立的 script src)
3. 搜索关键函数名:
   - `_generateRequirementsToken` / `generateRequirementsToken`
   - `buildGenerateFailMessage`
   - `fingerprintSize` / 数组长度常量
   - `FNV1a` / `fnv1a_hash`
   - `PrefixProof` / `PrefixRequirements` / `errorPrefix`
4. 提取新版常量:
   - `flow` 默认值
   - `PrefixRequirements` (目前 `gAAAAAC`)
   - `PrefixProof` (目前 `gAAAAAB`)
   - `errorPrefix` (目前 `wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D`)
   - `Suffix` (目前 `~S`)
5. 对比新旧 sdk.js 差异 (如果 `docs/superpowers/reverse-sessions/` 下有历史版本)
6. 生成差异摘要 `diff-summary.md`

**人工确认**: 差异摘要审阅，确认所有变更点已识别

## Phase 4: 字段映射

**工具**: js-reverse `evaluate_script`

**步骤**:
1. 使用已知字段模式库逐一匹配新数组元素:

| 索引 | 特征签名 | 字段含义 | 验证方式 |
|------|---------|---------|---------|
| [0] | int(2000~8000) | screen.width + screen.height | `screen.width + screen.height` |
| [1] | `"Mon Jan 2 ... GMT..."` | Date.toString() | `new Date().toString()` 格式对比 |
| [2] | 2^31~2^34 | jsHeapSizeLimit | `performance.memory.jsHeapSizeLimit` |
| [3] | 1/2/small int | nonce | 固定 1(prepare) / 递增(proof) |
| [4] | `"Mozilla/5.0..."` | navigator.userAgent | `navigator.userAgent` |
| [5] | `"https://..."` | SDK script URL | 页面 script src |
| [6] | `"prod-*"` 或 null | buildID | `document.documentElement.dataset.build` |
| [7] | 2-5 char 语言码 | navigator.language | `navigator.language` |
| [8] | 逗号分隔语言 | navigator.languages | `navigator.languages.join(",")` |
| [9] | float 0.0~1.0 | Math.random / elapsed | PoW 迭代时递增 |
| [10] | `"xxx−[object Xxx]"` | navigator probe | SDK 硬编码随机选 |
| [11] | `"_react*"` 等 | document random key | SDK 硬编码随机选 |
| [12] | `"onclick"`/`"fetch"` | window random key | SDK 硬编码随机选 |
| [13] | float >= 0 | performance.now() | `performance.now()` 量级对比 |
| [14] | UUID 格式 | device_id | localStorage `oai-did` |
| [15] | 空字符串 | location.search | 通常为空 |
| [16] | 4/8/12/16/24/32 | hardwareConcurrency | `navigator.hardwareConcurrency` |
| [17] | 大浮点数 | performance.timeOrigin | `performance.timeOrigin` 量级对比 |
| [18-24] | 0 或 1 | "X" in window probes (7个) | SDK 指定属性名 |

2. 在浏览器中执行 JS 交叉验证:
```js
() => ({
  screenSum: screen.width + screen.height,
  ua: navigator.userAgent,
  lang: navigator.language,
  langs: Array.from(navigator.languages).join(","),
  jsHeap: performance?.memory?.jsHeapSizeLimit ?? 0,
  hw: navigator.hardwareConcurrency,
  timeOrigin: performance.timeOrigin,
  now: performance.now(),
  deviceMemory: navigator.deviceMemory,
  buildID: document.documentElement.dataset.build,
})
```

3. 自动标注匹配成功的字段
4. 对匹配失败/模糊的字段标记 `⚠️ 需人工确认`
5. 输出最终字段映射表到 `field-mapping.md`

**人工确认**: 审核映射表，特别是 ⚠️ 标记的字段

## Phase 5: 代码修复

**修改文件** (按依赖顺序):

### 5.1 `internal/browserfp/browserfp.go`
- 更新 `DefaultBuildID` 为新版 build-id
- 如有新版 UA 池则追加到 `UserAgents`

### 5.2 `internal/fingerprint/fingerprint.go`
- 更新 `Build25` 函数名 (如数组长度变化则改名)
- 更新字段顺序和 [0]-[24] 注释
- 更新 `NavigatorProbes` 引用 (如 SDK 换新池)
- 更新 `sdkScript` URL 常量
- 检查 Prefix/Suffix/ErrorPrefix 常量是否需要更新

### 5.3 `internal/prooftoken/prooftoken.go`
- 更新 `Config` 结构体 (新增/删除字段)
- 更新 `NewConfig` 硬编码默认值
- 更新 `buildConfig` 中的 fingerprint.Options 映射
- 更新 `GenerateRequirementsToken` / `SolveProofOfWork` 的 token 前缀/后缀常量
- 如有新的 `envFlags` 逻辑则更新

### 5.4 `internal/chatgpt/request.go`
- 更新 `buildSentinelReqToken` 的 fingerprint.Options (语言/屏幕/时区等)
- 更新 `createBaseHeaderForState` 的 `Accept-Language` / `Oai-Language` / `Sec-Ch-Ua` / `Oai-Client-Version`
- 更新 `defaultUserAgent` 返回的新版 UA
- 检查 `POSTSentinelReq` / `POSTSentinelPrepare` 的请求体格式

### 5.5 `internal/turnstile/turnstile.go`
- 更新 `deriveBrowserHints` 的 WebGL renderer/vendor 值
- 检查 DX 脚本 URL 是否变化

### 5.6 `internal/so/so.go`
- 更新 `buildWindow` 中的硬件参数 (Screen/CPU/Heap)

### 5.7 `util/useragent.go`
- 更新 `FixedUserAgent` 常量

**每修改一个文件，展示 diff 预览等待确认。**

## Phase 6: 验证

1. `go build -o aurora.exe` — 编译通过
2. `go vet ./...` — 静态检查通过
3. `go test ./internal/...` — 测试通过 (仅关注由本次变更引起的失败)
4. 输出变更摘要: 修改了哪些文件、更新了哪些常量、新增/删除的字段

## 回滚路径

- Phase 5 开始前自动 `git stash`
- Phase 6 验证失败时提示 `git stash pop` 恢复
- 所有原始抓包数据已保存在资产目录，可随时重新分析

## 资产持久化

每次逆向会话资产保存在:
```
docs/superpowers/reverse-sessions/YYYY-MM-DD/
├── chatgpt-page.html              # 首页 HTML
├── sentinel-prepare-payload.json  # prepare token 解码后的 25 元素数组
├── sentinel-req-payload.json      # req token 解码后的数组
├── sdk.js                         # 新版 sentinel SDK
├── turnstile-dx.js                # Turnstile DX 脚本 (如有独立文件)
├── so-collector.js                # SO collector 脚本 (如有独立文件)
├── field-mapping.md               # 字段映射表 (Phase 4 输出)
└── diff-summary.md                # 新旧差异摘要 (Phase 3 输出)
```

## 已知字段模式库

当前 (2026-06-24) 已知的 25 元素 sentinel config 模式库内置在 Skill 中，作为 Phase 4 自动匹配的基准。Skill 文件维护一个 `fingerprint_patterns` 代码块，记录每个索引的特征签名、字段名、验证方式。每次逆向成功后，Skill 自动更新此模式库以匹配最新协议。
