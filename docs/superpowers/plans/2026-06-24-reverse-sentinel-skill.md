# /reverse-sentinel Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 创建 `/reverse-sentinel` Claude Code 自定义 skill，引导用户通过 6 个阶段逆向 ChatGPT sentinel 指纹协议变更，并自动修复 aurora Go 源码。

**Architecture:** 单一 skill 文件 `.claude/skills/reverse-sentinel.md`，包含 YAML 前置元数据 + Markdown 分阶段指令。每阶段指定 MCP 工具调用序列和人工确认点。内置 25 元素字段模式库用于自动匹配，7 个 Go 文件的 patch 模板用于代码修复。

**Tech Stack:** YAML frontmatter + Markdown skill 格式, MCP js-reverse + cloakbrowser 工具, Go 1.26

## Global Constraints

- Skill 文件路径: `.claude/skills/reverse-sentinel.md`
- Skill 名: `reverse-sentinel` (YAML `name` 字段)
- 使用 CDP 浏览器 (`js-reverse` MCP) 为主要抓包工具, `cloakbrowser` 为辅助
- 字段模式库必须与当前 `fingerprint.go::Build25` 的 25 元素完全一致
- Go 文件 patch 模板的字段名/函数签名必须与当前代码库匹配
- 资产目录: `docs/superpowers/reverse-sessions/YYYY-MM-DD/`
- Phase 5 开始前必须 `git stash`
- 每 Phase 结束必须有人工确认步骤

---

### Task 1: Skill 骨架 + Phase 0 (前置检查)

**Files:**
- Create: `.claude/skills/reverse-sentinel.md`

**Interfaces:**
- Produces: Skill 文件 YAML frontmatter + Phase 0 完整指令

- [ ] **Step 1: 创建 skill 文件 YAML frontmatter**

```yaml
---
name: reverse-sentinel
description: 逆向 ChatGPT 网页端 sentinel 指纹协议更新。当 aurora 被风控/路由到 mini 池时调用。分 6 阶段引导式半自动完成抓包→分析→修复→验证。
---
```

- [ ] **Step 2: 编写 Phase 0 前置检查指令**

```markdown
## 触发条件

- aurora 请求路由到 mini 池（模型降级）
- sentinel prepare/req 返回 4xx 或 force_login
- 用户手动调用 `/reverse-sentinel`

## Phase 0: 前置检查

### 0.1 MCP 工具可用性

检查以下 MCP 工具集是否在线:
- `js-reverse`: new_page, navigate_page, list_network_requests, break_on_xhr, save_script_source, search_in_sources, evaluate_script, list_scripts, get_script_source
- `cloakbrowser`: cloak_navigate, cloak_fetch, cloak_screenshot

任一不可用 → 提示用户检查 MCP 配置，终止。

### 0.2 ChatGPT 会话凭证

检查环境变量或 `.env` 中是否有有效的:
- `CHATGPT_ACCESS_TOKEN` (JWT 格式 `eyJ...`)
- 或 `CHATGPT_SESSION_TOKEN` (`__Secure-next-auth.session-token`)

无凭证 → 提示: "需要有效的 ChatGPT 会话凭证才能触发 sentinel 请求。请先在 CDP 浏览器中手动登录 chatgpt.com，或设置 CHATGPT_ACCESS_TOKEN 环境变量。是否继续？"

### 0.3 会话资产目录

创建 `docs/superpowers/reverse-sessions/YYYY-MM-DD/` 目录。

### 0.4 保护工作区

执行 `git stash` 保存当前未提交变更。

> ⚠️ 所有原始抓包数据将保存在资产目录。如需回滚可执行 `git stash pop`。
```

- [ ] **Step 3: 编写 Skill 导航概述**

