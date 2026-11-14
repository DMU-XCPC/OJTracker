package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"ojtracker/internal/api"
	"ojtracker/internal/refresh"
	"ojtracker/internal/sqlite"
	"ojtracker/platform"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

// RoundTrip 将测试函数接入 HTTP 客户端的传输层。
func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUserLifecycle(t *testing.T) {
	// 为每次测试创建临时 SQLite 数据库并启用外键。
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := sqlite.Close(db); err != nil {
			t.Error(err)
		}
	}()
	// 用可切换的模拟上游覆盖在线与断网场景。
	offline := false
	c := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if offline {
			return nil, fmt.Errorf("offline")
		}
		var raw string
		switch r.URL.Host {
		case "codeforces.com":
			// 检查账号查询参数，再提供 Codeforces 用户样本。
			if r.URL.Query().Get("handles") != "tourist" {
				t.Fatal(r.URL)
			}
			raw = `{"status":"OK","result":[{"handle":"tourist","rating":3000}]}`
		case "atcoder.jp":
			// 返回计分与非计分场次，覆盖 AtCoder 筛选逻辑。
			raw = `[{"IsRated":true,"NewRating":3500,"EndTime":"2020-01-01T00:00:00Z"},{"IsRated":false,"NewRating":0}]`
		case "ac.nowcoder.com":
			// 校验 UID 路径，并用精简竞赛主页验证快照写入。
			if r.URL.Path != "/acm/contest/profile/123" {
				t.Fatal(r.URL)
			}
			raw = `<a class="coder-name rate-score4">演示</a><script>window.curUser.id = "123";</script><div class="state-num">1800</div><span>Rating</span><div class="state-num">42</div><span>Rating排名</span>`
		default:
			// 收到未知目标主机时报告错误。
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw)), Header: http.Header{}}, nil
	})}
	// 将模拟客户端接入真实业务路由。
	updater := refresh.NewUpdater(db, platform.NewClients(c), 0)
	handler := api.NewHandler(db, updater)
	// 发送测试请求，断言状态码并返回正文供业务断言使用。
	call := func(method, path, body string, status int) string {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	// 创建两个同名用户，验证 ID 独立。
	var u, v api.User
	json.Unmarshal([]byte(call("POST", "/users", `{"name":"A"}`, 201)), &u)
	json.Unmarshal([]byte(call("POST", "/users", `{"name":"A"}`, 201)), &v)
	if u.ID == v.ID {
		t.Fatal("duplicate IDs")
	}
	// 核对用户不存在和平台未绑定两类 404。
	path := "/users/" + u.ID
	if !strings.Contains(call("GET", path+"/accounts/atcoder", "", 404), "platform_not_bound") {
		t.Fatal("missing binding error")
	}
	if !strings.Contains(call("GET", "/users/missing/accounts/atcoder", "", 404), "user_not_found") {
		t.Fatal("missing user error")
	}
	// 为第一个用户绑定两个平台，保留另一个用户作对照。
	for _, p := range []string{"atcoder", "codeforces"} {
		call("PUT", path+"/accounts/"+p, `{"handle":"tourist"}`, 204)
	}
	// 牛客绑定使用数字 UID，昵称应被拒绝。
	call("PUT", path+"/accounts/nowcoder", `{"handle":"demo"}`, 400)
	call("PUT", path+"/accounts/nowcoder", `{"handle":"123"}`, 204)
	// 启动后台调度器并记录其退出通知。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { updater.Schedule(ctx, time.Hour); close(done) }()
	// 限时等待三份快照入库，超时先停止任务再报告失败。
	deadline := time.Now().Add(3 * time.Second)
	for {
		var count int64
		if e = db.Model(&sqlite.Account{}).Where("profile IS NOT NULL").Count(&count).Error; e != nil {
			t.Fatal(e)
		}
		if count == 3 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("scheduler timeout")
		}
		time.Sleep(time.Millisecond)
	}
	// 等待后台任务停止，单独验证主动刷新链路。
	cancel()
	<-done
	call("POST", path+"/refresh", "", 200)
	// 检查各平台的专属字段，排除旧版 details 包装。
	for platform, field := range map[string]string{"codeforces": "friendOfCount", "atcoder": "ratedContests", "nowcoder": `"ratingRank":42`} {
		data := call("GET", path+"/accounts/"+platform, "", 200)
		if !strings.Contains(data, field) || strings.Contains(data, `"details"`) {
			t.Fatal(data)
		}
	}
	// 模拟上游断网，验证刷新失败后仍能查询最近成功的评分。
	offline = true
	call("POST", path+"/refresh", "", 502)
	data := call("GET", path, "", 200)
	for _, part := range []string{`"rating":3000`, `"rating":3500`, "offline"} {
		if !strings.Contains(data, part) {
			t.Fatal(data)
		}
	}
	// 验证换绑、改名与删除流程，以及另一个用户的数据隔离。
	call("PUT", path+"/accounts/codeforces", `{"handle":"new"}`, 204)
	call("PUT", path, `{"name":"B"}`, 200)
	call("DELETE", path, "", 204)
	call("GET", path, "", 404)
	call("GET", "/users/"+v.ID, "", 200)
	// 直接核对绑定表，确认级联删除已生效。
	var count int64
	if e = db.Model(&sqlite.Account{}).Count(&count).Error; e != nil || count != 0 {
		t.Fatal(count, e)
	}
}
