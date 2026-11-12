// Package nowcoder 读取牛客公开竞赛主页上的账号和评分资料。
package nowcoder

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"ojtracker/platform/internal/httpfetch"
)

type Client struct{ HTTP *http.Client }
type Request struct{ UID string }
type Snapshot struct {
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	Rating   *int   `json:"rating"`
	Rank     *int   `json:"ratingRank"`
}

// 牛客主页按数字 UID 路由。评分及排名按旁边的标签识别，忽略其他统计项。
var (
	uidPattern = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)
	pageID     = regexp.MustCompile(`window\.curUser\.id\s*=\s*"([0-9]+)"`)
	nameTag    = regexp.MustCompile(`<a\s+[^>]*class="coder-name[^"]*"[^>]*>\s*([^<]+?)\s*</a>`)
	stats      = regexp.MustCompile(`<div\s+class="state-num[^"]*"[^>]*>\s*([^<]*)</div>\s*<span>\s*(Rating|Rating排名)\s*</span>`)
)

func ValidUID(uid string) bool { return uidPattern.MatchString(uid) }

func (c Client) Fetch(ctx context.Context, uid string) (Snapshot, error) {
	// 构造公开主页请求，拒绝昵称和路径片段。
	request := Request{UID: uid}
	if !ValidUID(request.UID) {
		return Snapshot{}, fmt.Errorf("invalid nowcoder UID")
	}
	body, err := httpfetch.GetBytes(ctx, c.HTTP, "https://ac.nowcoder.com/acm/contest/profile/"+request.UID)
	if err != nil {
		return Snapshot{}, err
	}
	return parseProfile(request.UID, string(body))
}

func parseProfile(uid, page string) (Snapshot, error) {
	// 错误页也可能返回 HTTP 200；核对页面身份再接受统计值。
	id, name := pageID.FindStringSubmatch(page), nameTag.FindStringSubmatch(page)
	if id == nil || id[1] != uid || name == nil {
		return Snapshot{}, fmt.Errorf("nowcoder profile unavailable for UID %s", uid)
	}
	result := Snapshot{UID: uid, Nickname: html.UnescapeString(strings.TrimSpace(name[1]))}
	fields := stats.FindAllStringSubmatch(page, -1)
	if len(fields) == 0 {
		return Snapshot{}, fmt.Errorf("nowcoder rating statistics missing")
	}
	// 无排名或无评分的占位符映射为 null；遇到未知格式时保留旧快照。
	for _, field := range fields {
		value := strings.TrimSpace(field[1])
		switch value {
		case "", "-", "--", "暂无", "未上榜", "暂无排名":
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return Snapshot{}, fmt.Errorf("invalid nowcoder %s: %q", field[2], value)
		}
		if field[2] == "Rating" {
			result.Rating = &n
		} else {
			result.Rank = &n
		}
	}
	return result, nil
}
