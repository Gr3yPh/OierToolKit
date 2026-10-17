# otk-remote 协议规范 v1

otk-remote 是 OierToolKit 的远程 OJ 互操作协议：**otk 只做客户端**（拉题、
提交、等待结果），OJ 侧实现下面三个 HTTP 端点即可接入。纯 JSON over HTTP，
无任何外部依赖要求，PHP / Node / Go 等任意技术栈都能实现。

## 通用约定

- 传输：HTTP 或 HTTPS，请求与响应体均为 `application/json; charset=utf-8`
- 路径前缀：`/api/v1`（不兼容变更时升为 `/api/v2`）
- 认证：可选。OJ 可要求 `Authorization: Bearer <token>`；客户端用
  `OTK_TOKEN` 环境变量或 `otk config remote_token <token>` 提供
- 错误：非 2xx 状态码，响应体建议 `{"error": "人类可读原因"}`
  （客户端报错时优先展示该字段）
- 题号、提交号按不透明字符串处理，客户端会对它们做 URL 编码

## 1. 获取题目

```
GET {base}/api/v1/problems/{id}
```

响应示例：

```json
{
  "id": "P1007",
  "title": "有偿参观",
  "time_limit_ms": 1000,
  "memory_limit_mb": 128,
  "samples": [
    { "input": "3 5\n", "output": "8\n" },
    { "input": "10 -2\n", "output": "8\n" }
  ]
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | 题号；客户端以此作为项目名 |
| title | string | 题目标题，可为空 |
| time_limit_ms | int | 时限（毫秒）；0 或缺省表示不下发，客户端保留默认 |
| memory_limit_mb | number | 内存限制（MB）；0 或缺省同上 |
| samples | array | 样例对；可为空数组 |

客户端行为：以题号在当前目录建项目（当前目录已是项目则就地更新），时限与
内存写入项目配置，样例落成 `1.in/1.out`、`2.in/2.out`… 并入样例库，同时
把 `remote_base` 与 `remote_id` 记入项目配置供提交使用。

## 2. 提交

```
POST {base}/api/v1/submissions
Content-Type: application/json
```

请求体：

```json
{ "problem": "P1007", "language": "cpp", "source": "#include <cstdio>\n..." }
```

成功响应（200 或 201）：

```json
{ "id": "42" }
```

`id` 为提交号，字符串或数字皆可（客户端统一按字符串处理）。

## 3. 查询提交

```
GET {base}/api/v1/submissions/{id}
```

响应示例（带测试点明细）：

```json
{
  "id": "42",
  "verdict": "WA",
  "score": 60,
  "max_score": 100,
  "time_ms": 112,
  "memory_kb": 1024,
  "detail": "",
  "test_points": [
    { "index": 1, "verdict": "AC", "score": 30, "time_ms": 12, "memory_kb": 980, "detail": "" },
    { "index": 2, "verdict": "AC", "score": 30, "time_ms": 11, "memory_kb": 960, "detail": "" },
    { "index": 3, "verdict": "WA", "score": 0,  "time_ms": 8,  "memory_kb": 900,
      "detail": "第 3 组数据:\n  期望 8\n  实际 7" }
  ]
}
```

### 汇总字段

| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | 提交号 |
| verdict | string | 总体 verdict（取值见下）；Pending / Judging 时其余字段可省略 |
| score | int | 本题得分；非计分 OJ 省略或填 0 |
| max_score | int | 本题满分；大于 0 时客户端按计分模式展示（如 `WA 60/100`） |
| time_ms | int | 建议填所有测试点耗时的最大值 |
| memory_kb | int | 建议填峰值内存 |
| detail | string | 总体补充信息；CE 时放编译器输出 |
| test_points | array | 测试点明细，按 index 升序；未出完时可只含已完成的点 |

### 测试点字段

| 字段 | 类型 | 说明 |
|---|---|---|
| index | int | 测试点序号，从 1 起 |
| verdict | string | 该点 verdict；该点未出结果时也可为 Pending / Judging |
| score | int | 该点得分 |
| time_ms | int | 该点运行时间 |
| memory_kb | int | 该点峰值内存 |
| detail | string | 该点补充信息（WA 对比、RE 信息等），客户端会缩进展示 |

### 汇总规则

- 总体 verdict 由服务端汇总：任一测试点非 AC 则总体非 AC；测试点未出齐时
  总体为 Pending / Judging，客户端继续轮询
- 各 verdict 数量（如 `AC 8/10`）由客户端从 `test_points` 推导，协议不重复
  提供
- `test_points` 整体省略时（如 CE 发生在编译阶段、或 OJ 不提供明细），
  行为与旧版协议一致：只展示总体 verdict、分数与 detail

verdict 取值（大小写不敏感，客户端按大写比较）：

| verdict | 含义 |
|---|---|
| Pending / Judging | 未出结果，客户端继续轮询；**其余均视为终态** |
| AC | 通过 |
| WA | 答案错误 |
| TLE | 超时 |
| MLE | 超内存 |
| RE | 运行错误 |
| CE | 编译错误（detail 放编译器输出） |
| SE | 服务端内部错误 |

客户端默认每 2 秒轮询到总体终态，随后展示逐测试点明细与总分；
`otk submit -q` 只提交不轮询。

## 服务端注意事项

- 执行用户代码的沙箱、权限、限流由 OJ 侧全权负责，协议不作规定
- 建议对 `source` 大小设上限（如 256KB），并对提交频率限流
- 题目不存在返回 404 + `{"error": "..."}`；认证失败返回 401
