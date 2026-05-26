package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// DateTimeTool returns the current time in various formats.
// It supports multiple output formats and timezone conversion.
type DateTimeTool struct{}

// inputParams defines the JSON schema for DateTimeTool parameters.
type datetimeParams struct {
	Format   string `json:"format"`   // output format: "unix", "iso", "rfc3339", "date", "time"
	Timezone string `json:"timezone"` // IANA timezone, e.g. "Asia/Shanghai", "America/New_York"
}

// Info —
func (dt *DateTimeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_current_datetime",
		Desc: "获取当前日期时间，支持不同格式和时区。Format 可选值: unix(时间戳), iso(ISO 8601), rfc3339(RFC 3339), date(日期), time(时间)。Timezone 为 IANA 时区名(如 Asia/Shanghai), 不传则用 UTC。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"format": {
				Type:     schema.String,
				Desc:     "输出格式: unix / iso / rfc3339 / date / time",
				Required: false,
			},
			"timezone": {
				Type:     schema.String,
				Desc:     "IANA 时区名称，如 Asia/Shanghai",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 解析 JSON 参数，返回格式化时间。
func (dt *DateTimeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params datetimeParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("datetime tool: invalid args: %w", err)
	}

	now := time.Now()

	// Apply timezone.
	if params.Timezone != "" {
		loc, err := time.LoadLocation(params.Timezone)
		if err != nil {
			return "", fmt.Errorf("datetime tool: invalid timezone %q: %w", params.Timezone, err)
		}
		now = now.In(loc)
	}

	// Format output.
	result := formatTime(now, params.Format)
	return result, nil
}

func formatTime(t time.Time, format string) string {
	switch format {
	case "unix":
		return fmt.Sprintf("%d", t.Unix())
	case "iso":
		return t.Format("2006-01-02T15:04:05-07:00")
	case "rfc3339":
		return t.Format(time.RFC3339)
	case "date":
		return t.Format("2006-01-02")
	case "time":
		return t.Format("15:04:05")
	default:
		return t.Format("2006-01-02 15:04:05 MST")
	}
}

// Compile-time check.
var _ tool.InvokableTool = (*DateTimeTool)(nil)
