package storage

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock/api"
	"go.uber.org/zap"
)

// FileStore 带锁文件存储：原子写入（tmp→rename）+ gzip 备份 + 数据合并去重。
type FileStore struct {
	dataDir   string
	fileLocks map[string]*sync.Mutex
	locksMu   sync.RWMutex
	logger    *zap.Logger
}

// NewFileStore 创建文件存储，自动建好 realtime/historical/backup 子目录。
func NewFileStore(dataDir string, logger *zap.Logger) *FileStore {
	if dataDir == "" {
		dataDir = "./data"
	}
	ensureDir(filepath.Join(dataDir, "stocks", "realtime"))
	ensureDir(filepath.Join(dataDir, "stocks", "historical"))
	ensureDir(filepath.Join(dataDir, "stocks", "backup"))
	return &FileStore{
		dataDir:   dataDir,
		fileLocks: make(map[string]*sync.Mutex),
		logger:    logger,
	}
}

func ensureDir(dir string) {
	_ = os.MkdirAll(dir, 0755)
}

func (s *FileStore) lock(key string) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	if m, ok := s.fileLocks[key]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.fileLocks[key] = m
	return m
}

// SaveStockData 原子写入行情数据：先写 tmp → sync → rename，写前 gzip 备份旧文件。
func (s *FileStore) SaveStockData(ctx context.Context, data *api.StockData) error {
	code := data.Code
	lk := s.lock(code)
	lk.Lock()
	defer lk.Unlock()

	filePath := filepath.Join(s.dataDir, "stocks", "realtime", code+".json")
	_ = s.backup(filePath)

	return s.atomicWrite(filePath, data)
}

// SaveKLineData 原子写入 K 线数据，与已有数据合并去重后保存。
func (s *FileStore) SaveKLineData(ctx context.Context, code, period string, data []api.KLineData) error {
	key := code + "_" + period
	lk := s.lock(key)
	lk.Lock()
	defer lk.Unlock()

	ensureDir(filepath.Join(s.dataDir, "stocks", "historical", period))
	filePath := filepath.Join(s.dataDir, "stocks", "historical", period, code+".json")

	existing, _ := s.GetKLineData(ctx, code, period, 0)
	merged := mergeKLine(existing, data)

	_ = s.backup(filePath)
	return s.atomicWrite(filePath, merged)
}

func (s *FileStore) atomicWrite(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func (s *FileStore) backup(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}
	backupDir := filepath.Join(s.dataDir, "stocks", "backup")
	ensureDir(backupDir)
	ts := time.Now().Format("20060102_150405")
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s_%s%s.gz", name, ts, ext))

	src, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	gw := gzip.NewWriter(dst)
	defer gw.Close()
	_, err = io.Copy(gw, src)
	return err
}

// GetStockData 从本地文件读取行情数据。
func (s *FileStore) GetStockData(ctx context.Context, code string) (*api.StockData, error) {
	filePath := filepath.Join(s.dataDir, "stocks", "realtime", code+".json")
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var data api.StockData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetKLineData 从本地文件读取 K 线数据。
func (s *FileStore) GetKLineData(ctx context.Context, code, period string, limit int) ([]api.KLineData, error) {
	filePath := filepath.Join(s.dataDir, "stocks", "historical", period, code+".json")
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var data []api.KLineData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}
	if limit > 0 && len(data) > limit {
		data = data[len(data)-limit:]
	}
	return data, nil
}

// CleanupOldFiles 清理超过 maxAge 的 gzip 备份文件。
func (s *FileStore) CleanupOldFiles(ctx context.Context, maxAge time.Duration) error {
	backupDir := filepath.Join(s.dataDir, "stocks", "backup")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil
	}
	cutoff := time.Now().Add(-maxAge)
	var deleted int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		if info != nil && info.ModTime().Before(cutoff) {
			p := filepath.Join(backupDir, e.Name())
			os.Remove(p)
			deleted++
		}
	}
	if deleted > 0 {
		s.logger.Info("old backups cleaned", zap.Int("deleted", deleted))
	}
	return nil
}

func mergeKLine(existing, new []api.KLineData) []api.KLineData {
	m := make(map[string]api.KLineData, len(existing)+len(new))
	for _, e := range existing {
		m[e.Time] = e
	}
	for _, n := range new {
		m[n.Time] = n
	}
	result := make([]api.KLineData, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}
