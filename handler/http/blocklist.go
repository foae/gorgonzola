package http

import (
	"encoding/binary"
	"encoding/json"
	"github.com/dgraph-io/badger"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
)

func (h *Handler) Health(g *gin.Context) {
	l, v := h.db.Size()
	output := struct {
		LSM  int64    `json:"lsm"`
		VLog int64    `json:"v_log"`
		Keys []uint16 `json:"keys"`
	}{
		LSM:  l,
		VLog: v,
		Keys: make([]uint16, 0),
	}

	// iterate through all the keys
	_ = h.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			key := binary.BigEndian.Uint16(k)
			output.Keys = append(output.Keys, key)
		}
		return nil
	})

	g.JSON(http.StatusOK, output)
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
