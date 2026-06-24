---
name: reverse-sentinel
description: 逆向 ChatGPT 网页端 sentinel 指纹协议更新。当 aurora 被风控/路由到 mini 池时调用。分 6 阶段引导式半自动完成抓包→分析→修复→验证。
---

## 触发条件

以下任一症状:
- aurora 请求路由到 mini 池（模型降级）
- sentinel prepare/req/finalize 返回 4xx 或 force_login
- ChatGPT 网页端已知有 UI/功能更新
- 用户手动调用 `/reverse-sentinel`

## 流程概览

```
Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6
前置检查   侦察      网络抓包   SDK分析    字段映射   代码修复   验证
(CDP指纹) (CDP指纹) (js-rvrs)
```

每阶段完成后暂停，展示结果，等待确认后进入下一阶段。

| 阶段 | 自动执行 | 暂停确认 |
|------|---------|---------|
| 0 前置 | MCP 检查、CDP 连接验证、git stash | 未登录时 |
| 1 侦察 | CDP 指纹浏览器提取 build-id/SDK URL | SDK URL |
| 2 抓包 | XHR 断点、抓取 token、解码 base64 | 请求成功确认 |
| 3 SDK分析 | 下载脚本、搜索关键函数 | 差异汇总 |
| 4 字段映射 | 模式匹配 + JS 交叉验证 | **未知字段含义** |
| 5 代码修复 | 逐文件 patch | 每文件 diff 预览 |
| 6 验证 | build + vet + test | 最终确认 |

## Phase 0: 前置检查

### 0.1 MCP 工具可用性

检查以下 MCP 工具集是否在线:
- `js-reverse`: `new_page`, `navigate_page`, `select_page`, `list_network_requests`, `break_on_xhr`, `save_script_source`, `search_in_sources`, `evaluate_script`, `list_scripts`, `get_script_source`

任一不可用 → 提示用户检查 MCP 配置，终止。

> **CDP 指纹浏览器**: 本 Skill 通过 js-reverse 连接到比特指纹浏览器（或其他支持 CDP 端口的指纹浏览器）的 DevTools 调试端口。指纹浏览器已预先登录 chatgpt.com，无需在本 Skill 中处理登录流程。js-reverse 通过 CDP 端口操控已打开的 chatgpt.com 页面。

### 0.2 CDP 指纹浏览器会话验证

比特指纹浏览器已预先登录 chatgpt.com。确认当前页面可用：

```
工具: js-reverse select_page
```

确认当前激活的页面域名为 `chatgpt.com` 且能正常显示 ChatGPT 界面。

页面不可用 → 提示:
> "CDP 指纹浏览器中 chatgpt.com 未打开或无法访问。请在指纹浏览器中打开/刷新 chatgpt.com，确认正常后再重新运行本 Skill。"

### 0.3 会话资产目录

创建资产目录:
```
docs/superpowers/reverse-sessions/YYYY-MM-DD/
```

### 0.4 保护工作区

```bash
git stash
```

保存当前未提交变更，防止逆向过程中的修改意外混合。

> ⚠️ 所有原始抓包数据将保存在资产目录。如需回滚可执行 `git stash pop`。

---

## Phase 1: 侦察 — 获取新版指纹常量

**目标**: 从 chatgpt.com 首页提取 build-id、SDK URL、UA。

### 1.1 确认 CDP 指纹浏览器在 chatgpt.com

CDP 指纹浏览器应已打开并登录 chatgpt.com。使用 `js-reverse select_page` 确认当前页面:

```
工具: js-reverse select_page
```

查看当前激活的页面是否为 `https://chatgpt.com/`。

如果不是 chatgpt.com，导航到它:
```
工具: js-reverse navigate_page
参数: type = "url", url = "https://chatgpt.com/"
```

> 指纹浏览器已登录 ChatGPT，无需重新认证。CDP 操作直接复用浏览器的现有会话。

### 1.2 提取关键常量

