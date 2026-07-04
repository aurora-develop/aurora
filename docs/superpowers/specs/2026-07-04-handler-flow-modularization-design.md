# Handler Flow 模块化设计

**Date:** 2026-07-04  
**Status:** In Design  
**Goal:** 优先提升新接口扩展效率，将 `initialize/handlers.go` 中可复用的业务编排逻辑下沉到 `internal/...` 模块中全局复用。

## 背景

当前项目的主要复杂度集中在 `initialize/handlers.go`：同一个 handler 同时承担了 HTTP 请求绑定、鉴权解析、账号选择、上游 ChatGPT 请求编排、continue 循环、SSE 输出、结果转换与错误返回等职责。

这导致两个问题：

1. 新增接口时，需要重复拼装整条业务链，而不是复用稳定模块。
2. 相似能力之间已经出现明显重复，但复用点仍停留在 handler 文件内部，难以在全局统一使用。

本次设计目标不是先做一轮泛工具化，而是优先抽取“完整业务编排模块”，让 handler 回到协议层，复用点落在 `internal/...`。

## 设计目标

1. 保持现有仓库风格，新增模块仍放在 `internal/...`，不引入新的顶层 `services/` 目录。
2. 优先抽取对新增接口最有帮助的 flow / orchestrator 模块，而不是先收纳零散 util。
3. 明确职责分层：
   - `initialize/*` 负责 HTTP 协议层
   - `internal/<domain>flow` 负责业务编排层
   - `internal/chatgpt`、`internal/tokens`、`internal/proxys`、`httpclient` 保持底层依赖职责
4. 保证模块化迁移是渐进式的，不要求一次性重写所有 handler。

## 推荐架构

推荐演进为如下三层结构：

```text
client
  -> gin router / initialize handlers
  -> internal/<domain>flow
  -> internal/chatgpt | internal/tokens | internal/proxys | httpclient
```

其中：

- `initialize` 只处理 HTTP 形态问题：`BindJSON`、`MultipartForm`、header/query/form 提取、状态码返回、OpenAI 协议响应。
- `internal/<domain>flow` 负责“为了完成某个业务能力，需要按什么顺序调用哪些底层组件”。
- `internal/chatgpt` 继续只负责与 ChatGPT Web 上游交互，不承担更高层的 API 编排职责。

## 模块划分

### 1. `internal/conversationflow`

负责 conversation 类能力的公共执行链，覆盖当前这些入口背后的共享流程：

- `/v1/chat/completions` (`nightmare`)
- `/v1/responses` (`responses`)
- TTS 前置 conversation (`tts`)
- tool-calling conversation 执行链

它应当统一封装以下逻辑：

- 获取或创建 `ChatClientState`
- 调用 `postConversationGptClientOrder`
- 处理 continue loop
- 累积 `full_response` / `thinking` / `sentinel`
- 维护 `conversation_id` / `parent_message_id`
- 注册 `SessionManager`
- 处理 tool-calling 的独立执行分支
- 为 stream / non-stream 响应提供统一结果模型

这部分是第一优先级，因为 `nightmare`、`responses`、`tts` 当前都在重复 conversation orchestration，只是输入输出包装不同。

### 2. `internal/imageflow`

负责图片生成、改图、变体相关的编排逻辑。

建议拆成两个清晰子职责：

- **request normalization**：把 JSON / multipart / Responses-style 图像输入统一成内部结构
- **generation pipeline**：上传源图、调用图片生成上游、做结果转换与错误处理

它应吸收当前 `imageGenerations` 与 `runImageEditFlow` 中共享的这些逻辑：

- `stream` 判定与公共行为
- 图片结果转 `b64_json` / `url`
- URL 下载再转 base64
- 空结果判定
- 图片上传循环
- 上游返回结果的统一整理

该模块目标是让后续新增图片类接口时，可以直接复用 normalization + generation，而不需要在 handler 中重写整条流程。

### 3. `internal/audioflow`

负责音频相关能力的业务编排，初期包含两个入口：

- `Synthesize(...)`：对应 TTS
- `Transcribe(...)`：对应 transcription / translation

这里不强行把两条底层能力统一成单一抽象，但应放入同一业务域包，统一管理：

- paid token 校验要求
- 默认参数归一化
- MIME / 输出格式映射
- 上游调用前后的公共准备与结果收口

目标是让以后继续增加音频能力时，不再把协议外逻辑继续塞回 handler。

### 4. `internal/authresolver`

负责从请求上下文、环境配置和账号池中解析最终使用的 secret。

当前这些逻辑已经构成一个独立子系统，只是仍放在 handler 文件中：

- `secretFromAuthorization`
- `accessTokenFromRefreshToken`
- `authorizationTokenAndTeam`
- `teamAccountIDFromRequest`
- `splitAuthorizationTokenAndTeam`

该模块应统一处理：

- service authorization 与真实用户 token 的分流
- access token / refresh token / UUID / team account 的解析
- paid / free 账号选择策略
- 状态码和错误信息的规范输出基础

由于几乎所有能力都依赖 secret 解析，因此这是第一批必须抽离的模块。

