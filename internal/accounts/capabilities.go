package accounts

// Capability 表示一个功能对账号类型的要求
// ★ 当 ChatGPT 策略变化时，只改这个文件的对应常量 ★
type Capability struct {
	Name          string
	RequiresLogin bool // 需要登录（free 或 puid）
}

// 系统内所有功能及其当前账号要求
var (
	CapChat           = Capability{Name: "chat", RequiresLogin: true}
	CapResponses      = Capability{Name: "responses", RequiresLogin: true}
	CapToolCalling    = Capability{Name: "tool_calling", RequiresLogin: true}
	CapImageGenerate  = Capability{Name: "image_generation", RequiresLogin: true}
	CapImageEdit      = Capability{Name: "image_edit", RequiresLogin: true}
	CapImageVariation = Capability{Name: "image_variation", RequiresLogin: true}
	CapTTS            = Capability{Name: "tts", RequiresLogin: true}
	CapTranscribe     = Capability{Name: "transcribe", RequiresLogin: true}
	CapFileUpload     = Capability{Name: "file_upload", RequiresLogin: true}
	CapWebSocket      = Capability{Name: "websocket", RequiresLogin: true}
)

// Satisfies 判断账号类型是否满足某项能力要求
func (t AccountType) Satisfies(cap Capability) bool {
	switch t {
	case TypePUID:
		return true // 付费账号全部可用
	case TypeFree:
		return true // 免费登录账号也全部可用（只是有次数限制）
	case TypeNoAuth:
		return !cap.RequiresLogin // 匿名账号只能不用登录的能力
	default:
		return false
	}
}
