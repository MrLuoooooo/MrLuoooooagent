package config

import "time"

// Config holds all application configuration.
type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Auth          AuthConfig          `mapstructure:"auth"`
	ModelProvider ModelProviderConfig `mapstructure:"model_provider"`
	VectorStore   VectorStoreConfig   `mapstructure:"vector_store"`
	Document      DocumentConfig      `mapstructure:"document"`
	Conversation  ConversationConfig  `mapstructure:"conversation"`
	Log           LogConfig           `mapstructure:"log"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// ModelProviderConfig configures local and remote LLM providers.
// The resolver auto-detects availability and falls back gracefully.
type ModelProviderConfig struct {
	// Mode: "auto" (default) → try Ollama, fallback to Cloud; "local" → Ollama only; "cloud" → remote only.
	Mode                 string        `mapstructure:"mode"`
	EmbeddingDimension   int           `mapstructure:"embedding_dimension"`
	Timeout              time.Duration `mapstructure:"timeout"`

	Local  LocalProviderConfig  `mapstructure:"local"`
	Cloud  CloudProviderConfig  `mapstructure:"cloud"`
}

// LocalProviderConfig for Ollama.
type LocalProviderConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	BaseURL         string `mapstructure:"base_url"`
	ChatModel       string `mapstructure:"chat_model"`
	EmbeddingModel  string `mapstructure:"embedding_model"`
}

// CloudProviderConfig for OpenAI-compatible APIs (OpenAI, 阿里云百炼, etc.).
type CloudProviderConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	Type           string `mapstructure:"type"`            // "openai" or "aliyun"
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
}

// DocumentConfig holds document processing configuration.
type DocumentConfig struct {
	MaxFileSize  int64    `mapstructure:"max_file_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
	ChunkSize    int      `mapstructure:"chunk_size"`
	ChunkOverlap int      `mapstructure:"chunk_overlap"`
}

// ConversationConfig holds conversation configuration.
type ConversationConfig struct {
	MaxHistory  int    `mapstructure:"max_history"`
	StorageType string `mapstructure:"storage_type"`
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
