package platform

import (
	"net/http"

	"ojtracker/platform/atcoder"
	"ojtracker/platform/codeforces"
)

// Clients 集中管理各平台的具体客户端。
type Clients struct {
	Codeforces codeforces.Client
	AtCoder    atcoder.Client
}

// NewClients 为各平台注入同一个 HTTP 客户端，统一超时和连接复用策略。
func NewClients(client *http.Client) Clients {
	return Clients{
		Codeforces: codeforces.Client{HTTP: client},
		AtCoder:    atcoder.Client{HTTP: client},
	}
}