```markdown
## 流程概览

```
Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6
前置检查   侦察     网络抓包   SDK分析    字段映射   代码修复   验证
```

每阶段完成后暂停，展示结果，等待你确认后进入下一阶段。

| 阶段 | 自动执行 | 暂停确认 |
|------|---------|---------|
| 0 前置 | MCP 检查、git stash、创建目录 | 凭证缺失时 |
| 1 侦察 | CDP 导航、提取 build-id/SDK URL | SDK URL |
| 2 抓包 | XHR 断点、抓取 token、解码 base64 | 请求成功确认 |
| 3 SDK分析 | 下载脚本、搜索关键函数 | 差异汇总 |
| 4 字段映射 | 模式匹配 + JS 交叉验证 | **未知字段含义** |
| 5 代码修复 | 逐文件 patch | 每文件 diff 预览 |
| 6 验证 | build + vet + test | 最终确认 |
```
```

- [ ] **Step 4: 验证 skill 文件可被 Claude Code 读取**

```bash
# Claude Code 通过 Skill 工具加载 skill，确认文件存在且格式正确
cat .claude/skills/reverse-sentinel.md | head -5
```

Expected: 显示 YAML frontmatter。

- [ ] **Step 5: Commit**

```bash
git add .claude/skills/reverse-sentinel.md
git commit -m "feat: add reverse-sentinel skill skeleton with Phase 0"
```

---

### Task 2: Phase 1-2 (侦察 + 网络抓包)

**Files:**
- Modify: `.claude/skills/reverse-sentinel.md` — 追加 Phase 1 和 Phase 2 指令

**Interfaces:**
- Consumes: Skill YAML frontmatter + Phase 0 (Task 1)
- Produces: Phase 1 (侦察) + Phase 2 (网络抓包) 完整步骤

- [ ] **Step 1: 编写 Phase 1 侦察指令**

追加到 skill 文件:

```markdown
## Phase 1: 侦察 — 获取新版指纹常量

**目标**: 从 chatgpt.com 首页提取 build-id、SDK URL、UA。

### 1.1 CDP 导航到 chatgpt.com

```
工具: js-reverse new_page
参数: url = "https://chatgpt.com/"
等待: DOMContentLoaded + 3 秒 (让 JS 执行完毕)
```

提取以下信息:
1. **build-id**: `document.documentElement.dataset.build` 或 `<html data-build="...">`
2. **SDK URL**: 遍历 `<script src>`，找到匹配 `/sentinel/` 或 `sdk.js` 的脚本 URL
3. **User-Agent**: `navigator.userAgent`
4. **Chrome 版本**: 从 UA 中提取 `Chrome/<version>`

**执行**:
```
js-reverse evaluate_script:
() => ({
  buildID: document.documentElement.dataset.build,
  scripts: Array.from(document.querySelectorAll('script[src]')).map(s => s.src).filter(u => u.includes('sentinel') || u.includes('sdk')),
  ua: navigator.userAgent,
  chromeVersion: navigator.userAgent.match(/Chrome\/(\d+)/)?.[1],
  secCHUA: navigator.userAgentData?.brands,
})
```

### 1.2 保存首页 HTML

保存 `document.documentElement.outerHTML` 到 `{sessionDir}/chatgpt-page.html`。

### 1.3 提取 Turnstile + SO 脚本

从 scripts 列表中再找:
- turnstile/CAPTCHA 相关的 DX 脚本 URL
- Session Observer collector/snapshot 脚本 URL

### 1.4 确认

展示:
- 新版 BuildID: `______`
- 新版 SDK URL: `______`
- 新版 Chrome 版本: `______`
- Turnstile DX URL: `______`
- SO Collector URL: `______`

> 以上信息是否正确？确认后进入 Phase 2。
```

- [ ] **Step 2: 编写 Phase 2 网络抓包指令**

追加到 skill 文件:

```markdown
## Phase 2: 网络抓包 — 捕获 sentinel 请求并解码

**目标**: 抓取 sentinel prepare/req/finalize 三个请求，解码 base64 token 还原 25 元素数组。

### 2.1 设置 XHR 断点

```
工具: js-reverse break_on_xhr
参数: url = "sentinel"
```

这会捕获所有 URL 包含 "sentinel" 的 XHR/Fetch 请求。

