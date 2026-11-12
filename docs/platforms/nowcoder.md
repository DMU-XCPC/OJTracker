# 牛客竞赛 API 示例

先创建服务用户，并在当前 shell 设置 `base` 和 `id`。下列账号与评分快照均为示例。[创建用户与通用错误说明](../API.md)

## 绑定账号

从竞赛主页路径 `/acm/contest/profile/{UID}` 取数字 UID，以字符串传入 `handle`。传入昵称会返回 `400` 和 `{"error":"invalid nowcoder UID"}`。

```sh
curl -i -X PUT "$base/users/$id/accounts/nowcoder" \
  -H 'Content-Type: application/json' -d '{"handle":"123456789"}'
```

成功返回 `204 No Content`，响应体为空。账号的可用性在刷新时确认。

## 刷新该用户的所有绑定

```sh
curl -i -X POST "$base/users/$id/refresh"
```

请求体为空。服务等待所有已绑定平台完成本轮尝试，然后返回结果。全部成功时为 `200 OK`。

```json
{"errors":{}}
```

## 查询牛客竞赛快照

```sh
curl -i "$base/users/$id/accounts/nowcoder"
```

GET 请求体为空，成功时返回 `200 OK`。

```json
{
  "platform": "nowcoder",
  "handle": "123456789",
  "profile": {
    "uid": "123456789",
    "nickname": "示例选手",
    "rating": 1800,
    "ratingRank": 42
  },
  "updated_at": "2026-11-12T08:00:00Z",
  "last_error": null
}
```

评分或排名为空时，对应字段为 null。

## 解绑账号

```sh
curl -i -X DELETE "$base/users/$id/accounts/nowcoder"
```

请求体为空。成功返回 `204 No Content`，响应体为空。再次查询该绑定会返回 `404` 和 `{"error":"platform_not_bound"}`。
