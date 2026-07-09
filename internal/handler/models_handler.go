package handler

import (
	"github.com/gin-gonic/gin"
)

type ModelsHandler struct{}

func NewModelsHandler() *ModelsHandler {
	return &ModelsHandler{}
}

// ListModels 返回模型列表（原 initialize/handlers.go:engines）
func (h *ModelsHandler) ListModels(c *gin.Context) {
	type ResData struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int    `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	type JSONData struct {
		Object string    `json:"object"`
		Data   []ResData `json:"data"`
	}

	models := []string{
		"auto",
		"gpt-5-5-instant",
		"gpt-5-5-thinking",
		"gpt-5-5-pro",
		"gpt-5",
		"gpt-4o",
		"gpt-4o-mini",
		"o3",
		"o4-mini",
		"o4-mini-high",
	}

	var resModelList []ResData
	for _, model := range models {
		resModelList = append(resModelList, ResData{
			ID:      model,
			Object:  "model",
			Created: 1685474247,
			OwnedBy: "openai",
		})
	}

	c.JSON(200, JSONData{
		Object: "list",
		Data:   resModelList,
	})
}
