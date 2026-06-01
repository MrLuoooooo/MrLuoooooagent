package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	eino_tool "github.com/cloudwego/eino/components/tool"
	eino_schema "github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/callback"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/esindexer"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/esretriever"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/modelmanager"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/openaiembed"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/openaimodel"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/tool"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/graph"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/handler"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/logger"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/pipeline"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/prompt"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/scheduler"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/server/middleware"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type ResolvedConfig struct {
	ChatModel, EmbeddingModel, BaseURL, APIKey, Provider string
	EmbeddingDimension int
}

type ResolvedEmbeddingConfig struct {
	Model, BaseURL, APIKey, Provider string
	Dimension int
}

type ollamaModel struct{ Name string `json:"name"` }
type ollamaTags struct{ Models []ollamaModel `json:"models"` }

func resolveModelProvider(cfg *config.Config, logger *zap.Logger) *ResolvedConfig {
	switch cfg.ModelProvider.Mode {
	case "cloud": return useCloud(cfg, logger)
	case "local": return useLocal(cfg, logger)
	}
	// auto 模式：优先使用 OpenAI 兼容的云端 API
	if cfg.ModelProvider.Cloud.Enabled {
		logger.Info("model: 使用云端 API", zap.String("chat", cfg.ModelProvider.Cloud.ChatModel))
		return useCloud(cfg, logger)
	}
	if r := probeOllama(cfg, logger); r != nil {
		logger.Info("model: 使用本地 Ollama", zap.String("chat", r.ChatModel))
		return r
	}
	logger.Error("no LLM configured")
	return nil
}

func probeOllama(cfg *config.Config, logger *zap.Logger) *ResolvedConfig {
	if !cfg.ModelProvider.Local.Enabled { return nil }
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Get(cfg.ModelProvider.Local.BaseURL + "/api/tags")
	if err != nil { logger.Warn("ollama unreachable", zap.Error(err)); return nil }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return nil }
	var tags ollamaTags
	json.NewDecoder(resp.Body).Decode(&tags)
	norm := func(s string) string {
		if len(s) > 7 && s[len(s)-7:] == ":latest" { return s[:len(s)-7] }
		return s
	}
	hasChat, hasEmb := false, false
	var avail []string
	for _, m := range tags.Models {
		n := norm(m.Name); avail = append(avail, n)
		if n == cfg.ModelProvider.Local.ChatModel { hasChat = true }
		if n == cfg.ModelProvider.Local.EmbeddingModel { hasEmb = true }
	}
	if !hasChat { logger.Warn("ollama chat missing", zap.Strings("avail", avail)); return nil }
	if !hasEmb && cfg.ModelProvider.Local.ChatModel != cfg.ModelProvider.Local.EmbeddingModel {
		logger.Warn("ollama embed missing", zap.Strings("avail", avail)); return nil
	}
	dim := cfg.ModelProvider.EmbeddingDimension
	if dim <= 0 { dim = 768 }
	return &ResolvedConfig{ChatModel: cfg.ModelProvider.Local.ChatModel, EmbeddingModel: cfg.ModelProvider.Local.EmbeddingModel, BaseURL: cfg.ModelProvider.Local.BaseURL + "/v1", APIKey: "ollama", EmbeddingDimension: dim, Provider: "ollama"}
}

func resolveEmbeddingProvider(cfg *config.Config, logger *zap.Logger) *ResolvedEmbeddingConfig {
	if cfg.ModelProvider.Local.Enabled {
		if ec := probeOllamaEmbedding(cfg); ec != nil {
			logger.Info("embed: local Ollama", zap.String("model", ec.Model))
			return ec
		}
	}
	if cfg.ModelProvider.Cloud.Enabled && cfg.ModelProvider.Cloud.APIKey != "" && cfg.ModelProvider.Cloud.EmbeddingModel != "" {
		dim := cfg.ModelProvider.EmbeddingDimension
		if dim <= 0 { dim = 1536 }
		return &ResolvedEmbeddingConfig{Model: cfg.ModelProvider.Cloud.EmbeddingModel, BaseURL: cfg.ModelProvider.Cloud.BaseURL, APIKey: cfg.ModelProvider.Cloud.APIKey, Dimension: dim, Provider: "cloud/" + cfg.ModelProvider.Cloud.Type}
	}
	logger.Info("embed: no backend configured (set cloud.embedding_model to enable cloud embeddings)")
	return nil
}

