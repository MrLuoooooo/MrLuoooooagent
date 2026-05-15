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
	"github.com/yourusername/goagentpro/internal/callback"
	"github.com/yourusername/goagentpro/internal/component/esindexer"
	"github.com/yourusername/goagentpro/internal/component/esretriever"
	"github.com/yourusername/goagentpro/internal/component/openaiembed"
	"github.com/yourusername/goagentpro/internal/component/openaimodel"
	"github.com/yourusername/goagentpro/internal/component/tool"
	"github.com/yourusername/goagentpro/internal/config"
	"github.com/yourusername/goagentpro/internal/graph"
	"github.com/yourusername/goagentpro/internal/handler"
	"github.com/yourusername/goagentpro/internal/logger"
	"github.com/yourusername/goagentpro/internal/pipeline"
	"github.com/yourusername/goagentpro/internal/service"
	"github.com/yourusername/goagentpro/internal/store"
	"github.com/yourusername/goagentpro/internal/prompt"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ── Model provider auto-detection ──

type ResolvedConfig struct {
	ChatModel          string
	EmbeddingModel     string
	BaseURL            string
	APIKey             string
	EmbeddingDimension int
	Provider           string
}

// ResolvedEmbeddingConfig is resolved independently — embedding may live on a
// different provider than chat (e.g. Ollama nomic-embed-text + DeepSeek chat).
type ResolvedEmbeddingConfig struct {
	Model     string
	BaseURL   string
	APIKey    string
	Dimension int
	Provider  string
}

type ollamaModel struct {
	Name string `json:"name"`
}
type ollamaTags struct {
	Models []ollamaModel `json:"models"`
}

func resolveModelProvider(cfg *config.Config, logger *zap.Logger) *ResolvedConfig {
	switch cfg.ModelProvider.Mode {
	case "cloud":
		return useCloud(cfg, logger)
	case "local":
		return useLocal(cfg, logger)
	}
	// auto 模式：云端 API 优先（支持原生 function calling），不可用时回退 Ollama
	if cfg.ModelProvider.Cloud.Enabled && cfg.ModelProvider.Cloud.APIKey != "" {
		logger.Info("model: 使用云端 API（支持原生 function calling）", zap.String("chat", cfg.ModelProvider.Cloud.ChatModel))
		return useCloud(cfg, logger)
	}
	logger.Info("model: 未配置云端 API Key，检测本地 Ollama")
	if r := probeOllama(cfg, logger); r != nil {
		logger.Info("model: 使用本地 Ollama（prompt 回退工具调用）", zap.String("chat", r.ChatModel))
		return r
	}
	if cfg.ModelProvider.Cloud.Enabled || cfg.ModelProvider.Cloud.APIKey != "" {
		logger.Info("model: Ollama 不可用，尝试云端")
		return useCloud(cfg, logger)
	}
	logger.Error("没有配置llm: 未检测到本地 Ollama，也未配置云端 API Key。" +
		"请启动 Ollama 并确保已下载模型，或在配置中设置 model_provider.cloud.api_key。")
	return nil
}