执行 JS:
```js
() => ({
  buildID: document.documentElement.dataset.build,
  scripts: Array.from(document.querySelectorAll('script[src]'))
    .map(s => s.src)
    .filter(u => u.includes('sentinel') || u.includes('sdk')),
  ua: navigator.userAgent,
  chromeVersion: navigator.userAgent.match(/Chrome\/(\d+)/)?.[1],
  secCHUA: navigator.userAgentData?.brands,
  allScripts: Array.from(document.querySelectorAll('script[src]'))
    .map(s => s.src)
})
```

提取:
1. **build-id**: `document.documentElement.dataset.build`
2. **SDK URL**: 匹配 `/sentinel/` 或 `sdk.js` 的 script src
3. **User-Agent**: `navigator.userAgent`
4. **Chrome 版本**: 从 UA 中提取 `Chrome/<version>`
5. **Turnstile DX URL**: 匹配 `turnstile` 或 `captcha` 的 script
6. **SO Collector URL**: 匹配 `session` 或 `collector` 的 script

### 1.3 保存首页 HTML

```
工具: js-reverse evaluate_script
```

获取 `document.documentElement.outerHTML`，保存到 `{sessionDir}/chatgpt-page.html`。

### 1.4 确认

展示:
- 新版 BuildID: `______`
- 新版 SDK URL: `______`
- 新版 Chrome 版本: `______`
- Turnstile DX URL: `______` (如有)
- SO Collector URL: `______` (如有)

> 以上信息是否正确？确认后进入 Phase 2。

---

## Phase 2: 网络抓包 — 捕获 sentinel 请求并解码

**目标**: 抓取 sentinel prepare/req/finalize 三个请求，解码 base64 token 还原数组。

### 2.1 设置 XHR 断点

```
工具: js-reverse break_on_xhr
参数: url = "sentinel"
```

捕获所有 URL 包含 "sentinel" 的 XHR/Fetch 请求。

### 2.2 触发 sentinel 流程

在 chatgpt.com 页面通过 JS 发送一条消息，触发完整 sentinel 流程:

```
工具: js-reverse evaluate_script
```

```js
async () => {
  // 找到输入框并填入文本
  const editor = document.querySelector('[contenteditable="true"], #prompt-textarea, textarea');
  if (!editor) return {error: '找不到输入框'};
  editor.focus();
  // 用 ClipboardEvent 或直接 textContent 填入
  if (editor.getAttribute('contenteditable') === 'true') {
    editor.textContent = 'hi';
  } else {
    editor.value = 'hi';
  }
  editor.dispatchEvent(new Event('input', {bubbles: true}));
  // 等待发送按钮可用后点击
  await new Promise(r => setTimeout(r, 500));
  const sendBtn = document.querySelector('[data-testid="send-button"], button[aria-label="Send"]');
  if (sendBtn) { sendBtn.click(); return {sent: true}; }
  // fallback: 模拟 Enter 键
  editor.dispatchEvent(new KeyboardEvent('keydown', {key:'Enter',code:'Enter',keyCode:13,bubbles:true}));
  return {sent: true, fallback: 'Enter key'};
}
```

触发后 sentinel 发出三个请求:
1. `POST /sentinel/chat-requirements/prepare`
2. `POST /sentinel/req`
3. `POST /sentinel/chat-requirements/finalize`

使用 `list_network_requests` 收集这些请求，用 `reqid` 获取完整 request body。

> 如消息发送后未捕获到 sentinel 请求，检查 break_on_xhr 是否生效，或页面是否需要手动通过一次人机验证。

### 2.3 解码 prepare token

对 prepare 请求 body JSON 中的 `p` 字段:
1. 去掉前缀 `gAAAAAC`
2. 去掉后缀 `~S`
3. base64 decode → JSON.parse

```js
() => {
  const token = "gAAAAAC<从抓包中复制>";
  const stripped = token.replace(/^gAAAAAC/, '').replace(/~S$/, '');
  const config = JSON.parse(atob(stripped));
  return { length: config.length, config, types: config.map(v => typeof v) };
}
```

保存到 `{sessionDir}/sentinel-prepare-payload.json`。

