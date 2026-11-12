package refresh

import (
	"context"
	"log"
	"time"
)

// Schedule 启动后立即刷新，每轮完成后再等待全局配置的间隔。
func (s *Updater) Schedule(ctx context.Context, interval time.Duration) {
	for {
		// 执行全量刷新；停机时跳过本轮错误日志。
		issues, e := s.Refresh(ctx, "")
		if ctx.Err() != nil {
			return
		}
		// 汇总记录数据库故障和各绑定的刷新错误。
		if e != nil || len(issues) > 0 {
			log.Printf("scheduled refresh: errors=%v db=%v", issues, e)
		}
		// 等待全局间隔或退出信号，避免轮次重叠。
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
