package official

type APIRequest struct {
	Messages  []api_message `json:"messages"`
	Stream    bool          `json:"stream"`
	Model     string        `json:"model"`
	PluginIDs []string      `json:"plugin_ids"`
}

type api_message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
