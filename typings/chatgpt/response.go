package chatgpt

type ChatGPTResponse struct {
	Message        Message     `json:"message"`
	ConversationID string      `json:"conversation_id"`
	Error          interface{} `json:"error"`
}
type ChatGPTWSSResponse struct {
	WssUrl         string `json:"wss_url"`
	ConversationId string `json:"conversation_id,omitempty"`
	ResponseId     string `json:"response_id,omitempty"`
}

type WSSMsgResponse struct {
	SequenceId int                `json:"sequenceId"`
	Type       string             `json:"type"`
	From       string             `json:"from"`
	DataType   string             `json:"dataType"`
	Data       WSSMsgResponseData `json:"data"`
}

type WSSMsgResponseData struct {
	Type           string `json:"type"`
	Body           string `json:"body"`
	MoreBody       bool   `json:"more_body"`
	ResponseId     string `json:"response_id"`
	ConversationId string `json:"conversation_id"`
}

type Message struct {
	ID         string      `json:"id"`
	Author     Author      `json:"author"`
	CreateTime float64     `json:"create_time"`
	UpdateTime interface{} `json:"update_time"`
	Content    Content     `json:"content"`
	EndTurn    interface{} `json:"end_turn"`
	Weight     float64     `json:"weight"`
	Metadata   Metadata    `json:"metadata"`
	Recipient  string      `json:"recipient"`
}

type Content struct {
	ContentType string        `json:"content_type"`
	Parts       []interface{} `json:"parts"`
}

type Author struct {
	Role     string                 `json:"role"`
	Name     interface{}            `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Metadata struct {
	Timestamp     string         `json:"timestamp_"`
	Citations     []Citation     `json:"citations,omitempty"`
	MessageType   string         `json:"message_type"`
	FinishDetails *FinishDetails `json:"finish_details"`
	ModelSlug     string         `json:"model_slug"`
}
type Citation struct {
	Metadata CitaMeta `json:"metadata"`
	StartIx  int      `json:"start_ix"`
	EndIx    int      `json:"end_ix"`
}
type CitaMeta struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
type FinishDetails struct {
	Type string `json:"type"`
	Stop string `json:"stop"`
}
type DalleContent struct {
	AssetPointer string `json:"asset_pointer"`
	Metadata     struct {
		Dalle struct {
			Prompt string `json:"prompt"`
		} `json:"dalle"`
	} `json:"metadata"`
}

type ProofWork struct {
	Difficulty string `json:"difficulty,omitempty"`
	Required   bool   `json:"required"`
	Seed       string `json:"seed,omitempty"`
}

type RequirementsResponse struct {
	Arkose struct {
		Required bool        `json:"required"`
		Dx       interface{} `json:"dx"`
	} `json:"arkose"`
	Proof     ProofWork `json:"proofofwork,omitempty"`
	Turnstile struct {
		Required bool `json:"required"`
	} `json:"turnstile"`
	Token string `json:"token"`
}