func probeOllamaEmbedding(cfg *config.Config) *ResolvedEmbeddingConfig {
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Get(cfg.ModelProvider.Local.BaseURL + "/api/tags")
	if err != nil { return nil }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return nil }
	var tags ollamaTags
	json.NewDecoder(resp.Body).Decode(&tags)
	norm := func(s string) string {
		if len(s) > 7 && s[len(s)-7:] == ":latest" { return s[:len(s)-7] }
		return s
	}
	for _, m := range tags.Models {
		if norm(m.Name) == cfg.ModelProvider.Local.EmbeddingModel {
			dim := cfg.ModelProvider.EmbeddingDimension
			if dim <= 0 { dim = 768 }
			return &ResolvedEmbeddingConfig{Model: cfg.ModelProvider.Local.EmbeddingModel, BaseURL: cfg.ModelProvider.Local.BaseURL + "/v1", APIKey: "ollama", Dimension: dim, Provider: "ollama"}
		}
	}
	return nil
}

func useCloud(cfg *config.Config, _ *zap.Logger) *ResolvedConfig {
	c := cfg.ModelProvider.Cloud
	dim := cfg.ModelProvider.EmbeddingDimension
	if dim <= 0 { dim = 1536 }
	cm := c.ChatModel
	if cm == "" { cm = "gpt-4" }
	ak := c.APIKey
	if ak == "" { ak = "sk-placeholder" }
	return &ResolvedConfig{ChatModel: cm, EmbeddingModel: "", BaseURL: c.BaseURL, APIKey: ak, EmbeddingDimension: dim, Provider: "cloud/" + c.Type}
}

func useLocal(cfg *config.Config, logger *zap.Logger) *ResolvedConfig {
	r := probeOllama(cfg, logger)
	if r == nil { logger.Warn("mode=local but Ollama unavailable") }
	return r
}

// ── Stubs ──

type stubEmbedder struct{}

func (s *stubEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	dim := 768
	result := make([][]float64, len(texts))
	for i := range texts {
		v := make([]float64, dim)
		v[0] = 1.0 // valid unit vector, won't match any real document
		result[i] = v
	}
	return result, nil
}

type stubChatModel struct{}

func (s *stubChatModel) Generate(_ context.Context, _ []*eino_schema.Message, _ ...model.Option) (*eino_schema.Message, error) {
	return nil, fmt.Errorf("no LLM backend configured")
}
func (s *stubChatModel) Stream(_ context.Context, _ []*eino_schema.Message, _ ...model.Option) (*eino_schema.StreamReader[*eino_schema.Message], error) {
	return nil, fmt.Errorf("no LLM backend configured")
}
func (s *stubChatModel) BindTools(_ []*eino_schema.ToolInfo) error { return nil }

// ── fx providers ──

func ProvideConfig() (*config.Config, error) { return config.Load() }

func ProvideLogger(cfg *config.Config) (*zap.Logger, error) {
	return logger.NewLogger(&logger.Config{Level: cfg.Log.Level, FilePath: cfg.Log.FilePath, MaxSize: cfg.Log.MaxSize, MaxBackups: cfg.Log.MaxBackups, MaxAge: cfg.Log.MaxAge, Compress: cfg.Log.Compress}), nil
}

func ProvideEmbedder(ec *ResolvedEmbeddingConfig) embedding.Embedder {
	if ec == nil { return &stubEmbedder{} }
	return openaiembed.NewOpenAIEmbedder(ec.APIKey, ec.Model, ec.BaseURL)
}

func ProvideModelManager(resolved *ResolvedConfig, cfg *config.Config, customStore *service.ModelStore, logger *zap.Logger) *modelmanager.ModelManager {
	if resolved == nil { return modelmanager.NewModelManager(&stubChatModel{}, cfg, customStore, "", "", logger) }
	initial := openaimodel.NewOpenAIChatModel(resolved.APIKey, resolved.ChatModel, resolved.BaseURL, logger)
	return modelmanager.NewModelManager(initial, cfg, customStore, resolved.ChatModel, resolved.BaseURL, logger)
}