### 2.4 解码 req token

同样方法对 `/sentinel/req` 的 body 中 `p` 字段解码。req token 的 `[3]` 通常是 2 (prepare 是 1)。

保存到 `{sessionDir}/sentinel-req-payload.json`。

### 2.5 提取 finalize payload

收集 `/sentinel/chat-requirements/finalize` 的完整 request body。

保存到 `{sessionDir}/sentinel-finalize-payload.json`。

### 2.6 确认

展示:
- prepare config 数组长度: `______`
- req config 数组长度: `______`
- 数组长度是否从之前的 25 变化: `______`
- 元素类型是否变化: `______`

> 确认无误？进入 Phase 3。

---

## Phase 3: SDK 脚本分析 — 下载新版 SDK 并对比差异

**目标**: 下载新版 sentinel SDK 及相关脚本，搜索关键函数，对比新旧差异。

### 3.1 下载新版 SDK

```
工具: js-reverse save_script_source
参数: url = "{Phase 1 提取的 SDK URL}"
      filePath = "{sessionDir}/sdk.js"
      format = true
```

### 3.2 下载 Turnstile DX 脚本 (如有)

```
工具: js-reverse save_script_source
参数: url = "{Turnstile DX URL}"
      filePath = "{sessionDir}/turnstile-dx.js"
      format = true
```

### 3.3 下载 Session Observer 脚本 (如有)

```
工具: js-reverse save_script_source
参数: url = "{SO Collector URL}"
      filePath = "{sessionDir}/so-collector.js"
      format = true
```

### 3.4 搜索关键函数和常量

在 sdk.js 中搜索:
```
工具: js-reverse search_in_sources
搜索以下模式:
  1. "generateRequirementsToken"     → Token 生成入口
  2. "fingerprintSize"               → 数组长度常量
  3. "PrefixRequirements" / "PrefixProof" → Token 前缀
  4. "gAAAAAC" / "gAAAAAB"          → Token 前缀字面值
  5. "FNV1a" / "fnv1a" / "imul"      → PoW 哈希算法
  6. "errorPrefix" / "wQ8Lk5"        → 失败 fallback 前缀
  7. "~S"                            → Token 后缀
  8. "envFlags" / "windowProbes"     → 环境探测
```

### 3.5 提取新版常量值

在浏览器 console 中执行 JS 搜索 SDK 全局常量:
```js
() => {
  // 搜索所有已加载脚本中的关键常量
  const results = {};
  const scripts = Array.from(document.scripts);
  for (const s of scripts) {
    if (!s.text && !s.src.includes('sdk')) continue;
    const txt = s.text || '';
    results.fingerprintSize = txt.match(/fingerprintSize\s*[=:]\s*(\d+)/)?.[1];
    results.prefixReq = txt.match(/["'](gAAAAAC[^"']*)["']/)?.[1];
    results.prefixProof = txt.match(/["'](gAAAAAB[^"']*)["']/)?.[1];
    results.errorPrefix = txt.match(/["'](wQ8Lk5[^"']*)["']/)?.[1];
    if (Object.values(results).some(Boolean)) break;
  }
  return results;
}
```

### 3.6 对比仓库内参考文件

仓库 `chatgpt-turnstile-reversed/` 目录下存有历史参考 JS:
- `enhanced_fp.js` — 925 行，含 ~260 WebGL renderer、~200 downlink、audio/math/canvas 指纹
- `bdaGenerator.js` — 422 行，含 screenRes、canvas_fps、51 字体列表

新抓取的 SDK 与参考文件对比:
```bash
diff chatgpt-turnstile-reversed/enhanced_fp.js {sessionDir}/sdk.js > {sessionDir}/sdk-vs-enhanced_fp.diff
```

### 3.7 对比历史逆向会话

检查 `docs/superpowers/reverse-sessions/` 下是否有历史 sdk.js:
```bash
# 有历史版本 → diff
diff {旧目录}/sdk.js {sessionDir}/sdk.js > {sessionDir}/sdk-diff.txt
# 无历史版本 → 跳过，这是首次逆向基准
```

