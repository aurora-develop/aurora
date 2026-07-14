# internal/chatgpt 包指纹审计 — 已知问题存档

**日期：** 2026-07-10
**状态：** 已确认，已修复

## 问题：header 使用全局 deviceID/sessionID

所有 header 构造函数的 deviceID 和 sessionID 都来自全局变量，而非 `account.Fingerprint`。

### 全局变量（request.go:69-70）

```
oaiDeviceID  = uuid.NewString()  // 进程级别,所有账号共享
oaiSessionID = uuid.NewString()  // 进程级别,所有账号共享
```

### 修复内容

2026-07-14: #272 修复了所有 header 函数，现在优先使用 `account.Fingerprint.OaiDeviceID/SessionID/UserAgent`，
回退到全局变量。新增 `baseHeaderFromAccount()` 辅助函数用于那些已有 account 参数的调用点。

### 受影响的调用点（已修复）

| 函数 | 文件 | 修复方式 |
|------|------|---------|
| sentinelHeaderWithState | headers.go | `account.Fingerprint.OaiDeviceID/SessionID` |
| conversationHeadersWithState | headers.go | `account.Fingerprint.OaiDeviceID/SessionID` |
| imageConversationHeadersWithState | headers.go | `account.Fingerprint.OaiDeviceID/SessionID` |
| conversationFetchHeaders | headers.go | `baseHeaderFromAccount(account)` |
| buildSentinelReqToken | sentinel.go | `account.Fingerprint.OaiDeviceID` |
| getURLAttribution | common.go | `baseHeaderFromAccount(account)` |
| getTTSBlobFromURL | tts.go | `baseHeaderFromAccount(account)` |
| RemoveConversation | tts.go | `baseHeaderFromAccount(account)` |
| createUpload | files.go | `baseHeaderFromAccount(account)` |
| confirmUpload | files.go | `baseHeaderFromAccount(account)` |
| TranscribeAudio | transcribe.go | `baseHeaderFromAccount(account)` |

---

*本文件是审计存档，记录修复前后的状态*
