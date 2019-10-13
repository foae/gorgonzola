package http

import (
	"encoding/json"
	"github.com/foae/gorgonzola/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"net/url"
)

// Handler defines the structure of a handler.
type Handler struct {
	logger *zap.SugaredLogger
	db     repository.Interactor
}

// Config defines the structure of a handler config.
type Config struct {
	Logger *zap.SugaredLogger
	DB     repository.Interactor
}

// New returns a new instance of a handler
// based on a given configuration.
func New(cfg Config) *Handler {
	return &Handler{
		logger: cfg.Logger,
		db:     cfg.DB,
	}
}

func (h *Handler) Health(g *gin.Context) {
	g.JSON(http.StatusOK, "OK")
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
