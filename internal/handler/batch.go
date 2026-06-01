package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/pipeline"
	"go.uber.org/zap"
)

// BatchHandler 管批量任务入口。
type BatchHandler struct {
	pipeline *pipeline.BatchPipeline
	logger   *zap.Logger
}

// NewBatchHandler —
func NewBatchHandler(pipeline *pipeline.BatchPipeline, logger *zap.Logger) *BatchHandler {
	return &BatchHandler{pipeline: pipeline, logger: logger}
}

// HandleBatch executes a batch of tasks and streams progress via SSE.
// @Summary      批量执行任务
// @Description  并行执行最多 10 个子任务，通过 SSE 流式推送进度。每个任务独立运行，结果汇总返回。
// @Tags         批量任务
// @Accept       json
// @Produce      text/event-stream
// @Param        request body model.BatchRequest true "批量任务请求（最多 10 个任务）"
// @Success      200 "SSE 流式推送 task_start/task_done/task_error/summary/done 事件"
// @Failure      400 {object} model.APIEnvelope
// @Router       /batch [post]
func (h *BatchHandler) HandleBatch(c *gin.Context) {
	var req model.BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	if len(req.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, model.Err(400, "tasks 不能为空"))
		return
	}
	if len(req.Tasks) > 10 {
		c.JSON(http.StatusBadRequest, model.Err(400, "单次最多 10 个任务"))
		return
	}

	// Setup SSE.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	ch := h.pipeline.Execute(c.Request.Context(), req.Tasks)

	c.Stream(func(w io.Writer) bool {
		evt, ok := <-ch
		if !ok {
			return false
		}
		data, _ := json.Marshal(evt)
		w.Write([]byte("data: " + string(data) + "\n\n"))
		return evt.Type != model.BatchDone
	})
}
