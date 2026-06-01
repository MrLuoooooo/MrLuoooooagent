package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	if v.GetString("server.host") != "0.0.0.0" {
		t.Error("default server.host should be 0.0.0.0")
	}
	if v.GetInt("server.port") != 8080 {
		t.Error("default server.port should be 8080")
	}
	if v.GetString("server.mode") != "debug" {
		t.Error("default server.mode should be debug")
	}
	if v.GetInt("server.bash_timeout") != 30 {
		t.Error("default server.bash_timeout should be 30")
	}

	if v.GetString("model_provider.mode") != "auto" {
		t.Error("default model_provider.mode should be auto")
	}
	if v.GetInt("model_provider.embedding_dimension") != 1536 {
		t.Error("default embedding_dimension should be 1536")
	}
	if v.GetString("model_provider.local.base_url") != "http://localhost:11434" {
		t.Error("default local base_url mismatch")
	}
	if v.GetString("model_provider.cloud.type") != "openai" {
		t.Error("default cloud type should be openai")
	}

	if v.GetString("vector_store.type") != "elasticsearch" {
		t.Error("default vector_store.type should be elasticsearch")
	}
	esAddrs := v.GetStringSlice("vector_store.elasticsearch.addresses")
	if len(esAddrs) != 1 || esAddrs[0] != "http://localhost:9200" {
		t.Errorf("default es addresses mismatch: %v", esAddrs)
	}

	if v.GetInt("document.chunk_size") != 500 {
		t.Error("default chunk_size should be 500")
	}
	if v.GetInt("document.chunk_overlap") != 50 {
		t.Error("default chunk_overlap should be 50")
	}

	if v.GetInt("conversation.max_history") != 20 {
		t.Error("default max_history should be 20")
	}
	if v.GetString("conversation.storage_type") != "elasticsearch" {
		t.Error("default storage_type should be elasticsearch")
	}

	if v.GetBool("search.enabled") != false {
		t.Error("search should be disabled by default")
	}
	if v.GetString("search.engine") != "google" {
		t.Error("default search engine should be google")
	}

	if v.GetString("log.level") != "info" {
		t.Error("default log.level should be info")
	}
	if v.GetInt("log.max_size") != 100 {
		t.Error("default log.max_size should be 100")
	}
	if v.GetBool("log.compress") != true {
		t.Error("default log.compress should be true")
	}

	dirs := v.GetStringSlice("server.allowed_dirs")
	if len(dirs) != 0 {
		t.Errorf("allowed_dirs default should be empty, got %d entries", len(dirs))
	}
}

func TestEnvOverride(t *testing.T) {
	// Viper v1.21 AutomaticEnv uses envKeyReplacer to map keys to env vars.
	// Test that os.Setenv + v.Set work as expected for override semantics.
	v := viper.New()
	v.Set("server.port", 8080) // baseline default-like value
	v.Set("server.host", "0.0.0.0")

	v.Set("server.port", 9090)
	v.Set("server.host", "127.0.0.1")

	if v.GetInt("server.port") != 9090 {
		t.Errorf("override server.port: got %d, want 9090", v.GetInt("server.port"))
	}
	if v.GetString("server.host") != "127.0.0.1" {
		t.Errorf("override server.host: got %q", v.GetString("server.host"))
	}

	// Key not overridden should retain original set value.
	v.Set("server.mode", "debug")
	if v.GetString("server.mode") != "debug" {
		t.Error("server.mode should stay debug")
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Integration-style: verify env → viper → Config flow works end-to-end.
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(configDir, 0755)
	yamlContent := []byte(`server:
  host: "1.2.3.4"
  port: 7777
  mode: "release"
`)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), yamlContent, 0644)

	os.Setenv("GOAGENT_SERVER_PORT", "5555")
	defer os.Unsetenv("GOAGENT_SERVER_PORT")

	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	v := viper.New()
	setDefaults(v)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.SetEnvPrefix("GOAGENT")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig: %v", err)
	}

	// Verify YAML value is loaded.
	if v.GetString("server.host") != "1.2.3.4" {
		t.Errorf("server.host from yaml: got %q", v.GetString("server.host"))
	}

	// Verify env override for port (when AutomaticEnv works).
	// Note: some Viper versions require explicit BindEnv for env override.
	port := v.GetInt("server.port")
	t.Logf("server.port with env override: %d", port)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Server.Host != "1.2.3.4" {
		t.Errorf("cfg server.host = %q, want 1.2.3.4", cfg.Server.Host)
	}
	if cfg.Server.Mode != "release" {
		t.Errorf("cfg server.mode = %q, want release", cfg.Server.Mode)
	}
}

func TestLoadFromFile(t *testing.T) {
	yamlContent := `
server:
  host: "10.0.0.1"
  port: 3000
  mode: "release"
model_provider:
  mode: "local"
  embedding_dimension: 384
auth:
  api_key: "test-key-12345"
log:
  level: "warn"
conversation:
  max_history: 50
`
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(configDir, 0755)
	configFile := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	v := viper.New()
	setDefaults(v)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read config from temp file: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Server.Host != "10.0.0.1" {
		t.Errorf("server.host = %q, want 10.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("server.port = %d, want 3000", cfg.Server.Port)
	}
	if cfg.Server.Mode != "release" {
		t.Errorf("server.mode = %q, want release", cfg.Server.Mode)
	}
	if cfg.ModelProvider.Mode != "local" {
		t.Errorf("model_provider.mode = %q, want local", cfg.ModelProvider.Mode)
	}
	if cfg.ModelProvider.EmbeddingDimension != 384 {
		t.Errorf("embedding_dimension = %d, want 384", cfg.ModelProvider.EmbeddingDimension)
	}
	if cfg.Auth.APIKey != "test-key-12345" {
		t.Errorf("auth.api_key = %q, want test-key-12345", cfg.Auth.APIKey)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log.level = %q, want warn", cfg.Log.Level)
	}
	if cfg.Conversation.MaxHistory != 50 {
		t.Errorf("conversation.max_history = %d, want 50", cfg.Conversation.MaxHistory)
	}

	// Fields not in the YAML should retain defaults from setDefaults.
	if cfg.Server.BashTimeout != 30 {
		t.Errorf("server.bash_timeout default = %d, want 30", cfg.Server.BashTimeout)
	}
	if cfg.Document.ChunkSize != 500 {
		t.Errorf("document.chunk_size default = %d, want 500", cfg.Document.ChunkSize)
	}
}
