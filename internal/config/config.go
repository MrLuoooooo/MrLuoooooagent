package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Load reads configuration from file and environment variables.
func Load() (*Config, error) {
	// Load .env into environment, silently skip if not found.
	_ = godotenv.Load()

	v := viper.New()
	setDefaults(v)

	configPath := "./configs"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}
	configName := "config"
	if name := os.Getenv("CONFIG_NAME"); name != "" {
		configName = name
	}
	v.SetConfigName(configName)
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")

	v.SetEnvPrefix("GOAGENT")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Printf("No config file found (%s/%s.yaml), using defaults and env vars", configPath, configName)
		} else {
			return nil, fmt.Errorf("read config file %s/%s.yaml: %w", configPath, configName, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Viper's AutomaticEnv doesn't work with Unmarshal.
	// Manually apply environment variable overrides for sensitive fields.
	if key := os.Getenv("GOAGENT_MODEL_PROVIDER_CLOUD_API_KEY"); key != "" {
		cfg.ModelProvider.Cloud.APIKey = key
	}
	if url := os.Getenv("GOAGENT_MODEL_PROVIDER_CLOUD_BASE_URL"); url != "" {
		cfg.ModelProvider.Cloud.BaseURL = url
	}
	if model := os.Getenv("GOAGENT_MODEL_PROVIDER_CLOUD_CHAT_MODEL"); model != "" {
		cfg.ModelProvider.Cloud.ChatModel = model
	}
	if key := os.Getenv("GOAGENT_AUTH_API_KEY"); key != "" {
		cfg.Auth.APIKey = key
	}
	if key := os.Getenv("GOAGENT_SEARCH_API_KEY"); key != "" {
		cfg.Search.APIKey = key
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.bash_timeout", 30)
	v.SetDefault("server.cors_origins", []string{"*"})
	v.SetDefault("server.rate_limit_rps", 10.0)
	v.SetDefault("server.workspace_dir", "")
	v.SetDefault("server.allowed_dirs", []string{})

	v.SetDefault("auth.api_key", "")

	v.SetDefault("model_provider.mode", "auto")
	v.SetDefault("model_provider.embedding_dimension", 1536)
	v.SetDefault("model_provider.timeout", "120s")

	v.SetDefault("model_provider.local.enabled", false)
	v.SetDefault("model_provider.local.base_url", "http://localhost:11434")
	v.SetDefault("model_provider.local.chat_model", "")
	v.SetDefault("model_provider.local.embedding_model", "")

	v.SetDefault("model_provider.cloud.enabled", false)
	v.SetDefault("model_provider.cloud.type", "openai")
	v.SetDefault("model_provider.cloud.api_key", "")
	v.SetDefault("model_provider.cloud.base_url", "https://api.openai.com/v1")
	v.SetDefault("model_provider.cloud.chat_model", "")
	v.SetDefault("model_provider.cloud.embedding_model", "")

	v.SetDefault("vector_store.type", "elasticsearch")
	v.SetDefault("vector_store.elasticsearch.addresses", []string{"http://localhost:9200"})
	v.SetDefault("vector_store.elasticsearch.index_name", "goagent_vectors")
	v.SetDefault("vector_store.elasticsearch.conv_index_name", "goagent_conversations")
	v.SetDefault("vector_store.elasticsearch.conv_msg_index_name", "goagent_conv_messages")
	v.SetDefault("vector_store.elasticsearch.doc_index_name", "goagent_documents")

	v.SetDefault("document.max_file_size", 10485760)
	v.SetDefault("document.allowed_types", []string{".pdf", ".txt", ".md", ".docx", ".xlsx", ".pptx"})
	v.SetDefault("document.chunk_size", 500)
	v.SetDefault("document.chunk_overlap", 50)

	v.SetDefault("conversation.max_history", 20)
	v.SetDefault("conversation.storage_type", "elasticsearch")

	v.SetDefault("retrieval.top_k", 10)
	v.SetDefault("retrieval.candidate_top_k", 30)
	v.SetDefault("retrieval.score_threshold", 0.3)
	v.SetDefault("retrieval.hybrid_enabled", false)
	v.SetDefault("retrieval.reranker_enabled", false)

	v.SetDefault("search.enabled", false)
	v.SetDefault("search.api_key", "")
	v.SetDefault("search.base_url", "https://serpapi.com/search")
	v.SetDefault("search.engine", "google")
	v.SetDefault("search.format", "serpapi")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.file_path", "./logs/goagent.log")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 7)
	v.SetDefault("log.max_age", 30)
	v.SetDefault("log.compress", true)

	v.SetDefault("stock.data_dir", "")
}