### 3.8 生成差异摘要

输出 `{sessionDir}/diff-summary.md`:
```markdown
# Sentinal SDK 差异摘要 — YYYY-MM-DD

## 常量变化
| 常量 | 旧值 | 新值 |
|------|------|------|

## 函数变化
- generateRequirementsToken: [描述]

## 新增/删除/重排字段
- [索引]: [描述]
```

### 3.9 确认

> 差异摘要如上。确认进入 Phase 4？

---

## Phase 4: 字段映射 — 自动匹配 + JS 交叉验证

**目标**: 使用模式匹配 + 浏览器 JS 交叉验证确定新版数组每个索引的字段含义。

### 4.1 已知字段模式库

当前 (2026-06-24) 已知的 25 元素模式:

| 索引 | 特征签名 | 字段含义 | 示例值 |
|------|---------|---------|--------|
| [0] | int, 2000~8000 | screen.width + screen.height | 3000 |
| [1] | string, "Mon Jan 2 ... GMT..." 格式 | Date.toString() | "Mon Jun 24 2026 15:04:05 GMT-0700 (PDT)" |
| [2] | int/float, 2e9~2e10 | jsHeapSizeLimit | 4294967296 |
| [3] | int, 1 或 2 或小整数递增 | nonce / attempt | 1 |
| [4] | string, 以 "Mozilla/5.0" 开头 | navigator.userAgent | "Mozilla/5.0 (Windows..." |
| [5] | string, 以 "https://" 开头含 "sdk" | SDK script URL | "https://chatgpt.com/backend-api/sentinel/sdk.js" |
| [6] | string, "prod-*" 格式 或 null | buildID | "prod-2e2e6a5279d8..." |
| [7] | string, 2-5 字符, 含 "-" | navigator.language | "en-US" |
| [8] | string, 逗号分隔语言码 | navigator.languages | "en-US,en" |
| [9] | float, 0~1 或 0~500000 | Math.random / elapsed (ms) | 0.723 |
| [10] | string, 格式 "xxx−[object Xxx]" | navigator property probe | "geolocation−[object Geolocation]" |
| [11] | string, 含 "_react" 或 camelCase | document random own-key | "_reactListening8in7sfyhjvp" |
| [12] | string, 小写事件/API 名 | window random own-key | "requestIdleCallback" |
| [13] | float, >= 0 | performance.now() | 12345.6 |
| [14] | string, UUID 格式 | device_id | "a1b2c3d4-..." |
| [15] | string, 通常为空 | location.search | "" |
| [16] | int, 常见值 4/8/12/16/24/32 | hardwareConcurrency | 16 |
| [17] | float, > 1e12 | performance.timeOrigin | 1719234000000.0 |
| [18] | int, 0 或 1 | "X in window" probe #1 | 0 |
| [19] | int, 0 或 1 | "X in window" probe #2 | 0 |
| [20] | int, 0 或 1 | "X in window" probe #3 | 0 |
| [21] | int, 0 或 1 | "X in window" probe #4 | 0 |
| [22] | int, 0 或 1 | "X in window" probe #5 | 0 |
| [23] | int, 0 或 1 | "X in window" probe #6 | 0 |
| [24] | int, 0 或 1 | "X in window" probe #7 | 0 |

### 4.2 自动模式匹配

对 Phase 2 解码出的每个数组元素:
1. 提取 `typeof` 和值特征
2. 与模式库逐一比对 (类型 + 值域范围 + 字符串格式)
3. 匹配成功 → 标注 ✅
4. 匹配失败 → 标注 ⚠️ "需人工确认"
5. 浮点数歧义: [9] 通常 < 500000, [13] 通常 1000~50000, [17] 通常 > 1e12

### 4.3 JS 浏览器交叉验证

