package refresh

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"time"

	"ojtracker/platform"
	"ojtracker/platform/atcoder"
	"ojtracker/platform/codeforces"
	"ojtracker/platform/nowcoder"
)

// Updater 持有定时和手动刷新共享的客户端、请求令牌与间隔。
type Updater struct {
	db      *sql.DB
	clients platform.Clients
	gate    chan struct{}
	next    time.Time
	spacing time.Duration
}

func NewUpdater(db *sql.DB, clients platform.Clients, spacing time.Duration) *Updater {
	// 容量为一的通道协调手动刷新与定时任务。
	return &Updater{
		db:      db,
		clients: clients,
		gate:    make(chan struct{}, 1),
		spacing: spacing,
	}
}

type binding struct {
	user     string
	platform string
	handle   string
	revision string
}

// Refresh 在 uid 为空时扫描全部绑定；指定 uid 时只刷新该用户。
func (s *Updater) Refresh(ctx context.Context, uid string) (map[string]string, error) {
	// 由用户 ID 决定查询单个用户还是扫描全部绑定。
	q := "SELECT user_id,platform,handle,revision FROM accounts"
	var args []any
	if uid != "" {
		q += " WHERE user_id=?"
		args = append(args, uid)
	}
	// 固定用户和平台的排序，使抓取顺序稳定。
	rows, e := s.db.QueryContext(ctx, q+" ORDER BY user_id,platform", args...)
	if e != nil {
		return nil, e
	}
	// 保存当前绑定版本，以便抓取完成后校验写入目标。
	var bindings []binding
	for rows.Next() {
		var b binding
		if e = rows.Scan(&b.user, &b.platform, &b.handle, &b.revision); e != nil {
			rows.Close()
			return nil, e
		}
		bindings = append(bindings, b)
	}
	// 抓取前释放查询持有的连接，后续更新需要使用同一个单连接池。
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
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
			out[b.user+"/"+b.platform] = msg
		}
	}
	return out, nil
}

func (s *Updater) update(ctx context.Context, b binding) (string, error) {
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
	switch b.platform {
	case "codeforces":
		// 用 Codeforces 用户资料类型解码，再编码为快照。
		var u codeforces.User
		u, e = s.clients.Codeforces.Fetch(ctx, b.handle)
		if e == nil {
			raw, e = json.Marshal(u)
		}
	case "atcoder":
		// 用 AtCoder 计分摘要类型生成快照。
		var snapshot atcoder.Snapshot
		snapshot, e = s.clients.AtCoder.Fetch(ctx, b.handle)
		if e == nil {
			raw, e = json.Marshal(snapshot)
		}
	case "nowcoder":
		// 将公开竞赛主页解析为牛客专属快照。
		var snapshot nowcoder.Snapshot
		snapshot, e = s.clients.Nowcoder.Fetch(ctx, b.handle)
		if e == nil {
			raw, e = json.Marshal(snapshot)
		}
	default:
		// 数据库中的平台名没有对应适配器时终止本轮。
		return "", fmt.Errorf("unknown stored platform %q", b.platform)
	}
	// revision 限定写入时的绑定版本，避免迟到的响应覆盖新账号的数据。
	var res sql.Result
	msg := ""
	if e != nil {
		msg = e.Error()
		// 抓取失败时保留上一次成功的快照和更新时间。
		const recordFailure = `
 UPDATE accounts SET last_error = ?
 WHERE revision = ? AND user_id = ? AND platform = ?`
		res, e = s.db.ExecContext(ctx, recordFailure, msg, b.revision, b.user, b.platform)
	} else {
		// 成功时一起替换快照和时间戳，并清除先前错误。
		const saveSnapshot = `
 UPDATE accounts SET profile = ?, updated_at = ?, last_error = NULL
 WHERE revision = ? AND user_id = ? AND platform = ?`
		updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
		res, e = s.db.ExecContext(ctx, saveSnapshot,
			string(raw), updatedAt, b.revision, b.user, b.platform,
		)
	}
	// 将写库错误与单个平台的抓取错误分别返回。
	if e != nil {
		return "", e
	}
	// 零行更新表示绑定已删除或已换绑，丢弃旧结果。
	if n, _ := res.RowsAffected(); n == 0 {
		return "binding changed", nil
	}
	return msg, nil
}
