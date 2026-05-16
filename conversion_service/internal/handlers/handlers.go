package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/services"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	dl  *services.Downloader
	cfg config.Config
}

func NewHandler(dl *services.Downloader, cfg config.Config) *Handler {
	return &Handler{dl: dl, cfg: cfg}
}

func (h *Handler) GetVideo(c *gin.Context) {
	id := c.Param("id")
	q := c.Param("quality")
	token := c.GetHeader("Authorization")

	result, err := h.dl.RequestVideo(c.Request.Context(), token, id, q)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, services.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, services.ErrInvalidRequest) {
			status = http.StatusBadRequest
		} else if errors.Is(err, services.ErrForbidden) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	status := http.StatusOK
	if result.JobID != 0 && result.DownloadURL == "" {
		status = http.StatusAccepted
	}
	c.JSON(status, result)
}

func (h *Handler) GetSubtitle(c *gin.Context) {
	id := c.Param("id")
	lang := c.Param("lang")

	url, err := h.dl.SubtitleURL(c.Request.Context(), id, lang)
	if err != nil {
		status := http.StatusInternalServerError
		if err == services.ErrNotFound {
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
		return
	}

	c.JSON(http.StatusOK, gin.H{"buckets": buckets})

}

func (h *Handler) GetJob(c *gin.Context) {
	jobID, err := strconv.ParseInt(c.Param("job_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job_id"})
		return
	}

	token := c.GetHeader("Authorization")
	ctx := c.Request.Context()
	deadline := time.Now().Add(time.Duration(h.cfg.LongPollTimeout) * time.Second)
	interval := time.Duration(h.cfg.LongPollInterval) * time.Millisecond

	for {
		result, err := h.dl.JobStatus(ctx, token, jobID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, services.ErrNotFound) {
				status = http.StatusNotFound
			} else if errors.Is(err, services.ErrForbidden) {
				status = http.StatusUnauthorized
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}

		if result.Status == "completed" || result.Status == "failed" || result.Status == "expired" {
			c.JSON(http.StatusOK, result)
			return
		}

		if time.Now().After(deadline) {
			c.JSON(http.StatusOK, result)
			return
		}

		select {
		case <-time.After(interval):
		case <-ctx.Done():
			c.JSON(http.StatusRequestTimeout, gin.H{"error": "request canceled"})
			return
		}
	}
}
