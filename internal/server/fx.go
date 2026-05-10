package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	eino_tool "github.com/cloudwego/eino/components/tool"
	eino_schema "github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
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
	logger.Info("model: probing Ollama")
	if r := probeOllama(cfg, logger); r != nil {
		logger.Info("model: using Ollama", zap.String("chat", r.ChatModel))
		return r
	}
	if cfg.ModelProvider.Cloud.Enabled || cfg.ModelProvider.Cloud.APIKey != "" {
		logger.Info("model: Ollama unavailable, falling back to cloud")
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

func ProvideEmbedder(resolved *ResolvedConfig) embedding.Embedder {
	if resolved == nil {
		return &stubEmbedder{}
	}
	return openaiembed.NewOpenAIEmbedder(resolved.APIKey, resolved.EmbeddingModel, resolved.BaseURL)
}

func ProvideChatModel(resolved *ResolvedConfig) model.ChatModel {
	if resolved == nil {
		return &stubChatModel{}
	}
	return openaimodel.NewOpenAIChatModel(resolved.APIKey, resolved.ChatModel, resolved.BaseURL)
}

func ProvideESClient(cfg *config.Config) (*elasticsearch.Client, error) {
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.VectorStore.Elasticsearch.Addresses,
		Username:  cfg.VectorStore.Elasticsearch.Username,
		Password:  cfg.VectorStore.Elasticsearch.Password,
	})
}

func ProvideIndexer(client *elasticsearch.Client, emb embedding.Embedder, resolved *ResolvedConfig, cfg *config.Config) indexer.Indexer {
	dim := 768
	if resolved != nil && resolved.EmbeddingDimension > 0 {
		dim = resolved.EmbeddingDimension
	}
	return esindexer.NewElasticsearchIndexer(client, emb, cfg.VectorStore.Elasticsearch.IndexName, dim)
}

func ProvideRetriever(client *elasticsearch.Client, emb embedding.Embedder, cfg *config.Config) retriever.Retriever {
	return esretriever.NewESRetriever(client, emb, cfg.VectorStore.Elasticsearch.IndexName, 10)
}

func ProvideMemory() *store.ConversationMemory {
	return store.NewConversationMemory(100)
}

func ProvideDocumentStore() *store.DocumentStore {
	return store.NewDocumentStore()
}

// ProvideRAGChain creates the RAG pipeline.
func ProvideRAGChain(cm model.ChatModel, rd retriever.Retriever) (compose.Runnable[string, *eino_schema.Message], error) {
	tmpl := prompt.NewRAGTemplate()
	return pipeline.NewRAGChain(rd, tmpl, cm)
}

// ProvideAgentGraph creates the tool-calling Agent graph with all registered tools.
func ProvideAgentGraph(
	cm model.ChatModel,
	ragChain compose.Runnable[string, *eino_schema.Message],
) (compose.Runnable[*eino_schema.Message, *eino_schema.Message], error) {
	// Register core tools.
	tool.Register(&tool.DateTimeTool{})
	tool.Register(&tool.WebSearchTool{})

	// Register RAG as a tool that wraps the RAG pipeline.
	tool.Register(tool.NewRAGTool(func(ctx context.Context, query string) (string, error) {
		msg, err := ragChain.Invoke(ctx, query)
		if err != nil {
			return "", err
		}
		return msg.Content, nil
	}))

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

	return graph.NewAgentGraph(cm, tn)
}

func ProvideDocChain(emb embedding.Embedder, idx indexer.Indexer, cfg *config.Config) (compose.Runnable[[]byte, []string], error) {
	return pipeline.NewDocumentIngestionChain(emb, idx, cfg.Document.ChunkSize, cfg.Document.ChunkOverlap)
}

func ProvideChatService(
	rag compose.Runnable[string, *eino_schema.Message],
	agent compose.Runnable[*eino_schema.Message, *eino_schema.Message],
	logger *zap.Logger,
) *service.ChatService {
	return service.NewChatService(rag, agent, logger)
}

func ProvideDocService(doc compose.Runnable[[]byte, []string], logger *zap.Logger) *service.DocumentService {
	return service.NewDocumentService(doc, logger)
}

func ProvideConvService(mem *store.ConversationMemory, logger *zap.Logger) *service.ConversationService {
	return service.NewConversationService(mem, logger)
}

func ProvideChatHandler(svc *service.ChatService, mem *store.ConversationMemory, logger *zap.Logger) *handler.ChatHandler {
	return handler.NewChatHandler(svc, mem, logger)
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
		ProvideEmbedder,
		ProvideChatModel,
		ProvideESClient,
		ProvideIndexer,
		ProvideRetriever,
		ProvideMemory,
		ProvideDocumentStore,
		ProvideRAGChain,
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
)