func probeOllama(cfg *config.Config, logger *zap.Logger) *ResolvedConfig {
	if !cfg.ModelProvider.Local.Enabled {
		return nil
	}
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Get(cfg.ModelProvider.Local.BaseURL + "/api/tags")
	if err != nil {
		logger.Warn("ollama: 未运行或无法连接", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var tags ollamaTags
	json.NewDecoder(resp.Body).Decode(&tags)
	norm := func(s string) string {
		if len(s) > 7 && s[len(s)-7:] == ":latest" {
			return s[:len(s)-7]
		}
		return s
	}
	hasChat, hasEmb := false, false
	var avail []string
	for _, m := range tags.Models {
		n := norm(m.Name)
		avail = append(avail, n)
		if n == cfg.ModelProvider.Local.ChatModel {
			hasChat = true
		}
		if n == cfg.ModelProvider.Local.EmbeddingModel {
			hasEmb = true
		}
	}
	if !hasChat {
		logger.Warn("ollama: 未检测到本地模型，请运行: ollama pull "+cfg.ModelProvider.Local.ChatModel,
			zap.Strings("已安装", avail))
		return nil
	}
	if !hasEmb && cfg.ModelProvider.Local.ChatModel != cfg.ModelProvider.Local.EmbeddingModel {
		logger.Warn("ollama: 未检测到嵌入模型，请运行: ollama pull "+cfg.ModelProvider.Local.EmbeddingModel,
			zap.Strings("已安装", avail))
		return nil
	}
	dim := cfg.ModelProvider.EmbeddingDimension
	if dim <= 0 {
		dim = 768
	}
	return &ResolvedConfig{
		ChatModel: cfg.ModelProvider.Local.ChatModel, EmbeddingModel: cfg.ModelProvider.Local.EmbeddingModel,
		BaseURL: cfg.ModelProvider.Local.BaseURL + "/v1", APIKey: "ollama",
		EmbeddingDimension: dim, Provider: "ollama",
	}
}

// resolveEmbeddingProvider always prefers Ollama for embeddings because
// many chat APIs (e.g. DeepSeek) don't expose an /embeddings endpoint.
func resolveEmbeddingProvider(cfg *config.Config, logger *zap.Logger) *ResolvedEmbeddingConfig {
	if cfg.ModelProvider.Local.Enabled {
		if ec := probeOllamaEmbedding(cfg); ec != nil {
			logger.Info("embed: 使用本地 Ollama", zap.String("model", ec.Model))
			return ec
		}
		logger.Warn("embed: Ollama 嵌入模型不可用，回退云端")
	}
	if cfg.ModelProvider.Cloud.Enabled && cfg.ModelProvider.Cloud.APIKey != "" {
		dim := cfg.ModelProvider.EmbeddingDimension
		if dim <= 0 {
			dim = 1536
		}
		em := cfg.ModelProvider.Cloud.EmbeddingModel
		if em == "" {
			em = "text-embedding-3-small"
		}
		logger.Info("embed: 使用云端 API", zap.String("model", em))
		return &ResolvedEmbeddingConfig{
			Model: em, BaseURL: cfg.ModelProvider.Cloud.BaseURL,
			APIKey: cfg.ModelProvider.Cloud.APIKey, Dimension: dim,
			Provider: "cloud/" + cfg.ModelProvider.Cloud.Type,
		}
	}
	return nil
}

func probeOllamaEmbedding(cfg *config.Config) *ResolvedEmbeddingConfig {
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Get(cfg.ModelProvider.Local.BaseURL + "/api/tags")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var tags ollamaTags
	json.NewDecoder(resp.Body).Decode(&tags)
	norm := func(s string) string {
		if len(s) > 7 && s[len(s)-7:] == ":latest" {
			return s[:len(s)-7]
		}
		return s
	}
	for _, m := range tags.Models {
		if norm(m.Name) == cfg.ModelProvider.Local.EmbeddingModel {
			dim := cfg.ModelProvider.EmbeddingDimension
			if dim <= 0 {
				dim = 768
			}
			return &ResolvedEmbeddingConfig{
				Model:     cfg.ModelProvider.Local.EmbeddingModel,
				BaseURL:   cfg.ModelProvider.Local.BaseURL + "/v1",
				APIKey:    "ollama",
				Dimension: dim,
				Provider:  "ollama",
			}
		}
	}
	return nil
}

func useCloud(cfg *config.Config, logger *zap.Logger) *ResolvedConfig {
	c := cfg.ModelProvider.Cloud
	if c.APIKey == "" {
		logger.Warn("cloud enabled but api_key empty")
		return nil
	}
	dim := cfg.ModelProvider.EmbeddingDimension
	if dim <= 0 {
		dim = 1536
	}
	cm := c.ChatModel
	if cm == "" {
		cm = "gpt-4"
	}
	em := c.EmbeddingModel
	if em == "" {
		em = "text-embedding-3-small"
	}
	return &ResolvedConfig{ChatModel: cm, EmbeddingModel: em, BaseURL: c.BaseURL, APIKey: c.APIKey,
		EmbeddingDimension: dim, Provider: "cloud/" + c.Type}
}

func useLocal(cfg *config.Config, logger *zap.Logger) *ResolvedConfig {
	r := probeOllama(cfg, logger)
	if r == nil {
		logger.Warn("mode=local but Ollama unavailable")
	}
	return r
}

// ── Stubs (when no backend) ──

type stubEmbedder struct{}

func (s *stubEmbedder) EmbedStrings(_ context.Context, _ []string, _ ...embedding.Option) ([][]float64, error) {
	return nil, fmt.Errorf("no LLM backend configured — set model_provider.local.enabled=true or cloud.api_key")
}

type stubChatModel struct{}

func (s *stubChatModel) Generate(_ context.Context, _ []*eino_schema.Message, _ ...model.Option) (*eino_schema.Message, error) {
	return nil, fmt.Errorf("no LLM backend configured — set model_provider.local.enabled=true or cloud.api_key")
}
func (s *stubChatModel) Stream(_ context.Context, _ []*eino_schema.Message, _ ...model.Option) (*eino_schema.StreamReader[*eino_schema.Message], error) {
	return nil, fmt.Errorf("no LLM backend configured — set model_provider.local.enabled=true or cloud.api_key")
}
func (s *stubChatModel) BindTools(_ []*eino_schema.ToolInfo) error { return nil }

// ── fx providers ──

func ProvideConfig() (*config.Config, error) { return config.Load() }

func ProvideLogger(cfg *config.Config) (*zap.Logger, error) {
	return logger.NewLogger(&logger.Config{
		Level: cfg.Log.Level, FilePath: cfg.Log.FilePath,
		MaxSize: cfg.Log.MaxSize, MaxBackups: cfg.Log.MaxBackups,
		MaxAge: cfg.Log.MaxAge, Compress: cfg.Log.Compress,
	}), nil
}

func ProvideEmbedder(ec *ResolvedEmbeddingConfig) embedding.Embedder {
	if ec == nil {
		return &stubEmbedder{}
	}
	return openaiembed.NewOpenAIEmbedder(ec.APIKey, ec.Model, ec.BaseURL)
}

func ProvideChatModel(resolved *ResolvedConfig, logger *zap.Logger) model.ChatModel {
	if resolved == nil {
		return &stubChatModel{}
	}
	return openaimodel.NewOpenAIChatModel(resolved.APIKey, resolved.ChatModel, resolved.BaseURL, logger)
}

func ProvideESClient(cfg *config.Config) (*elasticsearch.Client, error) {
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.VectorStore.Elasticsearch.Addresses,
		Username:  cfg.VectorStore.Elasticsearch.Username,
		Password:  cfg.VectorStore.Elasticsearch.Password,
	})
}