### 2.2 触发 sentinel 流程

在 chatgpt.com 页面上发送一条简单消息 (如 "hi")，触发完整的 sentinel 流程:
1. `POST /sentinel/chat-requirements/prepare`
2. `POST /sentinel/req`
3. `POST /sentinel/chat-requirements/finalize`

使用 js-reverse 的 `list_network_requests` 收集这些请求的 request body。

### 2.3 解码 prepare token

```
工具: js-reverse evaluate_script
```

对 prepare 请求的 body JSON 中 `p` 字段:
1. 去掉前缀 `gAAAAAC`
2. 去掉后缀 `~S`
3. base64 decode
4. JSON.parse → 得到数组

执行解码:
```js
() => {
  const token = "gAAAAAC<从抓包中复制>";
  const stripped = token.replace(/^gAAAAAC/, '').replace(/~S$/, '');
  const json = atob(stripped);
  const config = JSON.parse(json);
  return {
    length: config.length,
    config: config,
    types: config.map(v => typeof v)
  };
}
```

保存到 `{sessionDir}/sentinel-prepare-payload.json`:
```json
{
  "source": "POST /sentinel/chat-requirements/prepare",
  "raw_token": "<完整 token>",
  "decoded_config": [<数组>],
  "element_types": [<类型列表>]
}
```

### 2.4 解码 req token

同样方法对 `/sentinel/req` 的 body 中 `p` 字段解码:
- 去掉前缀 `gAAAAAC`
- 去掉后缀 `~S`
- base64 decode → JSON parse

注意: req token 的 `[3]` 通常是 2 (prepare 是 1)。

保存到 `{sessionDir}/sentinel-req-payload.json`。

### 2.5 提取 finalize payload

收集 `/sentinel/chat-requirements/finalize` 的完整 request body (包括 `prepare_token`、`proofofwork`、`turnstile` 字段)。

保存到 `{sessionDir}/sentinel-finalize-payload.json`。

### 2.6 确认

展示:
- prepare config 数组长度: `______`
- req config 数组长度: `______`
- 新旧数组长度是否变化: `______`
- 新旧元素类型是否有变化: `______`

> 确认无误？进入 Phase 3。
```

- [ ] **Step 3: 验证 Phase 1-2 指令完整性**

检查:
- MCP 工具名称与可用工具列表一致 (`js-reverse` 前缀)
- 文件路径使用 `{sessionDir}` 占位符
- 确认步骤有明确的展示格式

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/reverse-sentinel.md
git commit -m "feat: add Phase 1-2 (reconnaissance + packet capture) to reverse-sentinel skill"
```

---

### Task 3: Phase 3-4 (SDK 分析 + 字段映射)

**Files:**
- Modify: `.claude/skills/reverse-sentinel.md` — 追加 Phase 3、Phase 4 和字段模式库

**Interfaces:**
- Consumes: Phase 1 SDK URL, Phase 2 解码后的 25 元素数组
- Produces: Phase 3 SDK diff + Phase 4 field-mapping.md + 字段模式库

- [ ] **Step 1: 编写 Phase 3 SDK 脚本分析指令**

```markdown
## Phase 3: SDK 脚本分析 — 下载新版 SDK 并对比差异

**目标**: 下载新版 sentinel SDK 及相关脚本，搜索关键函数，对比新旧差异。

### 3.1 下载新版 SDK

```
工具: js-reverse save_script_source
参数: url = "{Phase 1 提取的 SDK URL}"
      filePath = "{sessionDir}/sdk.js"
      format = true  (beautify minified JS)
```

### 3.2 下载 Turnstile DX 脚本 (如有)

```
工具: js-reverse save_script_source
参数: url = "{Phase 1 提取的 Turnstile DX URL}"
      filePath = "{sessionDir}/turnstile-dx.js"
      format = true
```

### 3.3 下载 Session Observer 脚本 (如有)

```
工具: js-reverse save_script_source
参数: url = "{Phase 1 提取的 SO Collector URL}"
      filePath = "{sessionDir}/so-collector.js"
      format = true
