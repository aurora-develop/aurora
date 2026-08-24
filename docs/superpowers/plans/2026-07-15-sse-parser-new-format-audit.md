# 上游 SSE 新格式解析差距审计

**日期：** 2026-07-15
**状态：** 已确认，暂不修复（存档待办）

## 背景

用户抓取了 2026-08 最新 chatgpt.com `/f/conversation` SSE 流，与我们当前解析器对比发现差异。新格式带 `event: delta` 前缀 + `data:` 行，消息带 `c:N` 递增序号。

## 发现的问题

### 🔴 问题 1：裸补丁数组 `{"v":[...]}` 未解析（正文丢失根因）

新版把多个 patch 打包成顶层裸数组帧（无 p/o 字段）：

```json
data: {"v":[
  {"p":"/message/content/parts/0","o":"append","v":"核心做到尽可能小..."},
  {"p":"/message/metadata/content_references","o":"append","v":[...]}
]}
```

`parseConversationEvent` 分支覆盖情况：
- 标准 patch `{"p":"..","o":"append","v":".."}` → ✅ ApplyPatch
- 批量 patch `{"p":"","o":"patch","v":[...]}` → ✅ 批量分支
- 裸字符串 `{"v":"文本"}` → ✅ L1488
- **裸补丁数组 `{"v":[{p,o,v},...]}`** → ❌ 无分支命中，整帧丢弃

**影响：新版大部分正文增量用此形式，全部丢失 —— 用户遇到的"缺信息"即此。**

修复方向：在 parseConversationEvent 加一个分支：raw["v"] 为 []interface{} 且 raw["p"]==nil 时，遍历数组逐项调 sseparser.ApplyPatch。

### 🟠 问题 2：thinking preamble / commentary 消息泄漏进正文

新格式含"思考前导"消息：

```json
{"v":{"message":{
  "author":{"role":"assistant"},
  "content":{"content_type":"text","parts":["我先确认你说的Pi Agent..."]},
  "metadata":{"is_thinking_preamble_message":true},
  "channel":"commentary"}}}
```

主循环过滤器（request.go:2551-2554）检查 role==assistant、content_type 后缀 text、message_type=next，preamble 三项全过。
未检查 `is_thinking_preamble_message` 和 `channel=commentary`。

**影响：思考前导文本混入给用户的回复。**

修复方向：主循环过滤条件加两个排除：
- `metadata.is_thinking_preamble_message == true` 跳过
- `activeChannel == "commentary"` 跳过（或只允许 final/null/analysis）

能被现有过滤挡住的杂音帧（无需修）：
- c:6 搜索查询（content_type=code）
- c:7/c:8 工具输出（role=tool + text）
- c:10 thoughts（content_type=thoughts）
- c:11 reasoning_recap

### 🟡 问题 3：顶层 type 事件静默忽略

新格式有一批 `data: {"type":"xxx",...}` 事件（无 event: delta）：
- resume_conversation_token / input_message / message_marker / title_generation / url_moderation — 忽略无害
- **message_stream_complete** — 流结束信号，可作终止条件增强健壮性
- **conversation_detail_metadata.limits_progress** — 含 image_gen.remaining 等配额余量
  （正是 docs/superpowers/plans/2026-07-15-image-quota-account-rotation.md 需要的数据源）

### 🟡 问题 4：c:N 序号未利用

每条 delta 带 `"c":N` 递增序号，可用于乱序保护。忽略不影响正确性。

---

## 两种修复方案对比

### 方案 A：最小修补（在现有解析器上加分支）

在 `parseConversationEvent` 加裸数组分支 + 主循环加 preamble/commentary 过滤。

- 优点：
  - 改动小（约 30-50 行），风险低，不动现有架构
  - 与现有 ChunkFromRaw/ApplyPatch 体系完全兼容
  - 快速解决正文丢失和 preamble 泄漏两个最痛问题
- 缺点：
  - parseConversationEvent 的 if-else 分支链继续膨胀，越来越难维护
  - 新格式的事件（type=xxx）依然没有系统性的处理框架，下次上游再变还要打补丁
  - channel 过滤逻辑散落在主循环里，与解析层割裂

### 方案 B：重写 SSE 解析层（状态机 + 事件分发）

新建一个 conversation SSE 状态机模块：
1. 统一入口按帧类型分发：OpenAI chunk / 完整 message / patch（单条+批量+裸数组）/ 顶层 type 事件
2. PatchState 升级为完整文档模型（不只 message，还有 metadata.content_references 等）
3. channel 感知：state 记录当前 message 的 channel/preamble 标记，由状态机统一过滤，主循环只消费干净增量
4. 顶层事件作为一等公民：message_stream_complete → 结束信号；limits_progress → 配额数据源（供未来账号轮换用）

- 优点：
  - 一劳永逸对齐最新格式，后续上游演进只改分发表
  - 正文/thinking/工具/引用天然分离，preamble 泄漏类问题结构性消除
  - 配额信息可直接喂给多账号轮换设计（image-quota-account-rotation.md）
  - 可写完整的单元测试覆盖真实抓包样本
- 缺点：
  - 工作量大（预计 500+ 行新代码 + 迁移），改动面广
  - Handler 主循环与 websocket 路径都要适配，回归风险高
  - 需要充分测试才能上线，短期无法快速止血

### 推荐

先做方案 A 止血（正文丢失是功能性 bug），方案 A 合入并验证稳定后，
再评估是否值得投入方案 B。两者不冲突：A 的补丁逻辑可以原样搬进 B 的分发器。

---

*本文件是审计存档，不追踪修改*
