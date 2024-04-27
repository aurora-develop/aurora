package duckgo

type ApiResponse struct {
	Message string `json:"message"`
	Created int    `json:"created"`
	Id      string `json:"id"`
	Action  string `json:"action"`
	Model   string `json:"model"`
}
