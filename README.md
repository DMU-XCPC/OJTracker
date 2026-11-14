# OJTracker

OJTracker 是一款使用 Go 开发的算法竞赛账号管理与追踪服务。用户可将 Codeforces、AtCoder 等平台的账号关联至同一用户 ID，并通过 REST API 查询各平台的竞赛资料与评分。服务支持定时更新和按需刷新。

## 运行

在项目目录执行以下命令。需要 Go 1.22 或更高版本、CGO 和 C 编译器。SQLite 引擎由驱动编译进程序。

```sh
go test ./...
go run . -config service.cfg
```

配置文件使用 JSON。默认监听 127.0.0.1:8080，database 是相对于启动目录的数据库路径。服务启动后立即扫描已有绑定，每轮结束后等待 update_interval（默认 1h）。手动刷新与定时刷新共享请求间隔 request_spacing（默认 2s）。request_timeout 是每次上游请求的超时（默认 15s）。修改配置后重启生效。API 默认面向可信调用方，公开部署时应在入口配置访问控制。

## API

完整请求和响应示例见[平台 API 调用指南](docs/API.md)。

请求与响应使用 JSON。创建用户后，将响应中的 id 用于后续请求。

|方法|路径|请求体或行为|成功状态|
|---|---|---|---|
|POST|/users|{"name":"小林"}|201，返回 id 和 name|
|GET|/users|列出用户 ID 和名称|200|
|GET|/users/{id}|查询用户及所有平台快照|200|
|PUT|/users/{id}|{"name":"林同学"}|200|
|DELETE|/users/{id}|删除用户及其所有绑定|204|
|GET|/users/{id}/accounts/{platform}|按用户 ID 查询单个平台的绑定及快照|200|
|PUT|/users/{id}/accounts/{platform}|{"handle":"tourist"}|204|
|DELETE|/users/{id}/accounts/{platform}|解绑一个平台|204|
|POST|/users/{id}/refresh|同步更新该用户所有绑定|200，返回 {"errors":{}}|

platform 可取 codeforces、atcoder。首次绑定后可直接调用 refresh，否则等待下次定时扫描。重复绑定相同账号保留快照，更换账号会清空旧快照。查询结果中的 accounts 包含 platform、handle、profile、updated_at 和 last_error。未成功抓取时 profile 和 updated_at 为 null。刷新中某个平台失败时，其他平台继续执行，接口返回 502 和按用户 ID/平台标识的错误。失败的平台保留最近成功的数据和更新时间。

```sh
base=http://127.0.0.1:8080
curl -sS -X POST "$base/users" -H 'Content-Type: application/json' -d '{"name":"小林"}'
# 将上一条响应中的 id 填入下一行
id='用户ID'
curl -i -X PUT "$base/users/$id/accounts/codeforces" -H 'Content-Type: application/json' -d '{"handle":"tourist"}'
curl -sS -X POST "$base/users/$id/refresh"
curl -sS "$base/users/$id"
```

## 平台字段

单平台查询执行一次 SQLite 左连接，按用户 ID 和平台查找绑定。用户不存在时返回 404 和 `{"error":"user_not_found"}`，用户存在但未绑定该平台时返回 404 和 `{"error":"platform_not_bound"}`。已绑定但尚无成功快照时返回 200，profile 为 null。查询只读取本地数据。

Codeforces 的 profile 直接返回 user.info 的 User 字段。AtCoder 的 profile 包含 handle、rating、maxRating、ratedContests 和 latestContest。各自的客户端和 Go 类型位于对应的平台目录。原先嵌套在 details 的字段现在直接位于 profile。旧数据库的快照在启动时转换。

所有用户和平台共享 service.cfg 中的 update_interval，每轮扫描全部绑定，完成后等待该间隔。手动刷新不改变下一轮的计划。

## SQLite 表文档

建表语句见 [schema.sql](internal/sqlite/schema.sql)。仅保存当前快照，每次成功刷新会覆盖上一份。

|表|列|SQLite 类型与约束|用途|
|---|---|---|---|
|users|id|TEXT 主键|服务生成的用户 ID|
|users|name|TEXT 必填|可修改、可重名的显示名|
|accounts|user_id|TEXT 必填，外键 users.id|用户删除时级联删除|
|accounts|platform|TEXT 必填|与 user_id 组成复合主键|
|accounts|handle|TEXT 必填|用户绑定的平台账号|
|accounts|revision|TEXT 必填|绑定版本，防止旧请求覆盖重新绑定后的数据|
|accounts|profile|TEXT 可空，JSON 校验|最近成功的公共及平台专属字段|
|accounts|updated_at|TEXT 可空|成功更新时间，UTC RFC3339 格式|
|accounts|last_error|TEXT 可空|最近一次更新错误，成功后清空|
