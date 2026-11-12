package atcoder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"ojtracker/platform/internal/httpfetch"
)

type Client struct{ HTTP *http.Client }
type Request struct{ Handle string }

// Contest 保留竞赛历史接口的原始字段名。
type Contest struct {
	IsRated           bool
	NewRating         int
	OldRating         int
	Place             int
	ContestScreenName string
	EndTime           time.Time
}
type Response []Contest

// Snapshot 汇总计分比赛；没有计分记录时，评分和最近比赛均为 nil。
type Snapshot struct {
	Handle        string   `json:"handle"`
	Rating        *int     `json:"rating"`
	MaxRating     *int     `json:"maxRating"`
	RatedContests int      `json:"ratedContests"`
	LatestContest *Contest `json:"latestContest"`
}

func (c Client) Fetch(ctx context.Context, h string) (Snapshot, error) {
	// 将账号编码为 URL 路径段并请求比赛历史。
	req := Request{Handle: h}
	var history Response
	e := httpfetch.Get(ctx, c.HTTP, "https://atcoder.jp/users/"+url.PathEscape(req.Handle)+"/history/json", &history)
	if e != nil {
		return Snapshot{}, e
	}
	// null 表示缺少历史，空数组则是有效的空记录。
	if history == nil {
		return Snapshot{}, fmt.Errorf("missing history")
	}
	var latest *Contest
	var peak *int
	count := 0
	// 按结束时间判断最近一场，避免依赖上游返回的排列顺序。
	for i := range history {
		r := &history[i]
		// 仅计分比赛参与场次和评分统计。
		if !r.IsRated {
			continue
		}
		count++
		// 按结束时间更新最近一场计分比赛。
		if latest == nil || r.EndTime.After(latest.EndTime) {
			latest = r
		}
		// 单独维护历次赛后评分的最大值。
		if peak == nil || r.NewRating > *peak {
			peak = &r.NewRating
		}
	}
	// 组装摘要，保留无计分比赛时的可空字段。
	p := Snapshot{
		Handle:        h,
		MaxRating:     peak,
		RatedContests: count,
		LatestContest: latest,
	}
	// 当前评分取最近一场计分比赛的赛后分。
	if latest != nil {
		p.Rating = &latest.NewRating
	}
	return p, nil
}