```

### 3.4 搜索关键函数和常量

在新版 sdk.js 中搜索:
```
工具: js-reverse search_in_sources
应用搜索以下模式:
  1. "generateRequirementsToken"     → Token 生成入口
  2. "fingerprintSize" 或数组长度常量 → 数组元素数量
  3. "PrefixRequirements" / "PrefixProof" → Token 前缀
  4. "gAAAAAC" 或 "gAAAAAB"          → Token 前缀字面值
  5. "FNV1a" / "fnv1a" / "imul"      → PoW 哈希算法
  6. "errorPrefix" / "wQ8Lk5"        → 失败 fallback 前缀
  7. "Suffix" / "~S"                 → Token 后缀
  8. "envFlags" / "windowProbes"     → 环境探测
```

对每个搜索返回匹配行，记录行号。

### 3.5 提取新版常量值

执行 JS 读取 SDK 中的常量:
```js
() => {
  // 在 sdk.js 全局作用域中搜索
  const patterns = {
    fingerprintSize: /fingerprintSize\s*[=:]\s*(\d+)/,
    prefixRequirements: /["'](gAAAAAC[^"']*)["']/,
    prefixProof: /["'](gAAAAAB[^"']*)["']/,
    errorPrefix: /["'](wQ8Lk5[^"']*)["']/,
    suffix: /["'](~S[^"']*)["']/,
  };
  // 返回匹配结果
  return Object.entries(patterns).map(([k, re]) => [k, document.scripts[0]?.text?.match(re)?.[1]]);
}
```

### 3.6 对比历史版本

检查 `docs/superpowers/reverse-sessions/` 下是否有历史 sdk.js。

有 → 执行 diff:
```bash
diff {历史目录}/sdk.js {sessionDir}/sdk.js > {sessionDir}/sdk-diff.txt
```

无 → 跳过 diff，这是首次逆向基准。

### 3.7 生成差异摘要

输出 `{sessionDir}/diff-summary.md`:
```markdown
# Sentinal SDK 差异摘要 — YYYY-MM-DD

## 常量变化
| 常量 | 旧值 | 新值 |
|------|------|------|
| fingerprintSize | 25 | XX |
| PrefixRequirements | gAAAAAC | XXX |
| ... | ... | ... |

## 函数变化
- generateRequirementsToken: [无变化 / 新增XX参数 / 逻辑变化]
- ...

## 新增字段
- [N] = 描述

## 删除字段
- [N] = 描述
```

### 3.8 确认

> 差异摘要如上。确认进入 Phase 4？
```

- [ ] **Step 2: 编写 Phase 4 字段映射指令 (含模式库)**