func ProvideChatModel(mm *modelmanager.ModelManager) model.ChatModel { return mm }

func ProvideCheckpointStore(cfg *config.Config, logger *zap.Logger) (*store.CheckpointStore, error) {
	dataDir := cfg.Stock.DataDir
	if dataDir == "" {
		dataDir = "./data"
	}
	return store.NewCheckpointStoreWithLogger(dataDir, logger)
}

func ProvideESClient(cfg *config.Config) (*elasticsearch.Client, error) {
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses:     cfg.VectorStore.Elasticsearch.Addresses,
		Username:      cfg.VectorStore.Elasticsearch.Username,
		Password:      cfg.VectorStore.Elasticsearch.Password,
		RetryOnStatus: []int{502, 503, 504},
		MaxRetries:    3,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	})
}

func ProvideIndexer(client *elasticsearch.Client, emb embedding.Embedder, ec *ResolvedEmbeddingConfig, cfg *config.Config) indexer.Indexer {
	dim := 768
	if ec != nil && ec.Dimension > 0 { dim = ec.Dimension }
	return esindexer.NewElasticsearchIndexer(client, emb, cfg.VectorStore.Elasticsearch.IndexName, dim)
}

func ProvideRetriever(client *elasticsearch.Client, emb embedding.Embedder, cfg *config.Config) retriever.Retriever {
	return esretriever.NewESRetriever(client, emb, cfg.VectorStore.Elasticsearch.IndexName, 10)
}

func ProvideESConversationStore(client *elasticsearch.Client, cfg *config.Config, logger *zap.Logger) (*store.ESConversationStore, error) {
	return store.NewESConversationStore(client, cfg.VectorStore.Elasticsearch.ConvIndexName, cfg.VectorStore.Elasticsearch.ConvMsgIndexName, logger)
}

func ProvideDocumentStore(client *elasticsearch.Client, cfg *config.Config, logger *zap.Logger) (*store.ESDocumentStore, error) {
	return store.NewESDocumentStore(client, cfg.VectorStore.Elasticsearch.DocIndexName, logger)
}

type docStoreAdapter struct{ es *store.ESDocumentStore }

func (a *docStoreAdapter) Save(ctx context.Context, doc service.DocMeta) error {
	return a.es.Save(ctx, store.DocumentMeta{ID: doc.ID, Filename: doc.Filename, ChunkCount: doc.ChunkCount, CreatedAt: doc.CreatedAt, Content: doc.Content})
}
func (a *docStoreAdapter) Delete(ctx context.Context, id string) error { return a.es.Delete(ctx, id) }
func (a *docStoreAdapter) List(ctx context.Context) ([]service.DocMeta, error) {
	docs, err := a.es.List(ctx)
	if err != nil { return nil, err }
	result := make([]service.DocMeta, len(docs))
	for i, d := range docs { result[i] = service.DocMeta{ID: d.ID, Filename: d.Filename, ChunkCount: d.ChunkCount, CreatedAt: d.CreatedAt, Content: d.Content} }
	return result, nil
}

func ProvideDocStoreAdapter(es *store.ESDocumentStore) service.DocStore { return &docStoreAdapter{es: es} }

func ProvideRAGChain(cm model.ChatModel, rd retriever.Retriever) (compose.Runnable[string, *eino_schema.Message], error) {
	tmpl := prompt.NewRAGTemplate()
	return pipeline.NewRAGChain(rd, tmpl, cm)
}

type ToolRegistry struct{}

