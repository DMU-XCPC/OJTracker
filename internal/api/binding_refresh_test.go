package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ojtracker/internal/api"
	"ojtracker/internal/refresh"
	"ojtracker/internal/sqlite"
	"ojtracker/platform"
)

func TestRebindDuringRefresh(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlite.Close(db); err != nil {
			t.Error(err)
		}
	})
	var handler http.Handler
	var path string
	rebind := false
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		// 在上游返回前通过公开接口换绑，确定性地复现旧版本写回。
		if rebind {
			request(t, handler, "PUT", path, `{"handle":"second"}`, 204)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"OK","result":[{"handle":"first","rating":1500}]}`)), Header: http.Header{}}, nil
	})}
	handler = api.NewHandler(db, refresh.NewUpdater(db, platform.NewClients(client), 0))
	user := "/users/" + createUser(t, handler)
	path = user + "/accounts/codeforces"
	request(t, handler, "PUT", path, `{"handle":"first"}`, 204)
	request(t, handler, "POST", user+"/refresh", "", 200)
	before := request(t, handler, "GET", path, "", 200)
	if !strings.Contains(string(before), `"rating":1500`) {
		t.Fatalf("missing snapshot: %s", before)
	}
	request(t, handler, "PUT", path, `{"handle":"first"}`, 204)
	expectJSON(t, request(t, handler, "GET", path, "", 200), string(before))
	// 旧任务应报告绑定变更，新账号仍处于尚未刷新状态。
	rebind = true
	expectJSON(t, request(t, handler, "POST", user+"/refresh", "", 502), `{"errors":{"`+strings.TrimPrefix(user, "/users/")+`/codeforces":"binding changed"}}`)
	expectJSON(t, request(t, handler, "GET", path, "", 200), `{"platform":"codeforces","handle":"second","profile":null,"updated_at":null,"last_error":null}`)
}

// 从真实路由发起请求，并校验状态码、空响应和 JSON 格式。
func request(t *testing.T, h http.Handler, method, path, body string, status int) []byte {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	data := w.Body.Bytes()
	if w.Code != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, status, data)
	}
	if status == 204 {
		if len(data) != 0 {
			t.Fatalf("%s %s: expected empty response, got %s", method, path, data)
		}
	} else if !json.Valid(data) {
		t.Fatalf("%s %s: invalid JSON: %q", method, path, data)
	}
	return data
}

// 结构化比较响应，忽略 JSON 字段顺序，同时区分数组、null 和缺失字段。
func expectJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var actual, expected any
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func createUser(t *testing.T, h http.Handler) string {
	t.Helper()
	data := request(t, h, "POST", "/users", `{"name":"  小林  "}`, 201)
	var u api.User
	if err := json.Unmarshal(data, &u); err != nil {
		t.Fatal(err)
	}
	if len(u.ID) != 32 || u.Name != "小林" {
		t.Fatalf("unexpected created user: %s", data)
	}
	return u.ID
}

// 名称中的引号应原样往返，不能影响其他用户的记录。
func TestQuotedUserName(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlite.Close(db); err != nil {
			t.Error(err)
		}
	})
	h := api.NewHandler(db, nil)
	first, second := createUser(t, h), createUser(t, h)
	renamed := request(t, h, "PUT", "/users/"+first, `{"name":"O'Brien"}`, 200)
	expectJSON(t, renamed, fmt.Sprintf(`{"id":%q,"name":"O'Brien"}`, first))
	expectJSON(t, request(t, h, "GET", "/users/"+second, "", 200), fmt.Sprintf(`{"id":%q,"name":"小林","accounts":[]}`, second))
}