### 5. `internal/httpstream`

负责 SSE / 流式协议写出细节，不承担业务判断。

初期可收纳这些职责：

- 写入通用 SSE header
- 写入 `event + data`
- 写入 `[DONE]`
- 写入 chat completion 的 stop chunk / done chunk

该模块优先级低于前四项，因为它主要帮助清理 handler 和减少协议重复，不直接决定新能力的扩展效率。但在 flow 模块边界稳定后，适合补上，避免协议细节散落在多个 handler 中。

## 接口建议

### `internal/conversationflow`

建议提供高层执行入口，而不是暴露过多细碎步骤。典型形态如下：

```go
type ExecuteRequest struct {
    OriginalRequest officialtypes.APIRequest
    Stream          bool
    ProxyURL        string
    Secret          *tokens.Secret
    Client          *bogdanfinn.TlsClient
    SessionManager  SessionRegistry
}

type ExecuteResult struct {
    Text           string
    ThinkingText   string
    ConversationID string
    Sentinel       []map[string]interface{}
    InputTokens    int
    OutputTokens   int
    StopSent       bool
}
```

建议暴露的入口包括：

- `ExecuteChat(...)`
- `ExecuteResponses(...)`
- `ExecuteToolCalling(...)`
- `ExecuteTTSConversation(...)`

handler 只负责准备输入，并把 `ExecuteResult` 转换为 OpenAI 形态响应。

### `internal/imageflow`

建议区分请求归一化与执行：

```go
type NormalizedImageRequest struct {
    Prompt         string
    Model          string
    ResponseFormat string
    N              int
    Stream         bool
    Sources        []ImageSource
    Variation      bool
}
```

公开入口建议为：

- `NormalizeEditRequest(...)`
- `NormalizeGenerationRequest(...)`
- `Generate(...)`

这样 handler 可以保留 HTTP 输入解析的薄层职责，而业务执行由 flow 模块统一完成。

### `internal/audioflow`

建议公开两个直接入口：

- `Synthesize(...)`
- `Transcribe(...)`

不要一开始设计过泛的音频总线，只需要把当前重复的业务编排先收敛起来。

### `internal/authresolver`

建议提供一个以“解析最终 secret”为核心的入口，例如：

```go
type ResolveRequest struct {
    NeedsPaid         bool
    AllowFallbackPaid bool
    ProxyURL          string
}

type ResolveResult struct {
    Secret *tokens.Secret
}
```

具体实现可继续依赖现有账号池结构，但 handler 不再自行关心 refresh token、UUID、team account 等分支细节。

### `internal/httpstream`

建议保持极轻接口：

- `WriteSSEHeader(...)`
- `WriteSSEEvent(...)`
- `WriteDone(...)`
- `WriteChatCompletionDone(...)`

它只负责“怎么写”，不负责“什么时候写”。

## 推荐实施顺序

### 第一步：`internal/authresolver` + `internal/conversationflow`

这是收益最高的一步。

原因：

- conversation 是当前最复杂、重复最多的主链
- auth 解析几乎影响所有端点
- 抽完这两个，`initialize/handlers.go` 的主复杂度会显著下降

### 第二步：`internal/imageflow`

图片相关能力已经接近一个完整子系统，只是还嵌在 handler 中。把它单独抽出后，后续新增图片类 API 的复用收益最高。

### 第三步：`internal/audioflow`

音频能力重复量次于 conversation / image，但抽出后边界会更清晰，也便于后续扩展。

### 第四步：`internal/httpstream`

作为清理收尾步骤，把残留的 SSE 协议写出细节统一下沉，避免继续散落在 handler 内。

## 风险与约束

### 1. 不改动底层依赖职责

`internal/chatgpt` 应继续承担上游交互，不应在本轮模块化中被改造成新的业务编排层。否则会把“抽 handler 流程”演变成“重写底层客户端边界”，风险过高。

### 2. 避免一次性重构整文件

推荐按端点族渐进迁移，例如：

1. 先让 `responses` 复用 `conversationflow`
2. 再迁移 `nightmare`
3. 再迁移 `tts`

而不是一开始就把 `handlers.go` 大段整体搬迁，避免行为漂移难以定位。

### 3. 避免泛化过度

这次目标是提升扩展效率，不是打造一个抽象层层叠叠的框架。因此：

- 不建议先造一个大而全的 `internal/common`
- 不建议过早统一所有音频/图片/聊天输入成一个超级请求模型
- 优先围绕稳定业务边界抽模块，而不是围绕工具函数命名抽模块

## 预期结果

完成第一阶段模块化后，代码组织应逐步变成：

- `initialize/*`：更薄，只保留 HTTP 协议层
- `internal/*flow`：复用型业务编排模块
- `internal/chatgpt` / `tokens` / `proxys` / `httpclient`：底层能力模块

最终效果是：

1. 新增 API 端点时，优先组合现有 flow，而不是复制 handler 流程。
2. conversation / image / audio 等能力的复用点变成显式模块，而不是隐藏在单个 handler 文件里。
3. 未来继续拆分 `handlers.go` 时，有明确落点，而不是边拆边想边界。
