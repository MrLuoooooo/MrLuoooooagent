package config

import "time"

// Config holds all application configuration.
type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Auth          AuthConfig          `mapstructure:"auth"`
	ModelProvider ModelProviderConfig `mapstructure:"model_provider"`
	VectorStore   VectorStoreConfig   `mapstructure:"vector_store"`
	MySQL         MySQLConfig         `mapstructure:"mysql"`
	Document      DocumentConfig      `mapstructure:"document"`
	Conversation  ConversationConfig  `mapstructure:"conversation"`
	Search        SearchConfig        `mapstructure:"search"`
	Cron          CronConfig          `mapstructure:"cron"`
	Log           LogConfig           `mapstructure:"log"`
	Stock         StockConfig         `mapstructure:"stock"`
	Agent         AgentConfig         `mapstructure:"agent"`
	Memory        MemoryConfig        `mapstructure:"memory"`
	Retrieval     RetrievalConfig     `mapstructure:"retrieval"`
	Alert         AlertConfig         `mapstructure:"alert"`
	MCP           MCPConfig           `mapstructure:"mcp"`
	SubAgents     SubAgentsConfig     `mapstructure:"sub_agents"`
	SemanticCache SemanticCacheConfig `mapstructure:"semantic_cache"`
	CozeLoop      CozeLoopConfig      `mapstructure:"cozeloop"`
}

// CozeLoopConfig 扣子罗盘 trace 上报配置。
// APIToken 建议走 .env 的 GOAGENT_COZELOOP_API_TOKEN，不落 yaml。
type CozeLoopConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	APIToken    string `mapstructure:"api_token"`
	WorkspaceID string `mapstructure:"workspace_id"`
	BaseURL     string `mapstructure:"base_url"` // 自建 coze-loop 服务端地址；空 = SaaS loop.coze.cn
}

// SemanticCacheConfig 语义缓存配置（非流式 RAG 分支）。
type SemanticCacheConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Threshold float64       `mapstructure:"threshold"` // 余弦相似度命中阈值，默认 0.92
	Capacity  int           `mapstructure:"capacity"`  // LRU 容量，默认 1024
	TTL       time.Duration `mapstructure:"ttl"`       // 条目有效期，默认 10m
}

// SubAgentsConfig 多 agent 委派配置。
type SubAgentsConfig struct {
	Enabled bool             `mapstructure:"enabled"`
	Agents  []SubAgentConfig `mapstructure:"agents"`
}

// SubAgentConfig 单个子 agent 配置。
type SubAgentConfig struct {
	Name        string        `mapstructure:"name"`
	Prompt      string        `mapstructure:"prompt"`
	Enabled     bool          `mapstructure:"enabled"`
	Model       string        `mapstructure:"model"` // 空则用当前模型
	Tools       []string      `mapstructure:"tools"` // 工具名白名单，子 agent 只见这些
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxRunSteps int           `mapstructure:"max_run_steps"`
}

// MySQLConfig holds business primary database configuration.
// 业务主库（会话/消息/反馈）；向量检索数据仍走 ES。
type MySQLConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	DSN          string `mapstructure:"dsn"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

// StockConfig holds stock data middleware configuration.
type StockConfig struct {
	DataDir string `mapstructure:"data_dir"`
}

// AgentConfig holds agent behavior configuration.
type AgentConfig struct {
	SystemPrompt      string `mapstructure:"system_prompt"`
	StockSystemPrompt string `mapstructure:"stock_system_prompt"`
}

// MemoryConfig holds long-term memory configuration.
type MemoryConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	RetrievalTopK int  `mapstructure:"retrieval_top_k"`
}

// RetrievalConfig holds RAG retrieval configuration.
type RetrievalConfig struct {
	TopK           int     `mapstructure:"top_k"`
	CandidateTopK  int     `mapstructure:"candidate_top_k"`
	ScoreThreshold float64 `mapstructure:"score_threshold"`
	HybridEnabled    bool    `mapstructure:"hybrid_enabled"`
	RerankerEnabled  bool    `mapstructure:"reranker_enabled"`
}

// AlertConfig holds intraday alert configuration.
type AlertConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	Watchlist      []string `mapstructure:"watchlist"`
	PriceChangePct float64  `mapstructure:"price_change_pct"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host         string   `mapstructure:"host"`
	Port         int      `mapstructure:"port"`
	Mode         string   `mapstructure:"mode"`
	WorkspaceDir string   `mapstructure:"workspace_dir"`
	AllowedDirs  []string `mapstructure:"allowed_dirs"`
	BashTimeout  int      `mapstructure:"bash_timeout"`
	CORSOrigins  []string `mapstructure:"cors_origins"`
	RateLimitRPS float64  `mapstructure:"rate_limit_rps"`
}