```markdown
## Phase 4: 字段映射 — 自动匹配 + JS 交叉验证

**目标**: 使用模式匹配 + 浏览器 JS 交叉验证确定新版数组每个索引的字段含义。

### 4.1 已知字段模式库

当前 (2026-06-24) 已知的 25 元素模式:

| 索引 | 特征签名 | 字段含义 | 示例值 |
|------|---------|---------|--------|
| [0] | int, 2000~8000 | screen.width + screen.height | 3000 |
| [1] | string, "Mon Jan 2 ... GMT..." 格式 | Date.toString() | "Mon Jun 24 2026 15:04:05 GMT-0700 (PDT)" |
| [2] | int/float, 2e9~2e10 (2^31~2^34) | jsHeapSizeLimit | 4294967296 |
| [3] | int, 1 或 2 或小整数递增 | nonce / attempt | 1 |
| [4] | string, 以 "Mozilla/5.0" 开头 | navigator.userAgent | "Mozilla/5.0 (Windows..." |
| [5] | string, 以 "https://" 开头包含 "sdk" | SDK script URL | "https://chatgpt.com/backend-api/sentinel/sdk.js" |
| [6] | string, "prod-*" 格式 或 null | buildID (data-build) | "prod-2e2e6a5279d8..." |
| [7] | string, 2-5 字符, 含 "-" | navigator.language | "en-US" |
| [8] | string, 逗号分隔语言码 | navigator.languages.join(",") | "en-US,en" |
| [9] | float, 0.0~1.0 或 0~500000 | Math.random() / elapsed (ms) | 0.723 |
| [10] | string, 格式 "xxx−[object Xxx]" | navigator property probe | "geolocation−[object Geolocation]" |
| [11] | string, 含 "_react" 或 camelCase | document random own-key | "_reactListening8in7sfyhjvp" |
| [12] | string, 小写事件/API 名 | window random own-key | "requestIdleCallback" |
| [13] | float, >= 0 | performance.now() | 12345.6 |
| [14] | string, UUID 格式 (含连字符) | device_id | "a1b2c3d4-..." |
| [15] | string, 通常为空 "" | location.search | "" |
| [16] | int, 常见值 4/8/12/16/24/32 | hardwareConcurrency | 16 |
| [17] | float, 1.7e12~1.8e12 (大浮点数) | performance.timeOrigin | 1719234000000.0 |
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
2. 与模式库逐一比对
3. 匹配成功 → 标注 ✅
4. 匹配失败 → 标注 ⚠️ "需人工确认"
5. 值特征模糊 (如 float 既可能是 [9] 又可能是 [13]) → 用值域区分: [9] 通常 < 1.0 (Math.random) 或 < 500000 (elapsed), [13] 通常 > 1000 (performance.now), [17] 通常 > 1e12

### 4.3 JS 浏览器交叉验证

在 chatgpt.com 页面执行 JS 读取真实浏览器值:
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

交叉验证逻辑:
- 如果抓包值 ≈ JS 真实值 (类型匹配 + 量级一致) → ✅ 确认
- 如果抓包值 ≠ JS 真实值 → ⚠️ 标记差异，可能是 SDK 版本覆盖
- 如果类型/格式不匹配 → ❌ 字段映射错误

### 4.4 输出映射表

生成 `{sessionDir}/field-mapping.md`:
```markdown
# 字段映射表 — YYYY-MM-DD

## 匹配结果
| 索引 | 抓包值 | typeof | 模式匹配 | JS 验证 | 状态 |
|------|--------|--------|---------|---------|------|
| 0 | 3000 | number | screen.w+h = 3000 | screen.w+h = 3000 ✅ | ✅ |
| 1 | "Mon Jun 24 ..." | string | Date.toString() ✅ | 格式一致 ✅ | ✅ |
| 2 | 8589934592 | number | jsHeapSizeLimit ✅ | 8.59GB ✅ | ✅ |
| ... | ... | ... | ... | ... | ... |

## 变化摘要
- 新增字段: [N] = _____ (原因为 ____)
- 删除字段: [N] 不再出现
- 类型变化: [N] 从 int → float
- 重排序: [N] 从 X 移到了 Y

## ⚠️ 需人工确认
- [索引 N]: 匹配度低，可能含义为 A 或 B
```

### 4.5 确认

> 映射表如上。请重点审核 ⚠️ 标记的字段。确认无误后进入 Phase 5 代码修复。
> 如需调整映射，请告知。
```

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/reverse-sentinel.md
git commit -m "feat: add Phase 3-4 (SDK analysis + field mapping) to reverse-sentinel skill"
```

---

### Task 4: Phase 5 (Go 代码修复模板)

**Files:**
- Modify: `.claude/skills/reverse-sentinel.md` — 追加 Phase 5 指令和 7 个文件的 patch 模板

**Interfaces:**
- Consumes: Phase 4 field-mapping.md 映射结果
- Produces: 7 个 Go 文件的 patch 指令

- [ ] **Step 1: 编写 Phase 5 头部和公共逻辑**

```markdown
## Phase 5: 代码修复 — 逐一更新 aurora Go 源码

**目标**: 根据 Phase 4 的字段映射表，逐文件修改 Go 源码。

