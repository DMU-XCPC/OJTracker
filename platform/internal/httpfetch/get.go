package httpfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Get 在请求上下文内获取上游 JSON，并限制读取的响应体大小。
func Get(ctx context.Context, c *http.Client, url string, out any) error {
	data, err := GetBytes(ctx, c, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// GetBytes 复用上游请求的取消、HTTP 状态检查和响应大小限制。
func GetBytes(ctx context.Context, c *http.Client, url string) ([]byte, error) {
	// 将调用方上下文传递给上游 GET 请求。
	req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return nil, e
	}
	// 发送请求，确保每条返回路径都关闭已取得的响应体。
	resp, e := c.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	// 仅解析 HTTP 200，其他状态作为上游错误返回。
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	// 多读一个字节，用于识别超过 4 MiB 的响应。
	const limit = 4 << 20
	b, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil {
		return nil, e
	}
	// 超限时直接报错，避免解码截断的数据。
	if len(b) > limit {
		return nil, fmt.Errorf("response too large")
	}
	return b, nil
}