func ProvideIndexer(client *elasticsearch.Client, emb embedding.Embedder, ec *ResolvedEmbeddingConfig, cfg *config.Config) indexer.Indexer {
	dim := 768
	if ec != nil && ec.Dimension > 0 {
		dim = ec.Dimension
	}
	return esindexer.NewElasticsearchIndexer(client, emb, cfg.VectorStore.Elasticsearch.IndexName, dim)
}

func ProvideRetriever(client *elasticsearch.Client, emb embedding.Embedder, cfg *config.Config) retriever.Retriever {
	return esretriever.NewESRetriever(client, emb, cfg.VectorStore.Elasticsearch.IndexName, 10)
}

func ProvideESConversationStore(client *elasticsearch.Client, cfg *config.Config, logger *zap.Logger) (*store.ESConversationStore, error) {
	return store.NewESConversationStore(client,
		cfg.VectorStore.Elasticsearch.ConvIndexName,
		cfg.VectorStore.Elasticsearch.ConvMsgIndexName,
		logger)
}

func ProvideDocumentStore(client *elasticsearch.Client, cfg *config.Config, logger *zap.Logger) (*store.ESDocumentStore, error) {
	return store.NewESDocumentStore(client,
		cfg.VectorStore.Elasticsearch.DocIndexName,
		logger)
}

