package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ojtracker/internal/api"
	"ojtracker/internal/config"
	"ojtracker/internal/refresh"
	"ojtracker/internal/sqlite"
	"ojtracker/platform"
)

func main() {
	// 解析配置路径，省略参数时使用默认文件。
	path := flag.String("config", "service.cfg", "configuration file")
	flag.Parse()
	cfg, e := config.Load(*path)
	if e != nil {
		log.Fatal(e)
	}
	// 初始化数据库，服务退出时释放连接。
	db, e := sqlite.Open(cfg.Database)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()

	// 创建服务并让定时任务监听系统退出信号。
	clients := platform.NewClients(&http.Client{Timeout: cfg.Timeout})
	updater := refresh.NewUpdater(db, clients, cfg.Spacing)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go updater.Schedule(ctx, cfg.Interval)

	// 挂载业务路由并限制请求头读取时间。
	srv := http.Server{
		Addr:              cfg.Listen,
		Handler:           api.NewHandler(db, updater),
		ReadHeaderTimeout: 5 * time.Second,
	}
	// 收到退出信号后关闭 HTTP 服务。
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	// 区分正常关闭和需要报告的监听错误。
	log.Printf("listening on %s", cfg.Listen)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
