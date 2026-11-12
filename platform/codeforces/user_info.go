package codeforces

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"ojtracker/platform/internal/httpfetch"
)

type Client struct{ HTTP *http.Client }
type Request struct{ Handles string }
type Response struct {
	Status  string
	Comment string
	Result  []User
}

// User 对应 user.info 返回的用户资料。指针字段保留缺失值与零值的区别。
type User struct {
	Handle                  string  `json:"handle"`
	Email                   *string `json:"email"`
	VkId                    *string `json:"vkId"`
	OpenId                  *string `json:"openId"`
	FirstName               *string `json:"firstName"`
	LastName                *string `json:"lastName"`
	Country                 *string `json:"country"`
	City                    *string `json:"city"`
	Organization            *string `json:"organization"`
	Contribution            int     `json:"contribution"`
	Rank                    *string `json:"rank"`
	Rating                  *int    `json:"rating"`
	MaxRank                 *string `json:"maxRank"`
	MaxRating               *int    `json:"maxRating"`
	LastOnlineTimeSeconds   int64   `json:"lastOnlineTimeSeconds"`
	RegistrationTimeSeconds int64   `json:"registrationTimeSeconds"`
	FriendOfCount           int     `json:"friendOfCount"`
	Avatar                  string  `json:"avatar"`
	TitlePhoto              string  `json:"titlePhoto"`
}

func (c Client) Fetch(ctx context.Context, h string) (User, error) {
	// 将单个账号编码到 user.info 的 handles 参数中。
	req := Request{Handles: h}
	var r Response
	query := url.Values{"handles": {req.Handles}}
	endpoint := "https://codeforces.com/api/user.info?" + query.Encode()
	// 获取并解码上游响应，传输或解析失败时直接返回错误。
	e := httpfetch.Get(ctx, c.HTTP, endpoint, &r)
	if e != nil {
		return User{}, e
	}
	// 校验业务状态、人数和账号名后，取出唯一用户。
	if r.Status != "OK" || len(r.Result) != 1 || r.Result[0].Handle == "" {
		return User{}, fmt.Errorf("invalid user.info response: %s", r.Comment)
	}
	return r.Result[0], nil
}
