package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

// Load reads configuration from file and environment variables.
func Load() (*Config, error) {
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
		log.Printf("Warning: failed to read config file (%s/%s.yaml): %v", configPath, configName, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")

	// Auth defaults (empty = dev mode, any token accepted).
	v.SetDefault("auth.api_key", "")

	// Model provider: auto mode, local disabled by default, cloud also disabled.
	v.SetDefault("model_provider.mode", "auto")
	v.SetDefault("model_provider.embedding_dimension", 1536)
	v.SetDefault("model_provider.timeout", "120s")

	// Ollama defaults.
	v.SetDefault("model_provider.local.enabled", false)
	v.SetDefault("model_provider.local.base_url", "http://localhost:11434")
	v.SetDefault("model_provider.local.chat_model", "")
	v.SetDefault("model_provider.local.embedding_model", "")

	// Cloud defaults.
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

	v.SetDefault("document.max_file_size", 10485760)
	v.SetDefault("document.allowed_types", []string{".pdf", ".txt", ".md"})
	v.SetDefault("document.chunk_size", 500)
	v.SetDefault("document.chunk_overlap", 50)

	v.SetDefault("conversation.max_history", 20)
	v.SetDefault("conversation.storage_type", "memory")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.file_path", "./logs/goagent.log")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 7)
	v.SetDefault("log.max_age", 30)
	v.SetDefault("log.compress", true)
}
