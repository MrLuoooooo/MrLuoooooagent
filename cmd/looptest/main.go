// looptest 最小 trace 上报探针：排除 span 内容导致的罗盘 500。
// 用法：go run ./cmd/looptest（token 从 .env 的 GOAGENT_COZELOOP_API_TOKEN 读）
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coze-dev/cozeloop-go"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	token := strings.TrimSpace(os.Getenv("GOAGENT_COZELOOP_API_TOKEN"))
	wsID := strings.TrimSpace(os.Getenv("GOAGENT_COZELOOP_WORKSPACE_ID"))
	if token == "" || wsID == "" {
		fmt.Println("FATAL: token/workspace_id 为空")
		os.Exit(1)
	}

	client, err := cozeloop.NewClient(
		cozeloop.WithAPIToken(token),
		cozeloop.WithWorkspaceID(wsID),
	)
	if err != nil {
		fmt.Println("FATAL: new client:", err)
		os.Exit(1)
	}

	ctx, span := client.StartSpan(context.Background(), "minimal_probe", "custom")
	span.SetInput(ctx, "hello")
	span.SetOutput(ctx, "world")
	span.Finish(ctx)

	// 给异步队列留出上报窗口，观察 stderr 的 cozeloop 错误输出
	fmt.Println("span finished, waiting 30s for export...")
	time.Sleep(30 * time.Second)
	client.Close(ctx)
	fmt.Println("closed. 若上方无 [Error] [cozeloop] 输出即上报成功")
}
