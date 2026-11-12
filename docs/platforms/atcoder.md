# AtCoder API 示例

先创建服务用户，并在当前 shell 设置 `base` 和 `id`。下列账号与评分快照均为示例。[创建用户与通用错误说明](../API.md)

## 绑定账号

将 AtCoder 用户名作为 `handle` 传入。

```sh
curl -i -X PUT "$base/users/$id/accounts/atcoder" \
  -H 'Content-Type: application/json' -d '{"handle":"ac_demo"}'
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

## 查询AtCoder快照

```sh
curl -i "$base/users/$id/accounts/atcoder"
```

GET 请求体为空。成功响应为 `200 OK`。

```json
{
  "platform": "atcoder",
  "handle": "ac_demo",
  "profile": {
    "handle": "ac_demo",
    "rating": 1420,
    "maxRating": 1510,
    "ratedContests": 18,
    "latestContest": {
      "IsRated": true,
      "NewRating": 1420,
      "OldRating": 1400,
      "Place": 315,
      "ContestScreenName": "abc380",
      "EndTime": "2024-11-16T22:40:00+09:00"
    }
  },
  "updated_at": "2025-01-02T01:02:05Z",
  "last_error": null
}
```

ratedContests 只统计计分比赛，latestContest 取结束时间最近的一场计分比赛。没有计分比赛时，ratedContests 为 0，rating、maxRating、latestContest 均为 null。

## 解绑账号

```sh
curl -i -X DELETE "$base/users/$id/accounts/atcoder"
```

请求体为空。成功返回 `204 No Content`，响应体为空。再次查询该绑定会返回 `404` 和 `{"error":"platform_not_bound"}`。
