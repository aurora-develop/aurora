# internal/chatgpt 包指纹审计 — 已知问题存档

**日期：** 2026-07-10
**状态：** 已确认，暂不修复

## 问题：header 使用全局 deviceID/sessionID

所有 header 构造函数的 deviceID 和 sessionID 都来自全局变量，而非 `account.Fingerprint`。

### 全局变量（request.go:69-70）

```
oaiDeviceID  = uuid.NewString()  // 进程级别,所有账号共享
oaiSessionID = uuid.NewString()  // 进程级别,所有账号共享
```

### 受影响位置

| 函数 | 位置 | 文件名:行 | 用全局 | 应该用 |
|------|------|-----------|-------|--------|
| sentinelHeaderWithState | request.go:730-731 | 请求头 | `oaiDeviceID/sessionID` | `account.Fingerprint.OaiDeviceID/SessionID` |
| conversationHeadersWithState | request.go:821-822 | 请求头 | `oaiDeviceID/sessionID` | `account.Fingerprint.OaiDeviceID/SessionID` |
| imageConversationHeadersWithState | request.go:1885-1886 | 请求头 | `oaiDeviceID/sessionID` | `account.Fingerprint.OaiDeviceID/SessionID` |
| createBaseHeaderForState | request.go:2785-2786 | 基础头 | `oaiDeviceID/sessionID` | 需传入 `*accounts.Account` |
| POSTSentinelReq | request.go:623 | 直接引用 | `oaiDeviceID` | `account.Fingerprint.OaiDeviceID` |
| POSTConversationInit | request.go:562 | 基础头 | `createBaseHeaderForState(state)` | 无 account 参数 |
| getURLAttribution | request.go:786 | 基础头 | `createBaseHeader()` | 无 account 参数 |
| GetTTS (2处) | request.go:2899,2980 | 基础头 | `createBaseHeader()` | 无 account 参数 |
| conversationFetchHeaders | request.go:1764 | 基础头 | `createBaseHeader()` | 无 account 参数 |
| RemoveConversation | request.go:2980 | 基础头 | `createBaseHeader()` | 无 account 参数 |

### 影响

- 池里每个 `*Account` 有独立的 `Fingerprint.OaiDeviceID` / `OaiSessionID`（`CreateAccount` 时随机分配）
- 但所有上游请求都发同一个设备 ID
- ChatGPT/Cloudflare 看到的所有请求共享一个设备指纹
- 每个账号的"独立浏览器身份"隔离失效

### 影响范围

- 至少 3 个 `account.Fingerprint` 字段被忽略：`OaiDeviceID`、`OaiSessionID`、`UserAgent`
- 涉及 4 个核心 header 函数 + 5 个使用了 `createBaseHeader` 的端点
- 修复需为所有 header 函数补 `*accounts.Account` 参数，FIXME 时全面改

---

*本文件是审计存档，不追踪修改*