在 chatgpt.com 页面执行:
```js
() => ({
  screenSum: screen.width + screen.height,
  dateStr: new Date().toString(),
  jsHeap: performance?.memory?.jsHeapSizeLimit ?? 0,
  ua: navigator.userAgent,
  lang: navigator.language,
  langs: Array.from(navigator.languages || []).join(","),
  hw: navigator.hardwareConcurrency,
  timeOrigin: performance.timeOrigin,
  now: performance.now(),
  deviceMemory: navigator.deviceMemory ?? 0,
  buildID: document.documentElement.dataset.build,
})
```

将 JS 真实值与抓包值逐一比对:
- 类型匹配 + 量级一致 → ✅ 确认
- 类型匹配但值不同 → ⚠️ 可能是 SDK 覆盖默认值
- 类型/格式不匹配 → ❌ 映射错误

### 4.4 输出映射表

生成 `{sessionDir}/field-mapping.md`:
```markdown
# 字段映射表 — YYYY-MM-DD

| 索引 | 抓包值 | typeof | 模式匹配 | JS 验证 | 状态 |
|------|--------|--------|---------|---------|------|
| 0 | 3000 | number | ✅ screen.w+h | ✅ 3000 | ✅ |
| 1 | "Mon Jun 24..." | string | ✅ Date.toString | 格式一致 | ✅ |
| ... | ... | ... | ... | ... | ... |

## 变化摘要
- 新增字段: [描述]
- 删除字段: [描述]
- 类型变化: [描述]

## ⚠️ 需人工确认
- [索引 N]: 可能含义 A 或 B
```

### 4.5 确认

> 映射表如上。请重点审核 ⚠️ 标记的字段。确认后进入 Phase 5 代码修复。

---

## Phase 5: 代码修复 — 逐一更新 aurora Go 源码

**目标**: 根据 Phase 4 的字段映射表，逐文件修改 Go 源码。

### 5.0 公共规则

**硬编码约定** (所有 `fingerprint.Options` 必须保持 cross-request 一致):
- Language: "en-US"
- Languages: "en-US,en"
- ScreenWidth: 1920
- ScreenHeight: 1080
- HardwareConcurrency: 16
- JSHeapSizeLimit: 4294967296
- BuildID: "{Phase 1 提取的新版 build-id}"
- Timezone: "America/Los_Angeles"
- Platform: "Win32"

**每修改一个文件，展示 `git diff` 预览，等待确认后再改下一个。**

> **编辑提示**: Go 源文件使用 tab 缩进。如 Edit 工具因空白符差异匹配失败，改用 Write 工具重写完整文件，或通过 PowerShell 精确替换。避免 Bash `sed -i`（会破坏 CJK 字符 UTF-8 编码）。

### 5.0b ⚠️ 跨文件一致性验证 (最关键步骤!)

**这是最容易导致"代码改完但仍被风控"的环节。** sentinel 服务器会对比 prepare token (prooftoken.go 生成) 和 req token (request.go 生成) 中的指纹字段 — 任何不一致都会触发风控。

在修改完成后、编译前，逐一比对以下字段在 **prooftoken.go (`buildConfig`)** 和 **request.go (`buildSentinelReqToken`)** 中的值:

| 字段 | 含义 | config 索引 | 必须一致的值 |
|------|------|------------|-------------|
| Language | navigator.language | [7] | `"en-US"` |
| Languages | navigator.languages | [8] | `[]string{"en-US","en"}` |
| ScreenWidth | screen.width | [0] 的一部分 | `1920` |
| ScreenHeight | screen.height | [0] 的一部分 | `1080` |
| HardwareConcurrency | navigator.hardwareConcurrency | [16] | `16` |
| JSHeapSizeLimit | performance.memory.jsHeapSizeLimit | [2] | `4294967296` |
| BuildID | data-build 属性 | [6] | 新版 build-id |
| Timezone | Date.toString() 时区 | [1] 的一部分 | `"America/Los_Angeles"` |
| Platform | navigator.platform | (UA 相关) | `"Win32"` |

**HTTP Header 也需对齐** — `createBaseHeaderForState` 中:
- `Accept-Language` 和 `Oai-Language` → 必须与 fingerprint Language (`"en-US"`) 一致
- `Oai-Client-Version` → 必须与 fingerprint BuildID 一致
- `defaultUserAgent` → 必须与 Phase 1 提取的新版 UA 对齐

