# OJTracker 平台 API 调用指南

在项目目录运行 `go run . -config service.cfg`，然后按下列顺序调用。命令使用默认监听地址。示例中的 ID、账号资料、分数和时间均为演示值。实际调用时请替换账号，并使用创建用户时返回的 ID。

## 1. 创建用户

```sh
base=http://127.0.0.1:8080
curl -i -X POST "$base/users" -H 'Content-Type: application/json' \
  -d '{"name":"Dust"}'
```

响应为 `201 Created`，响应体如下。

```json
{"id":"0123456789abcdef0123456789abcdef","name":"Dust"}
```

将实际返回的 ID 保存到 shell 变量，供后续命令使用。

```sh
id='0123456789abcdef0123456789abcdef'
```

## 平台调用示例

选择对应平台，按绑定、刷新、查询、解绑的顺序调用。每份示例包含请求体、响应体和 HTTP 状态码。

- [Codeforces](platforms/codeforces.md)
- [AtCoder](platforms/atcoder.md)
- [牛客竞赛](platforms/nowcoder.md)

同一用户在每个平台可绑定一个账号。重复绑定相同账号会保留快照，更换账号会清空旧快照。

## 刷新与错误处理

```sh
curl -i -X POST "$base/users/$id/refresh"
```

请求体为空。服务等待所有已绑定平台完成本轮尝试，然后返回结果。全部成功时为 `200 OK`。

```json
{"errors":{}}
```

全局定时任务也会刷新所有用户的绑定。每轮完成后等待 service.cfg 中的 update_interval，默认 1h。

下表中的 GET 均指单平台查询，请求体为空。客户端可根据 HTTP 状态和 error 字段区分用户不存在与平台未绑定。

| 请求场景 | 状态 | 响应体示例 |
|---|---|---|
| GET，用户 ID 不存在 | 404 | `{"error":"user_not_found"}` |
| GET，用户存在但未绑定该平台 | 404 | `{"error":"platform_not_bound"}` |
| GET，平台名不受支持 | 400 | `{"error":"unknown platform"}` |
| GET，已绑定但尚未尝试更新 | 200 | `{"platform":"atcoder","handle":"ac_demo","profile":null,"updated_at":null,"last_error":null}` |
| PUT，Codeforces 或 AtCoder 的绑定请求体为 `{"handle":""}` | 400 | `{"error":"invalid handle"}` |

例如 Codeforces 上游返回 HTTP 503 时，刷新接口返回 `502 Bad Gateway`。errors 只列出失败项，键由用户 ID 和平台名组成。

```json
{"errors":{"0123456789abcdef0123456789abcdef/codeforces":"upstream HTTP 503"}}
```

随后 GET 该平台仍返回 200，last_error 记录错误。profile 和 updated_at 保留上一次成功的值，首次刷新即失败时两者均为 null。