// docStoreAdapter adapts ESDocumentStore to service.DocStore interface.
type docStoreAdapter struct {
	es *store.ESDocumentStore
}

func (a *docStoreAdapter) Save(ctx context.Context, doc service.DocMeta) error {
	return a.es.Save(ctx, store.DocumentMeta{
		ID: doc.ID, Filename: doc.Filename, ChunkCount: doc.ChunkCount,
		CreatedAt: doc.CreatedAt, Content: doc.Content,
	})
}

func (a *docStoreAdapter) Delete(ctx context.Context, id string) error {
	return a.es.Delete(ctx, id)
}

func (a *docStoreAdapter) List(ctx context.Context) ([]service.DocMeta, error) {
	docs, err := a.es.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]service.DocMeta, len(docs))
	for i, d := range docs {
		result[i] = service.DocMeta{
			ID: d.ID, Filename: d.Filename, ChunkCount: d.ChunkCount,
			CreatedAt: d.CreatedAt, Content: d.Content,
		}
	}
	return result, nil
}

func ProvideDocStoreAdapter(es *store.ESDocumentStore) service.DocStore {
	return &docStoreAdapter{es: es}
}

// ProvideRAGChain creates the RAG pipeline.
func ProvideRAGChain(cm model.ChatModel, rd retriever.Retriever) (compose.Runnable[string, *eino_schema.Message], error) {
	tmpl := prompt.NewRAGTemplate()
	return pipeline.NewRAGChain(rd, tmpl, cm)
}

// ToolRegistry is a sentinel type that forces tool registration to complete
// before any consumer (e.g. ProvideAgentGraph) reads the global tool registry.
type ToolRegistry struct{}

// ProvideToolRegistry registers all tools into the global tool registry.
// It depends on ragChain so the RAG tool can wrap the pipeline.
func ProvideToolRegistry(
	cfg *config.Config,
	ragChain compose.Runnable[string, *eino_schema.Message],
) *ToolRegistry {
	// === 文件系统工具（8 个）===
	tool.Register(&tool.ReadFileTool{})
	tool.Register(&tool.WriteFileTool{})
	tool.Register(&tool.EditFileTool{})
	tool.Register(&tool.DeleteFileTool{})
	tool.Register(&tool.ListDirectoryTool{})
	tool.Register(&tool.CreateDirectoryTool{})
	tool.Register(&tool.SearchFilesTool{})
	tool.Register(&tool.GetFileInfoTool{})

	// === Bash 工具 ===
	tool.Register(tool.NewBashTool(cfg.Server.AllowedDirs))
	tool.Register(tool.NewWriteAndExecuteTool(cfg.Server.AllowedDirs))

	// === 现有工具 ===
	tool.Register(&tool.DateTimeTool{})

	// WebSearch: construct with search config.
	ws := tool.NewWebSearchTool(
		cfg.Search.BaseURL,
		cfg.Search.APIKey,
		cfg.Search.Engine,
		cfg.Search.Format,
		cfg.Search.Enabled,
	)
	tool.Register(ws)

	// Register RAG as a tool that wraps the RAG pipeline.
	tool.Register(tool.NewRAGTool(func(ctx context.Context, query string) (string, error) {
		msg, err := ragChain.Invoke(ctx, query)
		if err != nil {
			return "", err
		}
		return msg.Content, nil
	}))

	return &ToolRegistry{}
}