func ProvideToolRegistry(cfg *config.Config, ragChain compose.Runnable[string, *eino_schema.Message]) *ToolRegistry {
	tool.Register(&tool.ReadFileTool{})
	tool.Register(&tool.WriteFileTool{})
	tool.Register(&tool.EditFileTool{})
	tool.Register(&tool.DeleteFileTool{})
	tool.Register(&tool.ListDirectoryTool{})
	tool.Register(&tool.CreateDirectoryTool{})
	tool.Register(&tool.SearchFilesTool{})
	tool.Register(&tool.GetFileInfoTool{})
	tool.Register(tool.NewBashTool(cfg.Server.AllowedDirs))
	tool.Register(tool.NewWriteAndExecuteTool(cfg.Server.AllowedDirs))
	tool.Register(&tool.DateTimeTool{})
	tool.Register(tool.NewWebSearchTool(cfg.Search.BaseURL, cfg.Search.APIKey, cfg.Search.Engine, cfg.Search.Format, cfg.Search.Enabled))
	tool.Register(tool.NewRAGTool(func(ctx context.Context, query string) (string, error) {
		msg, err := ragChain.Invoke(ctx, query)
		if err != nil { return "", err }
		return msg.Content, nil
	}))
	tool.Register(tool.NewStockRealtimeTool(cfg.Stock.DataDir))
	tool.Register(tool.NewStockKLineTool(cfg.Stock.DataDir))
	tool.Register(tool.NewWebFetchTool(cfg.Search.Enabled))
	tool.Register(&tool.CalculatorTool{})
	tool.Register(&tool.StockSearchTool{})
	tool.Register(&tool.StockIndexTool{})
	tool.Register(&tool.JSONTool{})
	tool.Register(&tool.TextTools{})
	tool.Register(tool.NewBatchTool())
	return &ToolRegistry{}
}

func ProvideAgentGraph(cm model.ChatModel, _ *ToolRegistry, skills *service.SkillStore, cfg *config.Config, cpStore *store.CheckpointStore) (compose.Runnable[*eino_schema.Message, *eino_schema.Message], error) {
	allTools := tool.RegisteredTools()
	einoTools := make([]eino_tool.BaseTool, len(allTools))
	for i, t := range allTools { einoTools[i] = t }
	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{Tools: einoTools})
	if err != nil { return nil, err }
	infos, err := tool.ToolInfos(context.Background())
	if err != nil { return nil, err }
	if err := cm.BindTools(infos); err != nil {
		return nil, fmt.Errorf("bind tools: %w", err)
	}
	return graph.NewAgentGraph(cm, tn, infos, skills, cfg.Agent.SystemPrompt, cpStore)
}

func ProvideDocChain(emb embedding.Embedder, idx indexer.Indexer, cfg *config.Config) (compose.Runnable[[]byte, []string], error) {
	return pipeline.NewDocumentIngestionChain(emb, idx, cfg.Document.ChunkSize, cfg.Document.ChunkOverlap)
}

func ProvideBatchPipeline(agent compose.Runnable[*eino_schema.Message, *eino_schema.Message]) *pipeline.BatchPipeline {
	return pipeline.NewBatchPipeline(agent)
}

func ProvideModelStore() *service.ModelStore  { return service.NewModelStore() }
func ProvideSkillStore() *service.SkillStore  { return service.NewSkillStore() }
func ProvideApprovalStore() *service.ApprovalStore { return service.NewApprovalStore() }

func ProvideChatService(rag compose.Runnable[string, *eino_schema.Message], agent compose.Runnable[*eino_schema.Message, *eino_schema.Message], convSvc *service.ConversationService, logger *zap.Logger) *service.ChatService {
	return service.NewChatService(rag, agent, convSvc, logger)
}

func ProvideVectorDeleter(idx indexer.Indexer) service.VectorDeleter {
	if vd, ok := idx.(service.VectorDeleter); ok { return vd }
	return nil
}

func ProvideDocService(doc compose.Runnable[[]byte, []string], docStore service.DocStore, vecDeleter service.VectorDeleter, logger *zap.Logger) *service.DocumentService {
	return service.NewDocumentService(doc, docStore, vecDeleter, logger)
}

func ProvideConvService(esStore *store.ESConversationStore, logger *zap.Logger) *service.ConversationService {
	return service.NewConversationService(esStore, logger)
}

func ProvideChatHandler(svc *service.ChatService, convSvc *service.ConversationService, cpStore *store.CheckpointStore, logger *zap.Logger) *handler.ChatHandler {
	return handler.NewChatHandler(svc, convSvc, cpStore, logger)
}