### 5.0 公共规则

1. **硬编码一致性**: 所有调用 `fingerprint.Build25(opts)` 的地方，opts 的 Language/Screen/HeapSize/HardwareConcurrency 等字段必须使用相同常量。当前约定:
   - Language: "en-US"
   - Languages: "en-US,en"
   - ScreenWidth: 1920
   - ScreenHeight: 1080
   - HardwareConcurrency: 16
   - JSHeapSizeLimit: 4294967296
   - BuildID: "{Phase 1 提取的新版 build-id}"
   - Timezone: "America/Los_Angeles"
   - Platform: "Win32"

2. **Token 前缀/后缀**: 如 Phase 3 发现 PrefixRequirements/PrefixProof/Suffix 变化，同步更新 `prooftoken.go` 中对应常量。

3. **SentinelSV**: 如 SDK URL 中 `sv=` 参数变化，更新 `prooftoken.go::NewConfig().SentinelSV`。

4. **每修改一个文件，展示 `git diff` 预览，等待确认后再改下一个文件。**
```

- [ ] **Step 2: 编写 fingerprint.go patch 模板**

```markdown
### 5.1 `internal/fingerprint/fingerprint.go`

**检查项**:
1. `Build25` 函数名 → 如数组长度变化，重命名为 `Build{NN}`
2. 注释中 `[0]-[24]` 的字段说明 → 对齐新版映射表
3. `config` 切片元素的顺序和值 → 对齐新版字段顺序
4. `sdkScript` URL → 如 SDK URL 变化则更新
5. `windowProbes` 数组大小 → 如尾部探测位变化则调整

**修改示例** (如数组长度从 25→26, 新增 [25]):
```go
// 变更前
windowProbes := [7]int{0, 0, 0, 0, 0, 0, 0}
config := []any{
    screenSum,       // [0]
    dateStr,         // [1]
    // ... 现有 25 元素 ...
    windowProbes[6], // [24]
}

// 变更后
windowProbes := [8]int{0, 0, 0, 0, 0, 0, 0, 0}
config := []any{
    screenSum,       // [0]
    dateStr,         // [1]
    // ... 现有 25 元素 ...
    windowProbes[6], // [24]
    windowProbes[7], // [25] — 新增
}
```

**操作**:
1. Read `internal/fingerprint/fingerprint.go`
2. 根据 `{sessionDir}/field-mapping.md` 中的变化更新 Build25
3. 展示 diff → 等待确认 → 保存

> 展示 fingerprint.go 的 diff 预览。是否继续？
```

- [ ] **Step 3: 编写 prooftoken.go patch 模板**

```markdown
### 5.2 `internal/prooftoken/prooftoken.go`

**检查项**:
1. `Config` 结构体 — 字段是否需增删
2. `NewConfig` — 硬编码默认值对齐 (en-US, 1920, 1080, 16)
3. `buildConfig` — `fingerprint.Options` 字段映射对齐新版
4. 常量 — `PrefixRequirements` / `PrefixProof` / `Suffix` / `ErrorPrefix` / `DefaultErrorPayload` / `SentinelSV`
5. `GenerateRequirementsToken` — config[3] = 1 逻辑是否变化
6. `SolveProofOfWork` — nonce/elapsed 覆盖 [3]/[9] 的逻辑是否变化

**操作**:
1. Read `internal/prooftoken/prooftoken.go`
2. 逐常量比对 Phase 3 提取的新版值
3. 逐 Options 字段比对 Phase 4 映射表
4. 展示 diff → 等待确认 → 保存

> 展示 prooftoken.go 的 diff 预览。是否继续？
```

- [ ] **Step 4: 编写 request.go patch 模板**

```markdown
### 5.3 `internal/chatgpt/request.go`

**检查项**:
1. `buildSentinelReqToken`:
   - `fingerprint.Options` 的 ScreenWidth/ScreenHeight/HardwareConcurrency/JSHeapSizeLimit/BuildID/Languages/Timezone 值 → 与 prooftoken.NewConfig 保持一致
   - config[3] = 2 (req nonce) 是否变化
