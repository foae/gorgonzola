package http

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
)

func (h *Handler) Health(g *gin.Context) {
	g.String(http.StatusOK, "OK")
}

// AddToBlocklist defines an action at the handler.
func (h *Handler) AddToBlocklist(g *gin.Context) {
	var input struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(g.Request.Body).Decode(&input); err != nil {
		h.logger.Errorf("json: %v", err)
		g.Status(http.StatusInternalServerError)
		return
	}
	defer g.Request.Body.Close() // nolint

	_, err := url.Parse(input.URL)
	if err != nil {
		h.logger.Errorf("url: %v", err)
		g.Status(http.StatusBadRequest)
		return
	}

	g.JSON(http.StatusOK, input)
}
