package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger 记录每个请求的耗时和状态码。
// 每个请求生成 request_id（尊重上游 X-Request-ID）：写入响应头、gin context、
// 日志字段——是 zap 日志与扣子罗盘 trace 之间审计关联的锚点。
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path, method := c.Request.URL.Path, c.Request.Method

		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = genRequestID()
		}
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)

		c.Next()
		logger.Info("req",
			zap.String("request_id", rid),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("took", time.Since(start)),
		)
	}
}

// genRequestID 8 字节随机数的 hex 形式，碰撞概率可忽略。
func genRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}