// ProvideAgentGraph creates the tool-calling Agent graph from already-registered tools.
func ProvideAgentGraph(
	cm model.ChatModel,
	_ *ToolRegistry, // ensures tool registration completed first
) (compose.Runnable[*eino_schema.Message, *eino_schema.Message], error) {
	allTools := tool.RegisteredTools()

	// Build ToolsNode with all registered tools as eino_tool.BaseTool.
	einoTools := make([]eino_tool.BaseTool, len(allTools))
	for i, t := range allTools {
		einoTools[i] = t
	}
	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{Tools: einoTools})
	if err != nil {
		return nil, err
	}

	// Bind tool infos to the ChatModel so it knows what tools it can call.
	infos, err := tool.ToolInfos(context.Background())
	if err != nil {
		return nil, err
	}
	graph.MustBindTools(cm, infos)

	return graph.NewAgentGraph(cm, tn, infos)
}

func ProvideDocChain(emb embedding.Embedder, idx indexer.Indexer, cfg *config.Config) (compose.Runnable[[]byte, []string], error) {
	return pipeline.NewDocumentIngestionChain(emb, idx, cfg.Document.ChunkSize, cfg.Document.ChunkOverlap)
}

func ProvideChatService(
	rag compose.Runnable[string, *eino_schema.Message],
	agent compose.Runnable[*eino_schema.Message, *eino_schema.Message],
	convSvc *service.ConversationService,
	logger *zap.Logger,
) *service.ChatService {
	return service.NewChatService(rag, agent, convSvc, logger)
}

// VectorDeleter is implemented by Indexer implementations that support per-document
// vector deletion (e.g. Elasticsearch). If the current backend does not support it,
// DocumentService.Delete falls back to deleting only the metadata record.
func ProvideVectorDeleter(idx indexer.Indexer) service.VectorDeleter {
	if vd, ok := idx.(service.VectorDeleter); ok {
		return vd
	}
	return nil
}

func ProvideDocService(
	doc compose.Runnable[[]byte, []string],
	docStore service.DocStore,
	vecDeleter service.VectorDeleter,
	logger *zap.Logger,
) *service.DocumentService {
	return service.NewDocumentService(doc, docStore, vecDeleter, logger)
}

func ProvideConvService(esStore *store.ESConversationStore, logger *zap.Logger) *service.ConversationService {
	return service.NewConversationService(esStore, logger)
}

func ProvideChatHandler(svc *service.ChatService, convSvc *service.ConversationService, logger *zap.Logger) *handler.ChatHandler {
	return handler.NewChatHandler(svc, convSvc, logger)
}

func ProvideConvHandler(svc *service.ConversationService) *handler.ConversationHandler {
	return handler.NewConversationHandler(svc)
}

func ProvideDocHandler(svc *service.DocumentService, logger *zap.Logger) *handler.DocumentHandler {
	return handler.NewDocumentHandler(svc, logger)
}

func ProvideRouter(
	cfg *config.Config,
	logger *zap.Logger,
	chatH *handler.ChatHandler,
	convH *handler.ConversationHandler,
	docH *handler.DocumentHandler,
) *gin.Engine {
	return NewRouter(cfg, logger, chatH, convH, docH)
}

// ── Module ──

var Module = fx.Module("goagent",
		fx.Provide(
		ProvideConfig,
		ProvideLogger,
		resolveModelProvider,
		resolveEmbeddingProvider,
		ProvideEmbedder,
		ProvideChatModel,
		ProvideESClient,
		ProvideIndexer,
		ProvideRetriever,
		ProvideESConversationStore,
		ProvideDocumentStore,
		ProvideDocStoreAdapter,
		ProvideVectorDeleter,
		ProvideRAGChain,
			ProvideToolRegistry,
		ProvideAgentGraph,
		ProvideDocChain,
		ProvideChatService,
		ProvideDocService,
		ProvideConvService,
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
)
