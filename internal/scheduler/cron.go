package scheduler

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

var approvalPattern = regexp.MustCompile(`\[NEEDS_APPROVAL\]\s*\n([\s\S]*?)\[\/NEEDS_APPROVAL\]`)

// CronScheduler manages periodic stock analysis jobs using the agent graph.
type CronScheduler struct {
	cron       *cron.Cron
	agentGraph compose.Runnable[*schema.Message, *schema.Message]
	logger     *zap.Logger
	approvals  *service.ApprovalStore
}

// NewCronScheduler 启动定时任务调度器。
func NewCronScheduler(
	cfg *config.Config,
	agentGraph compose.Runnable[*schema.Message, *schema.Message],
	logger *zap.Logger,
	approvals *service.ApprovalStore,
) *CronScheduler {
	if approvals == nil {
		approvals = service.NewApprovalStore("data")
	}
	cs := &CronScheduler{
		// WithSeconds 开启秒级支持，适配 "0 30 9 * * 1-5" 这类 6 字段表达式
		cron:       cron.New(cron.WithLocation(time.Local), cron.WithSeconds()),
		agentGraph: agentGraph,
		logger:     logger,
		approvals:  approvals,
	}

	if !cfg.Cron.Enabled {
		logger.Info("cron: scheduler not enabled")
		return cs
	}

	for _, job := range cfg.Cron.Jobs {
		job := job
		_, err := cs.cron.AddFunc(job.CronExpr, func() {
			// cron job 跑在独立 goroutine，panic 会直接崩进程，必须拦截
			defer func() {
				if r := recover(); r != nil {
					cs.logger.Error("cron: job panic recovered",
						zap.String("name", job.Name),
						zap.Any("err", r),
						zap.Stack("stack"),
					)
				}
			}()
			cs.executeJob(job)
		})
		if err != nil {
			logger.Error("cron: failed to add job",
				zap.String("name", job.Name),
				zap.String("cron", job.CronExpr),
				zap.Error(err),
			)
			continue
		}
		logger.Info("cron: job registered",
			zap.String("name", job.Name),
			zap.String("cron", job.CronExpr),
			zap.Bool("auto_approve", job.AutoApprove),
		)
	}

	cs.cron.Start()
	logger.Info("cron: scheduler started", zap.Int("jobs", len(cs.cron.Entries())))
	return cs
}

func (cs *CronScheduler) executeJob(job config.CronJobConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cs.logger.Info("cron: executing job", zap.String("name", job.Name))

	userMsg := &schema.Message{Role: schema.User, Content: job.Prompt}
	result, err := cs.agentGraph.Invoke(ctx, userMsg)
	if err != nil {
		cs.logger.Error("cron: job execution failed",
			zap.String("name", job.Name),
			zap.Error(err),
		)
		return
	}

	output := result.Content
	cs.logger.Info("cron: job completed",
		zap.String("name", job.Name),
		zap.Int("output_len", len(output)),
	)

	// Parse [NEEDS_APPROVAL] blocks.
	matches := approvalPattern.FindAllStringSubmatch(output, -1)
	if len(matches) > 0 && !job.AutoApprove {
		for _, m := range matches {
			approval := parseApprovalBlock(m[1])
			item := &model.ApprovalItem{
				ID:         uuid.New().String()[:12],
				CreatedAt:  time.Now(),
				Source:     "cron",
				TaskName:   job.Name,
				ActionType: approval["action_type"],
				RiskLevel:  approval["risk_level"],
				Reason:     approval["reason"],
				Prompt:     job.Prompt,
				FullOutput: output,
				Status:     model.ApprovalPending,
			}
			cs.approvals.Add(item)
			cs.logger.Info("cron: approval item created",
				zap.String("id", item.ID),
				zap.String("task", job.Name),
				zap.String("risk", item.RiskLevel),
			)
		}
	} else {
		cs.logger.Info("cron: no approval needed (auto_approve=true or no APPROVAL block)",
			zap.String("name", job.Name),
		)
	}
}

func parseApprovalBlock(text string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			result[key] = val
		}
	}
	return result
}

// Stop shuts down the cron scheduler.
func (cs *CronScheduler) Stop() {
	cs.cron.Stop()
	cs.logger.Info("cron: scheduler stopped")
}
