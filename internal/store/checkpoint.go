package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

// CheckpointStore 实现 Eino 的 compose.CheckPointStore 接口，
// 基于文件存储 checkpoint 状态，路径：data/checkpoints/{convID}.eino_cp。
type CheckpointStore struct {
	dir    string
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewCheckpointStore 创建 checkpoint 存储，确保目录存在。
func NewCheckpointStore(dataDir string) (*CheckpointStore, error) {
	dir := filepath.Join(dataDir, "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create checkpoint dir: %w", err)
	}
	return &CheckpointStore{dir: dir, logger: zap.NewNop()}, nil
}

// NewCheckpointStoreWithLogger 创建带日志的 checkpoint 存储。
func NewCheckpointStoreWithLogger(dataDir string, logger *zap.Logger) (*CheckpointStore, error) {
	dir := filepath.Join(dataDir, "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("create checkpoint dir", zap.String("dir", dir), zap.Error(err))
		return nil, fmt.Errorf("create checkpoint dir: %w", err)
	}
	return &CheckpointStore{dir: dir, logger: logger}, nil
}

// Get 实现 Eino CheckPointStore 接口：读取 checkpoint 原始字节。
// 返回 (data, true, nil) 存在；(nil, false, nil) 不存在。
func (s *CheckpointStore) Get(_ context.Context, checkPointID string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(filepath.Join(s.dir, checkPointID+".eino_cp"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		s.logger.Error("read checkpoint", zap.String("cp_id", checkPointID), zap.Error(err))
		return nil, false, fmt.Errorf("read checkpoint: %w", err)
	}
	return data, true, nil
}

// Set 实现 Eino CheckPointStore 接口：写入 checkpoint 原始字节。
func (s *CheckpointStore) Set(_ context.Context, checkPointID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.WriteFile(filepath.Join(s.dir, checkPointID+".eino_cp"), data, 0644); err != nil {
		s.logger.Error("write checkpoint", zap.String("cp_id", checkPointID), zap.Error(err))
		return fmt.Errorf("write checkpoint: %w", err)
	}
	return nil
}

// Delete 删除 checkpoint（非 Eino 接口方法，用于 handler 层清理）。
func (s *CheckpointStore) Delete(_ context.Context, checkPointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, checkPointID+".eino_cp")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		s.logger.Error("delete checkpoint", zap.String("cp_id", checkPointID), zap.Error(err))
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}