// ModelEntry defines a switchable model in the model_list.
type ModelEntry struct {
	Name           string `mapstructure:"name"  json:"name"`
	Provider       string `mapstructure:"provider" json:"provider"`
	ChatModel      string `mapstructure:"chat_model" json:"chat_model"`
	APIKey         string `mapstructure:"api_key" json:"api_key"`
	BaseURL        string `mapstructure:"base_url" json:"base_url"`
	EmbeddingModel string `mapstructure:"embedding_model" json:"embedding_model"`
}

// ModelProviderConfig configures local and remote LLM providers.
type ModelProviderConfig struct {
	Mode               string        `mapstructure:"mode"`
	EmbeddingDimension int           `mapstructure:"embedding_dimension"`
	Timeout            time.Duration `mapstructure:"timeout"`
	LLMRateLimitQPS    int           `mapstructure:"llm_rate_limit_qps"`    // LLM API 全局 QPS 上限，默认 10
	LLMMaxConcurrent   int           `mapstructure:"llm_max_concurrent"`   // LLM API 最大并发数，默认 20

	Local     LocalProviderConfig  `mapstructure:"local"`
	Cloud     CloudProviderConfig  `mapstructure:"cloud"`
	ModelList []ModelEntry         `mapstructure:"model_list"`
}

// LocalProviderConfig for Ollama.
type LocalProviderConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	BaseURL        string `mapstructure:"base_url"`
	ChatModel      string `mapstructure:"chat_model"`
	EmbeddingModel string `mapstructure:"embedding_model"`
}

// CloudProviderConfig for OpenAI-compatible APIs.
type CloudProviderConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	Type           string `mapstructure:"type"`
	APIKey         string `mapstructure:"api_key"`
	BaseURL        string `mapstructure:"base_url"`
	ChatModel      string `mapstructure:"chat_model"`
	EmbeddingModel string `mapstructure:"embedding_model"`
}

// VectorStoreConfig holds vector store configuration.
type VectorStoreConfig struct {
	Type          string              `mapstructure:"type"`
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
}

// ElasticsearchConfig holds Elasticsearch configuration.
type ElasticsearchConfig struct {
	Addresses        []string `mapstructure:"addresses"`
	IndexName        string   `mapstructure:"index_name"`
	Username         string   `mapstructure:"username"`
	Password         string   `mapstructure:"password"`
	ConvIndexName    string   `mapstructure:"conv_index_name"`
	ConvMsgIndexName string   `mapstructure:"conv_msg_index_name"`
	DocIndexName     string   `mapstructure:"doc_index_name"`
}

// DocumentConfig holds document processing configuration.
type DocumentConfig struct {
	MaxFileSize  int64    `mapstructure:"max_file_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
	ChunkSize    int      `mapstructure:"chunk_size"`
	ChunkOverlap int      `mapstructure:"chunk_overlap"`
}

// SearchConfig holds web search configuration.
type SearchConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Engine  string `mapstructure:"engine"`
	Format  string `mapstructure:"format"`
}

// ConversationConfig holds conversation configuration.
type ConversationConfig struct {
	MaxHistory  int    `mapstructure:"max_history"`
	StorageType string `mapstructure:"storage_type"`
}

// CronJobConfig defines a single scheduled task.
type CronJobConfig struct {
	Name        string `mapstructure:"name"`
	CronExpr    string `mapstructure:"cron_expr"`
	Prompt      string `mapstructure:"prompt"`
	AutoApprove bool   `mapstructure:"auto_approve"`
}

// CronConfig holds cron scheduler configuration.
type CronConfig struct {
	Enabled bool            `mapstructure:"enabled"`
	Jobs    []CronJobConfig `mapstructure:"jobs"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level      string `mapstructure:"level"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// ── MCP Config ────────────────────────────

// MCPConfig holds MCP server connection configuration.
type MCPConfig struct {
	Enabled bool        `mapstructure:"enabled"`
	Servers []MCPServer `mapstructure:"servers"`
}

// MCPServer defines a single MCP server connection.
type MCPServer struct {
	Name      string            `mapstructure:"name"`
	Transport string            `mapstructure:"transport"` // stdio | sse
	Command   string            `mapstructure:"command"`   // stdio
	Args      []string          `mapstructure:"args"`      // stdio
	Env       map[string]string `mapstructure:"env"`       // stdio
	URL       string            `mapstructure:"url"`       // sse
}