func ProvideCronScheduler(cfg *config.Config, agent compose.Runnable[*eino_schema.Message, *eino_schema.Message], logger *zap.Logger, approvals *service.ApprovalStore) *scheduler.CronScheduler {
	return scheduler.NewCronScheduler(cfg, agent, logger, approvals)
}

func ProvideBatchHandler(bp *pipeline.BatchPipeline, logger *zap.Logger) *handler.BatchHandler {
	return handler.NewBatchHandler(bp, logger)
}

func ProvideApprovalHandler(store *service.ApprovalStore, logger *zap.Logger) *handler.ApprovalHandler {
	return handler.NewApprovalHandler(store, logger)
}

func ProvideModelHandler(cfg *config.Config, mm *modelmanager.ModelManager, store *service.ModelStore, logger *zap.Logger) *handler.ModelHandler {
	return handler.NewModelHandler(cfg, mm, store, logger)
}

func ProvideSkillHandler(store *service.SkillStore, logger *zap.Logger) *handler.SkillHandler {
	return handler.NewSkillHandler(store, logger)
}

func ProvideWorkspaceHandler(logger *zap.Logger, cfg *config.Config) *handler.WorkspaceHandler {
	return handler.NewWorkspaceHandler(logger, cfg.Server)
}

func ProvideConvHandler(svc *service.ConversationService, logger *zap.Logger) *handler.ConversationHandler {
	return handler.NewConversationHandler(svc, logger)
}

func ProvideDocHandler(svc *service.DocumentService, cfg *config.Config, logger *zap.Logger) *handler.DocumentHandler {
	return handler.NewDocumentHandler(svc, cfg, logger)
}

func ProvideRateLimiter(cfg *config.Config) *middleware.RateLimiter {
	rps := cfg.Server.RateLimitRPS
	if rps <= 0 {
		rps = 0
	}
	return middleware.NewRateLimiter(rps, rps*2)
}

func ProvideRouter(cfg *config.Config, logger *zap.Logger, chatH *handler.ChatHandler, convH *handler.ConversationHandler, docH *handler.DocumentHandler, batchH *handler.BatchHandler, approvalH *handler.ApprovalHandler, modelH *handler.ModelHandler, skillH *handler.SkillHandler, wsH *handler.WorkspaceHandler, rl *middleware.RateLimiter) *gin.Engine {
	return NewRouter(cfg, logger, chatH, convH, docH, batchH, approvalH, modelH, skillH, wsH, rl)
}

// ── Module ──

var Module = fx.Module("goagent",
	fx.Provide(
		ProvideConfig,
		ProvideLogger,
		resolveModelProvider,
		resolveEmbeddingProvider,
		ProvideEmbedder,
		ProvideModelManager,
		ProvideChatModel,
		ProvideModelStore,
		ProvideSkillStore,
		ProvideApprovalStore,
		ProvideESClient,
		ProvideIndexer,
		ProvideRetriever,
		ProvideESConversationStore,
		ProvideDocumentStore,
		ProvideDocStoreAdapter,
		ProvideVectorDeleter,
		ProvideRAGChain,
		ProvideToolRegistry,
		ProvideCheckpointStore,
		ProvideAgentGraph,
		ProvideBatchPipeline,
		ProvideCronScheduler,
		ProvideRateLimiter,
		ProvideDocChain,
		ProvideChatService,
		ProvideDocService,
		ProvideConvService,
		ProvideBatchHandler,
		ProvideApprovalHandler,
		ProvideModelHandler,
		ProvideSkillHandler,
		ProvideWorkspaceHandler,
		ProvideChatHandler,
		ProvideConvHandler,
		ProvideDocHandler,
		ProvideRouter,
	),
	fx.Invoke(func(logger *zap.Logger) {
		handler := callback.NewLoggingCallback(logger)
		callbacks.AppendGlobalHandlers(handler)
		logger.Info("global callback handler registered")
	}),
	fx.Invoke(func(bp *pipeline.BatchPipeline) {
		tool.SetBatchPipeline(bp)
	}),
)
