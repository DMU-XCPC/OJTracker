package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config 保存进程级配置，时长在启动时完成解析。
type Config struct {
	Listen   string
	Database string
	Interval time.Duration // 两轮全量刷新之间的等待时间。
	Timeout  time.Duration // 单次上游 HTTP 请求的超时。
	Spacing  time.Duration // 相邻两次上游请求开始时间的最小间隔。
}

// Load 读取并验证全局配置。
func Load(path string) (Config, error) {
	// 从指定路径读取进程配置。
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	// 先按 JSON 原始字符串解码，再校验时长单位。
	var raw struct {
		Listen, Database string
		Interval         string `json:"update_interval"`
		Timeout          string `json:"request_timeout"`
		Spacing          string `json:"request_spacing"`
	}
	if err = json.Unmarshal(data, &raw); err != nil {
		return cfg, err
	}
	// 确认监听地址和数据库路径齐全。
	if raw.Listen == "" || raw.Database == "" {
		return cfg, fmt.Errorf("listen and database required")
	}
	cfg.Listen, cfg.Database = raw.Listen, raw.Database
	// 分别解析三个时长，错误消息标明具体配置项。
	if cfg.Interval, err = positiveDuration("update_interval", raw.Interval); err != nil {
		return cfg, err
	}
	if cfg.Timeout, err = positiveDuration("request_timeout", raw.Timeout); err != nil {
		return cfg, err
	}
	cfg.Spacing, err = positiveDuration("request_spacing", raw.Spacing)
	return cfg, err
}

func positiveDuration(field, value string) (time.Duration, error) {
	// 配置必须是 Go 支持的正时长，例如 15s 或 1h。
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s requires a positive duration, got %q", field, value)
	}
	return d, nil
}
