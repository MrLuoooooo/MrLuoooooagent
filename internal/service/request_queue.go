package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// QueuedRequest 一个排队中的 Agent 请求。
type QueuedRequest struct {
	ConvID    string
	UserMsg   *schema.Message
	Question  string
	Priority  int
	Ctx       context.Context
	ResultCh  chan *QueueResult
	CreatedAt time.Time
}

// QueueResult 排队结果。
type QueueResult struct {
	Stream *schema.StreamReader[*schema.Message] // 流式结果
	Err    error                                  // 错误
	NodeID string                                 // "queued" / "position" / "coalesced" / "done" / "error"
}

// RequestQueue 永不拒绝的产品级请求队列。
type RequestQueue struct {
	mu       sync.Mutex
	pending  []*QueuedRequest
	dispatch chan struct{}
	logger   *zap.Logger
	dedup    map[string]*QueuedRequest
}

// NewRequestQueue —
func NewRequestQueue(logger *zap.Logger) *RequestQueue {
	return &RequestQueue{
		pending:  make([]*QueuedRequest, 0),
		dispatch: make(chan struct{}, 1),
		dedup:    make(map[string]*QueuedRequest),
		logger:   logger,
	}
}

// Submit 入队。req.ResultCh 会被创建并填充（如未预先设置）。
// 返回初始排队状态（queued/coalesced）。
// 调用者后续从 req.ResultCh 读取流式结果和位置更新。
func (q *RequestQueue) Submit(req *QueuedRequest) *QueueResult {
	q.mu.Lock()
	defer q.mu.Unlock()

	dedupKey := req.ConvID + "::" + hashContent(req.Question)
	if existing, ok := q.dedup[dedupKey]; ok {
		q.logger.Debug("request coalesced", zap.String("conv", req.ConvID))
		req.ResultCh = existing.ResultCh
		return &QueueResult{NodeID: "coalesced"}
	}

	req.ResultCh = make(chan *QueueResult, 16)
	req.CreatedAt = time.Now()
	q.pending = append(q.pending, req)
	q.dedup[dedupKey] = req

	select {
	case q.dispatch <- struct{}{}:
	default:
	}

	pos := len(q.pending)
	return &QueueResult{
		NodeID: "queued",
		Err:    fmt.Errorf("%d|%ds", pos, pos*2),
	}
}

// DrainAndDispatch 阻塞运行调度器。
func (q *RequestQueue) DrainAndDispatch(ctx context.Context, graph compose.Runnable[*schema.Message, *schema.Message]) {
	// 最后兜底：理论上 processNext 已自行 recover，这里防止任何漏网 panic 让整个调度器 goroutine 退出
	defer func() {
		if r := recover(); r != nil {
			q.logger.Error("queue: dispatcher goroutine panic recovered (should be unreachable)",
				zap.Any("err", r),
				zap.Stack("stack"),
			)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.dispatch:
			q.processNext(ctx, graph)
		}
	}
}

func (q *RequestQueue) processNext(ctx context.Context, graph compose.Runnable[*schema.Message, *schema.Message]) {
	var req *QueuedRequest // 提前声明，供 defer 闭包捕获；panic 时关闭 ResultCh 防止前端阻塞
	// goroutine 内 panic 会直接崩进程，必须拦截。panic 时关闭 ResultCh 避免前端 for-range 永久阻塞
	defer func() {
		if r := recover(); r != nil {
			q.logger.Error("queue: processNext panic recovered",
				zap.Any("err", r),
				zap.Stack("stack"),
			)
			if req != nil {
				select {
				case <-ctx.Done():
				default:
					close(req.ResultCh)
				}
			}
		}
	}()
	q.mu.Lock()
	if len(q.pending) == 0 {
		q.mu.Unlock()
		return
	}
	best := 0
	for i := 1; i < len(q.pending); i++ {
		if q.pending[i].Priority < q.pending[best].Priority ||
			(q.pending[i].Priority == q.pending[best].Priority && q.pending[i].CreatedAt.Before(q.pending[best].CreatedAt)) {
			best = i
		}
	}
	req = q.pending[best]
	q.pending = append(q.pending[:best], q.pending[best+1:]...)

	dedupKey := req.ConvID + "::" + hashContent(req.Question)
	delete(q.dedup, dedupKey)

	for _, r := range q.pending {
		pos := q.posOfLocked(r)
		select {
		case r.ResultCh <- &QueueResult{NodeID: "position", Err: fmt.Errorf("%d", pos)}:
		default:
		}
	}
	q.mu.Unlock()

	q.logger.Debug("queue: dispatching", zap.String("conv", req.ConvID), zap.Int("priority", req.Priority))
	stream, err := graph.Stream(ctx, req.UserMsg)
	if err != nil {
		req.ResultCh <- &QueueResult{Err: err, NodeID: "error"}
		close(req.ResultCh)
		return
	}
	req.ResultCh <- &QueueResult{Stream: stream, NodeID: "done"}

	for {
		msg, recvErr := stream.Recv()
		if recvErr != nil {
			close(req.ResultCh)
			return
		}
		select {
		case req.ResultCh <- &QueueResult{Stream: schema.StreamReaderFromArray([]*schema.Message{msg}), NodeID: "token"}:
		case <-ctx.Done():
			close(req.ResultCh)
			return
		}
	}
}

func (q *RequestQueue) posOfLocked(req *QueuedRequest) int {
	for i, r := range q.pending {
		if r == req {
			return i + 1
		}
	}
	return len(q.pending)
}

// PendingCount 当前排队人数。
func (q *RequestQueue) PendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

func hashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:12]
}
