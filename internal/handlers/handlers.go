package handlers

import (
	"errors"
	"net/http"

	"github.com/dario61k/conversion-service/internal/services/downloader"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	dl *downloader.Downloader
}

func New(dl *downloader.Downloader) *Handler { return &Handler{dl: dl} }

func (h *Handler) GetVideo(c *gin.Context) {
	id := c.Param("id")
	q := c.Param("quality")

	url, err := h.dl.VideoURL(c.Request.Context(), id, q)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, downloader.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_url": url})
}

func (h *Handler) GetSubtitle(c *gin.Context) {
	id := c.Param("id")
	lang := c.Param("lang")

	url, err := h.dl.SubtitleURL(c.Request.Context(), id, lang)
	if err != nil {
		status := http.StatusInternalServerError
		if err == downloader.ErrNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_url": url})
}

func (h *Handler) GetBucketList(c *gin.Context) {
	buckets, err := h.dl.ListBuckets(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
	}

	c.JSON(http.StatusOK, gin.H{"buckets": buckets})

}