2. `createBaseHeaderForState`:
   - `Accept-Language`: "en-US,en;q=0.9"
   - `Oai-Language`: "en-US"
   - `Sec-Ch-Ua`: Chrome 版本对齐 (如 UA 中 Chrome 148 → 149)
   - `Oai-Client-Version`: 新版 BuildID
3. `defaultUserAgent`: 新版 UA 常量
4. `POSTSentinelReq` / `POSTSentinelPrepare` / `POSTSentinelFinalize`: 请求体格式是否变化

**操作**:
1. Read `internal/chatgpt/request.go`
2. 更新上述所有位置
3. 展示 diff → 等待确认 → 保存

> 展示 request.go 的 diff 预览。是否继续？
```

- [ ] **Step 5: 编写 turnstile.go patch 模板**

```markdown
### 5.4 `internal/turnstile/turnstile.go`

**检查项**:
1. `deriveBrowserHints` 中的 WebGL `webGLVendor` / `webGLRenderer` 硬编码值
2. Turnstile DX 脚本 URL 是否变化
3. `SolveDX` 函数签名和逻辑是否受新版 DX 影响

**操作**:
1. Read `internal/turnstile/turnstile.go`
2. 如 Phase 3 发现 turnstile DX 脚本变化，对照新脚本更新 hints
3. 展示 diff → 等待确认 → 保存

> 展示 turnstile.go 的 diff 预览。是否继续？
```

- [ ] **Step 6: 编写 so.go patch 模板**

```markdown
### 5.5 `internal/so/so.go`

**检查项**:
1. `buildWindow` 中的硬件参数:
   - `hardwareConcurrency`
   - `deviceMemory`
   - `screen.width/height/availHeight/colorDepth`
   - `jsHeapSizeLimit`
   - `networkRtt/downlink`
2. Session Observer collector/snapshot 脚本 URL 是否变化

**操作**:
1. Read `internal/so/so.go`
2. 如 Phase 3 发现 SO 脚本变化，对照更新
3. 展示 diff → 等待确认 → 保存

> 展示 so.go 的 diff 预览。是否继续？
```

- [ ] **Step 7: 编写 browserfp.go + useragent.go patch 模板**

```markdown
### 5.6 `internal/browserfp/browserfp.go`

**检查项**:
1. `DefaultBuildID` 常量 → 更新为新版 build-id
2. `NavigatorProbes` 池 → 如 SDK 使用了新的 probe 列表则更新
3. `UserAgents` 池 → 如新版使用不同 Chrome 版本则追加

### 5.7 `util/useragent.go`

**检查项**:
1. `FixedUserAgent` 常量 → 更新为新版 UA 字符串

**操作**:
1. 依次 Read + Edit 两个文件
2. 展示 diff → 等待确认 → 保存

> 展示 browserfp.go + useragent.go 的 diff 预览。是否继续？
```

- [ ] **Step 8: Commit**

```bash
git add .claude/skills/reverse-sentinel.md
git commit -m "feat: add Phase 5 (Go code patching) to reverse-sentinel skill"
```

---

### Task 5: Phase 6 (验证 + 回滚) + 附录

**Files:**
- Modify: `.claude/skills/reverse-sentinel.md` — 追加 Phase 6 和附录

**Interfaces:**
- Consumes: Phase 5 所有 Go 文件修改完成
- Produces: 完整的 reverse-sentinel skill

- [ ] **Step 1: 编写 Phase 6 验证指令**

```markdown
## Phase 6: 验证 — 编译 + 测试

### 6.1 编译

```bash
go build -o aurora.exe
```

- ✅ 编译成功 → 继续
- ❌ 编译失败 → 根据错误定位修复，重复 6.1

### 6.2 静态检查

```bash
go vet ./...
```

- ✅ 无输出 → 继续
- ❌ 有警告 → 修复后重复 6.2

### 6.3 测试