> ⚠️ 历史上因 JSHeapSizeLimit / Timezone / Header 语言不一致导致过 3 轮"修复→仍被风控→再排查"的循环。请务必将此表逐项核对后再进入 Phase 6。

### 5.1 `internal/fingerprint/fingerprint.go`

检查项:
1. `Build25` 函数名 → 如数组长度变化，重命名为 `Build{NN}`
2. 注释中 `[0]-[24]` 字段说明 → 对齐新版映射表
3. `config` 切片元素顺序/值 → 对齐新版字段顺序
4. `sdkScript` URL → 如 SDK URL 变化则更新
5. `windowProbes` 数组大小 → 如尾部探测位数量变化则调整

如数组长度从 25→26，新增 [25]:
```go
windowProbes := [8]int{0, 0, 0, 0, 0, 0, 0, 0}
config := []any{
    // ... 现有元素 ...
    windowProbes[6], // [24]
    windowProbes[7], // [25] — 新增
}
```

操作: Read → Edit → 展示 diff → 确认 → 下一个文件

### 5.2 `internal/prooftoken/prooftoken.go`

检查项:
1. `Config` 结构体 — 字段是否需增删
2. `NewConfig` — 硬编码默认值与公共规则对齐
3. `buildConfig` — `fingerprint.Options` 字段映射对齐新版
4. 常量 — `PrefixRequirements` / `PrefixProof` / `Suffix` / `ErrorPrefix` / `DefaultErrorPayload`
5. `SentinelSV` — 如 SDK URL 中 `sv=` 参数变化则更新
6. `GenerateRequirementsToken` — config[3] = 1 逻辑
7. `SolveProofOfWork` — nonce/elapsed 覆盖逻辑

### 5.3 `internal/chatgpt/request.go`

检查项:
1. `buildSentinelReqToken`:
   - `fingerprint.Options` 值与 `prooftoken.NewConfig` 保持完全一致
   - config[3] = 2 (req nonce)
2. `createBaseHeaderForState`:
   - `Accept-Language`: "en-US,en;q=0.9"
   - `Oai-Language`: "en-US"
   - `Sec-Ch-Ua`: Chrome 版本对齐新版 UA
   - `Oai-Client-Version`: 新版 BuildID
3. `defaultUserAgent`: 新版 UA 字符串
4. `POSTSentinelReq` / `POSTSentinelPrepare`: 请求体格式

### 5.4 `internal/turnstile/turnstile.go`

检查项:
1. `deriveBrowserHints` 中 WebGL `webGLVendor` / `webGLRenderer` 硬编码值
2. Turnstile DX 脚本 URL
3. `SolveDX` 签名和逻辑是否受 DX 脚本变化影响

### 5.5 `internal/so/so.go`

检查项:
1. `buildWindow` 硬件参数: `hardwareConcurrency`, `deviceMemory`, `screen.*`, `jsHeapSizeLimit`, `networkRtt/downlink`
2. SO collector/snapshot 脚本 URL

### 5.6 `internal/browserfp/browserfp.go`

检查项:
1. `DefaultBuildID` → 新版 build-id
2. `NavigatorProbes` 池 → SDK 新 probe 则追加
3. `UserAgents` 池 → 新版 Chrome UA 则追加

### 5.7 `util/useragent.go`

检查项:
1. `FixedUserAgent` → 新版 UA 字符串

---

## Phase 6: 验证 — 编译 + 测试

### 6.1 编译

```bash
go build -o aurora.exe
```

✅ 成功 → 继续 | ❌ 失败 → 修复 → 重试 6.1

### 6.2 静态检查

```bash
go vet ./...
```

✅ 无输出 → 继续 | ❌ → 修复 → 重试 6.2

### 6.3 测试

```bash
go test ./internal/...
```

关键包: `fingerprint`, `prooftoken`, `chatgpt`

