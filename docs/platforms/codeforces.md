# Codeforces API 示例

先创建服务用户，并在当前 shell 设置 `base` 和 `id`。下列账号与评分快照均为示例。[创建用户与通用错误说明](../API.md)

## 绑定账号

将 Codeforces 用户名作为 `handle` 传入。

```sh
curl -i -X PUT "$base/users/$id/accounts/codeforces" \
  -H 'Content-Type: application/json' -d '{"handle":"cf_demo"}'
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

## 查询Codeforces快照

```sh
curl -i "$base/users/$id/accounts/codeforces"
```

GET 请求体为空。成功响应为 `200 OK`。

```json
{
  "platform": "codeforces",
  "handle": "cf_demo",
  "profile": {
    "handle": "cf_demo",
    "email": null,
    "vkId": null,
    "openId": null,
    "firstName": null,
    "lastName": null,
    "country": null,
    "city": null,
    "organization": null,
    "contribution": 3,
    "rank": "expert",
    "rating": 1720,
    "maxRank": "expert",
    "maxRating": 1830,
    "lastOnlineTimeSeconds": 1735689600,
    "registrationTimeSeconds": 1672531200,
    "friendOfCount": 12,
    "avatar": "https://example.org/avatar.png",
    "titlePhoto": "https://example.org/title.png"
  },
  "updated_at": "2025-01-02T01:02:03Z",
  "last_error": null
}
```

profile 来自 Codeforces 的 user.info。rank 和 maxRank 是当前和历史最高称号。两个以 TimeSeconds 结尾的字段使用 Unix 秒。个人资料和评级中的可选字段可为 null。

## 解绑账号

```sh
curl -i -X DELETE "$base/users/$id/accounts/codeforces"
```

请求体为空。成功返回 `204 No Content`，响应体为空。再次查询该绑定会返回 `404` 和 `{"error":"platform_not_bound"}`。
