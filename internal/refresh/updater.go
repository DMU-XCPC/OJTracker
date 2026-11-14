package refresh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"ojtracker/internal/sqlite"
	"ojtracker/platform"
	"ojtracker/platform/atcoder"
	"ojtracker/platform/codeforces"
	"ojtracker/platform/nowcoder"
)

// Updater 持有定时和手动刷新共享的客户端、请求令牌与间隔。
type Updater struct {
	db      *gorm.DB
	clients platform.Clients
	gate    chan struct{}
	next    time.Time
	spacing time.Duration
}

func NewUpdater(db *gorm.DB, clients platform.Clients, spacing time.Duration) *Updater {
	// 容量为一的通道协调手动刷新与定时任务。
	return &Updater{
		db:      db,
		clients: clients,
		gate:    make(chan struct{}, 1),
		spacing: spacing,
	}
}

// Refresh 在 uid 为空时扫描全部绑定；指定 uid 时只刷新该用户。
func (s *Updater) Refresh(ctx context.Context, uid string) (map[string]string, error) {
	bindings, err := sqlite.ListRefreshBindings(s.db.WithContext(ctx), uid)
	if err != nil {
		return nil, err
	}
	// 逐个更新；数据库故障终止本轮，单个绑定失败后继续处理其他项。
	out := map[string]string{}
	for _, b := range bindings {
		msg, e := s.update(ctx, b)
		if e != nil {
			return nil, e
		}
		// 以用户 ID 和平台组成错误键，便于定位失败项。
		if msg != "" {
			out[b.UserID+"/"+b.Platform] = msg
		}
	}
	return out, nil
}

func (s *Updater) update(ctx context.Context, b sqlite.Account) (string, error) {
	// 等待全局刷新令牌，调用方取消时提前退出。
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	// 手动刷新和定时任务共用 gate；持有它的任务才能读写下次请求时间。
	defer func() { <-s.gate }()
	// 等待请求间隔期间也响应取消。
	if d := time.Until(s.next); d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	// 按请求开始时间计算下一次允许访问上游的时间。
	s.next = time.Now().Add(s.spacing)
	var raw []byte
	var e error
	switch b.Platform {
	case "codeforces":
		// 用 Codeforces 用户资料类型解码，再编码为快照。
		var u codeforces.User
		u, e = s.clients.Codeforces.Fetch(ctx, b.Handle)
		if e == nil {
			raw, e = json.Marshal(u)
		}
	case "atcoder":
		// 用 AtCoder 计分摘要类型生成快照。
		var snapshot atcoder.Snapshot
		snapshot, e = s.clients.AtCoder.Fetch(ctx, b.Handle)
		if e == nil {
			raw, e = json.Marshal(snapshot)
		}
	case "nowcoder":
		// 将公开竞赛主页解析为牛客专属快照。
		var snapshot nowcoder.Snapshot
		snapshot, e = s.clients.Nowcoder.Fetch(ctx, b.Handle)
		if e == nil {
			raw, e = json.Marshal(snapshot)
		}
	default:
		// 数据库中的平台名没有对应适配器时终止本轮。
		return "", fmt.Errorf("unknown stored platform %q", b.Platform)
	}
	// revision 限定写入时的绑定版本，避免迟到的响应覆盖新账号的数据。
	msg := ""
	changes := map[string]any{}
	if e != nil {
		// 失败只更新错误信息，保留最近成功的快照。
		msg = e.Error()
		changes["last_error"] = msg
	} else {
		changes["profile"] = string(raw)
		changes["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		changes["last_error"] = nil
	}
	updated, err := sqlite.UpdateBindingSnapshot(s.db.WithContext(ctx), b, changes)
	if err != nil {
		return "", err
	}
	if !updated {
		return "binding changed", nil
	}
	return msg, nil
}