> ⚠️ `internal/chatgpt` 有 3 个预存在测试失败，与 sentinel 变更无关，可忽略。

### 6.4 变更摘要

```markdown
# /reverse-sentinel 变更摘要 — YYYY-MM-DD

## 修改文件
| 文件 | 变更 | 描述 |
|------|------|------|

## 常量更新
| 常量 | 旧值 | 新值 |
|------|------|------|

## 下一步
1. 启动 aurora 测试
2. 验证 sentinel 流程正常
3. 确认不再路由到 mini 池
```

### 6.5 确认

> 代码修改完成。是否满意？如有问题可 `git stash pop` 回滚。

---

## 回滚

如果修改后仍有问题:
```bash
# 回滚未提交的修改
git stash pop

# 或从 HEAD 恢复指定文件
git checkout HEAD -- internal/fingerprint/fingerprint.go \
  internal/prooftoken/prooftoken.go \
  internal/chatgpt/request.go \
  internal/turnstile/turnstile.go \
  internal/so/so.go \
  internal/browserfp/browserfp.go \
  util/useragent.go
```

抓包数据保留在资产目录，可随时重新分析。

## 资产目录结构

```
docs/superpowers/reverse-sessions/YYYY-MM-DD/
├── chatgpt-page.html              # 首页 HTML (Phase 1)
├── sentinel-prepare-payload.json  # prepare token 解码 (Phase 2)
├── sentinel-req-payload.json      # req token 解码 (Phase 2)
├── sentinel-finalize-payload.json # finalize payload (Phase 2)
├── sdk.js                         # 新版 sentinel SDK (Phase 3)
├── turnstile-dx.js                # Turnstile DX 脚本 (Phase 3)
├── so-collector.js                # SO collector 脚本 (Phase 3)
├── sdk-diff.txt                   # 新旧 SDK diff (Phase 3)
├── diff-summary.md                # 差异摘要 (Phase 3)
└── field-mapping.md               # 字段映射表 (Phase 4)
```

## 排错：修复后仍被风控

如果 Phase 6 编译通过但 aurora 运行时仍被路由到 mini 池，按以下顺序排查：

### 1. prepare/req 指纹不一致 (最常见根因)

对比 aurora 日志中 `/sentinel/chat-requirements/prepare` 和 `/sentinel/req` 两个请求的 `p` 字段解码后的数组。如有差异 → 检查 Phase 5.0b 跨文件一致性表。

### 2. HTTP Header 与指纹不一致

检查 aurora 发出的请求 header:
- `Accept-Language` 是否与 fingerprint config[7] 语言一致
- `Oai-Language` 是否与 fingerprint config[7] 一致
- `Oai-Client-Version` 是否与 fingerprint config[6] (BuildID) 一致
- `User-Agent` 头与 fingerprint config[4] 是否匹配（版本号对齐即可）

### 3. SDK URL 中的 sv= 参数过期

fingerprint config[5] 中的 SDK URL 可能含版本参数 (如 `sv=20260423af3c`)。如 SDK 脚本本身未变但 sv 参数变了 → 只需更新 `SentinelSV` 常量和 config[5] 字符串。

### 4. Turnstile / SO 脚本同步更新

sentinel SDK 更新时，Turnstile DX 脚本和 Session Observer 脚本往往同步更新。检查 `turnstile.go` 中的 `TurnstileDXURL` 和 `so.go` 中的 collector URL 是否需更新。

### 5. 快速验证命令

```bash
# 抓取 aurora 发出的第一个 prepare 请求的 config 数组
# (需在 aurora 日志中开启 DEBUG 级别)
grep "sentinel.*prepare" aurora.log | head -1 | jq '.p' -r | \
  sed 's/^gAAAAAC//;s/~S$//' | base64 -d | jq '.'
```

## 字段模式库维护

逆向成功后，更新本 Skill 中 "### 4.1 已知字段模式库" 章节:
1. 数组长度变化 → 更新条目数
2. 字段重排 → 更新索引顺序
3. 新增字段类型 → 添加新模式条目和特征签名
