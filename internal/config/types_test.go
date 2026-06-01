package config

import (
	"encoding/json"
	"testing"
)

// TestModelEntry_JSONTags - ModelEntry has both mapstructure and json tags
func TestModelEntry_JSONTags(t *testing.T) {
	entry := ModelEntry{
		Name:           "gpt-4",
		Provider:       "openai",
		ChatModel:      "gpt-4",
		APIKey:         "sk-xxx",
		BaseURL:        "https://api.openai.com/v1",
		EmbeddingModel: "text-embedding-3-small",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded ModelEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Name != "gpt-4" || decoded.APIKey != "sk-xxx" {
		t.Errorf("round-trip: %+v", decoded)
	}
}

func TestModelEntry_Empty(t *testing.T) {
	raw := `{}`
	var entry ModelEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if entry.Name != "" || entry.Provider != "" {
		t.Errorf("expected zero values, got %+v", entry)
	}
}

// TestConfig_DefaultStruct - Config uses mapstructure tags, test through struct construction
func TestConfig_DefaultStruct(t *testing.T) {
	cfg := Config{}
	if cfg.Server.Port != 0 || cfg.Auth.APIKey != "" {
		t.Errorf("expected zero values, got %+v", cfg)
	}
}

func TestServerConfig_Values(t *testing.T) {
	sc := ServerConfig{
		Host: "0.0.0.0", Port: 8080, RateLimitRPS: 10,
	}
	if sc.Host != "0.0.0.0" || sc.Port != 8080 || sc.RateLimitRPS != 10 {
		t.Errorf("got %+v", sc)
	}
}

func TestModelEntry_StructConstruction(t *testing.T) {
	e := ModelEntry{Name: "gpt-4", Provider: "openai", ChatModel: "gpt-4"}
	if e.Name != "gpt-4" || e.Provider != "openai" {
		t.Errorf("got %+v", e)
	}
}

func TestElasticsearchConfig_Values(t *testing.T) {
	ec := ElasticsearchConfig{
		Addresses: []string{"http://localhost:9200"},
		IndexName: "vectors", Username: "user", Password: "pass",
	}
	if len(ec.Addresses) != 1 || ec.Addresses[0] != "http://localhost:9200" {
		t.Errorf("addresses: %v", ec.Addresses)
	}
}

func TestDocumentConfig_Values(t *testing.T) {
	dc := DocumentConfig{
		MaxFileSize: 10485760, ChunkSize: 1000, ChunkOverlap: 200,
		AllowedTypes: []string{".pdf", ".txt"},
	}
	if dc.MaxFileSize != 10485760 || dc.ChunkSize != 1000 {
		t.Errorf("got %+v", dc)
	}
}

func TestCronJobConfig_Values(t *testing.T) {
	cjc := CronJobConfig{Name: "daily-report", CronExpr: "0 9 * * *", Prompt: "generate report", AutoApprove: true}
	if cjc.Name != "daily-report" || !cjc.AutoApprove {
		t.Errorf("got %+v", cjc)
	}
}

func TestLogConfig_Values(t *testing.T) {
	lc := LogConfig{Level: "info", FilePath: "/var/log/app.log", MaxSize: 100, MaxBackups: 3, MaxAge: 30, Compress: true}
	if lc.Level != "info" || lc.MaxSize != 100 || !lc.Compress {
		t.Errorf("got %+v", lc)
	}
}

func TestStockConfig_Default(t *testing.T) {
	sc := StockConfig{}
	if sc.DataDir != "" {
		t.Errorf("expected empty DataDir, got %q", sc.DataDir)
	}
}

func TestAgentConfig_Values(t *testing.T) {
	ac := AgentConfig{SystemPrompt: "You are a helpful assistant."}
	if ac.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("got %q", ac.SystemPrompt)
	}
}

func TestConversationConfig_Values(t *testing.T) {
	cc := ConversationConfig{MaxHistory: 50, StorageType: "elasticsearch"}
	if cc.MaxHistory != 50 || cc.StorageType != "elasticsearch" {
		t.Errorf("got %+v", cc)
	}
}

func TestVectorStoreConfig_Values(t *testing.T) {
	vc := VectorStoreConfig{Type: "elasticsearch"}
	if vc.Type != "elasticsearch" {
		t.Errorf("type = %q", vc.Type)
	}
}

func TestLocalProviderConfig_Values(t *testing.T) {
	lpc := LocalProviderConfig{Enabled: true, BaseURL: "http://localhost:11434", ChatModel: "llama3"}
	if !lpc.Enabled || lpc.ChatModel != "llama3" {
		t.Errorf("got %+v", lpc)
	}
}

func TestCloudProviderConfig_Values(t *testing.T) {
	cpc := CloudProviderConfig{Enabled: true, Type: "openai", APIKey: "sk-xxx", ChatModel: "gpt-4"}
	if !cpc.Enabled || cpc.ChatModel != "gpt-4" {
		t.Errorf("got %+v", cpc)
	}
}

func TestCronConfig_Values(t *testing.T) {
	cc := CronConfig{
		Enabled: true,
		Jobs: []CronJobConfig{
			{Name: "job1", CronExpr: "*/5 * * * *"},
		},
	}
	if !cc.Enabled || len(cc.Jobs) != 1 {
		t.Errorf("got %+v", cc)
	}
}

func TestSearchConfig_Values(t *testing.T) {
	sc := SearchConfig{Enabled: true, Engine: "duckduckgo", Format: "markdown"}
	if !sc.Enabled || sc.Engine != "duckduckgo" {
		t.Errorf("got %+v", sc)
	}
}