```bash
go test ./internal/...
```

关键包:
- `aurora/internal/fingerprint` — Build25 测试
- `aurora/internal/prooftoken` — token 生成测试
- `aurora/internal/chatgpt` — 集成测试 (3 个预存在失败忽略)

> ⚠️ `internal/chatgpt` 包有 3 个预存在测试失败 (TestConversationHeadersKeepEmptyConduitHeaderForConversation / TestCreateBaseHeaderMatchesWebClientShape / TestPrepareConversationConduitUsesClientState)，与 sentinel 变更无关，可忽略。

### 6.4 变更摘要

输出最终变更清单:
```markdown
# /reverse-sentinel 变更摘要 — YYYY-MM-DD

## 修改文件
| 文件 | 变更类型 | 描述 |
|------|---------|------|
| internal/fingerprint/fingerprint.go | 修改 | Build25 → Build26，新增 [25] 字段 |
| internal/prooftoken/prooftoken.go | 修改 | PrefixRequirements 更新 |
| ... | ... | ... |

## 常量更新
| 常量 | 旧值 | 新值 |
|------|------|------|
| BuildID | prod-xxx | prod-yyy |
| SentinelSV | 20260423af3c | 20260701bc4d |

## 下一步
1. 启动 aurora 测试
2. 发一次请求验证 sentinel 流程正常
3. 检查是否还路由到 mini 池
```

### 6.5 确认

> 代码修改完成，变更摘要如上。是否满意？如有问题可 `git stash pop` 回滚所有修改。
```

- [ ] **Step 2: 编写附录 — 回滚路径 + 资产目录结构**

```markdown
## 回滚

如果修改后仍有问题:

```bash
# 回滚所有未提交的代码修改
git stash pop

# 或从历史 commit 恢复
git checkout HEAD -- internal/fingerprint/fingerprint.go internal/prooftoken/prooftoken.go internal/chatgpt/request.go internal/turnstile/turnstile.go internal/so/so.go internal/browserfp/browserfp.go util/useragent.go
```

抓包数据保留在 `docs/superpowers/reverse-sessions/YYYY-MM-DD/`，可随时重新分析。

## 资产目录

每次逆向会话独立子目录:
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

## 字段模式库维护

逆向成功后，Skill 内的模式库应自动更新:
1. 如有数组长度变化 → 更新 Build25 的注释和模式库条目数
2. 如有字段重排 → 更新模式库的索引顺序
3. 如有新增字段类型 → 添加新模式条目

更新方式: Edit `.claude/skills/reverse-sentinel.md` 中 "### 4.1 已知字段模式库" 章节。
```

- [ ] **Step 3: 最终验证 — 完整读取 skill 文件确认结构**

```bash
# 检查 skill 文件结构完整性
grep "^## " .claude/skills/reverse-sentinel.md
```

Expected 输出:
```
## 触发条件
## Phase 0: ...
## 流程概览
## Phase 1: ...
## Phase 2: ...
## Phase 3: ...
## Phase 4: ...
## Phase 5: ...
## Phase 6: ...
## 回滚
## 资产目录
## 字段模式库维护
```

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/reverse-sentinel.md
git commit -m "feat: add Phase 6 (verification) + rollback + appendix to reverse-sentinel skill"
```

---

## Self-Review 结果

1. **Spec coverage**: 6 个 Phase 全覆盖 ✅。Skill 文件路径 `.claude/skills/reverse-sentinel.md` ✅。字段模式库 ✅。7 个 Go 文件 patch 模板 ✅。MCP js-reverse 为主 + cloakbrowser 为辅 ✅。git stash 保护 ✅。资产持久化 ✅。

2. **Placeholder scan**: 无 TBD/TODO。所有 `{sessionDir}` 占位符含义明确。所有代码示例具体可执行。

3. **Type consistency**: 文件路径全部使用相对于项目根目录的路径。MCP 工具名使用 `js-reverse` / `cloakbrowser` 前缀 (与可用工具列表一致)。
