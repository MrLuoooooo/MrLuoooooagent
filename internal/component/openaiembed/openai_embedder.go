package openaiembed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

// OpenAIEmbedder 调 OpenAI 兼容 embedding 接口做文本向量化。
type OpenAIEmbedder struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIEmbedder 初始化 embedding 客户端。
func NewOpenAIEmbedder(apiKey, model, baseURL string) embedding.Embedder {
	return &OpenAIEmbedder{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// EmbedStrings 批量把文本转成向量，超 2048 条自动分批。
func (e *OpenAIEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	const maxBatchSize = 2048

	allEmbeddings := make([][]float64, 0, len(texts))

	for i := 0; i < len(texts); i += maxBatchSize {
		end := min(i+maxBatchSize, len(texts))
		batch := texts[i:end]
		embeddings, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed: %w", err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

func (e *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	reqBody := openAIEmbedRequest{
		Model: e.model,
		Input: texts,
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := e.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI error (%d): %s", resp.StatusCode, string(body))
	}

	var respBody openAIEmbedResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	embeddings := make([][]float64, len(respBody.Data))
	for _, item := range respBody.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, nil
}

type openAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbedResponse struct {
	Object string          `json:"object"`
	Data   []openAIEmbedData `json:"data"`
	Model  string          `json:"model"`
	Usage  openAIUsage     `json:"usage"`
}

type openAIEmbedData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type openAIUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

var _ embedding.Embedder = (*OpenAIEmbedder)(nil)