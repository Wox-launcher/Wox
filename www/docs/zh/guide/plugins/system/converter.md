# 转换器插件

转换器处理单位、货币、加密货币价格、进制、日期、时区，以及带类型值的简单计算。

## 快速开始

```text
1km to m
100lb to kg
32 bytes to gb
32bytes =? gb
1 GB to MiB
1 btc to usd
100 usd + 50 usd
```

转换器监听全局输入。如果其他全局结果优先级更高，可以使用 `calculator` 显式触发。

![转换器插件结果列表](/images/system-plugin-converter.png)

## 支持内容

| 类型 | 示例 |
| --- | --- |
| 单位 | 长度、重量、温度、时间 |
| 进制 | 二进制、八进制、十进制、十六进制 |
| 货币 | 常见法币 |
| 加密货币 | BTC、ETH、USDT、BNB 等常见符号 |
| 时间 | 时间戳、日期、时长、时区 |
| 计算 | 对兼容值使用 `+`、`-`、`*`、`/` |

## 表达式与自然语言

支持嵌套括号、运算优先级、正负号、幂、百分比和兼容的复合单位。数字输入与 Calculator 共用小数点和分组分隔符设置。

```text
(100 USD + 50 EUR) * 2 to CNY
(1h + 30min) / 2 to minutes
12% of (100 USD + 50 USD)
19m + 47%
20% off 80
15% tip on 42
ratio of 3 to 5
square root of 625
2 power 10
USD1K
8 dollars/hour in gbp
3 teaspoon in ml
2 inches in px at 72 ppi
145 mins to timespan
workhours in 2023
55h in workdays
```

`tip on` 返回小费金额。工作日按周一至周五、每天 8 小时计算，不扣除节假日。`m` 默认表示分钟，有明确长度上下文时表示米；可以写 `meters` 或 `minutes` 消除歧义。物理单位转换必须完整匹配维度，例如使用 `km/h in m/s`，不接受 `km/h in m`。

## 日期与时区

```text
2 am ist to cet
2026-09-08 2 am ist to cet
5pm ldn in sf
time in São Paulo
time in JFK
now in sf
time diff Paris
diff Paris
time in 4 hours in San Francisco
2024-03-15T14:30:00Z
August 5 + 5
3:45pm + 5
monday in 3 weeks
35 days ago
```

省略日期时，使用源时区的今天。IST 固定表示印度（`Asia/Kolkata`）；CET 使用 `Europe/Paris`，包含夏令时偏移。夏令时切换期间不存在或重复的当地时刻不会返回单一转换结果。副标题显示源和目标的日期、时区及 UTC 偏移。

操作菜单支持复制格式化结果、未格式化结果、问题和答案。未格式化时间结果保留时区偏移。

## 存储转换语言

存储查询使用 `B` 作为 Byte base unit，`GB` 等 Decimal storage unit 表示 base-1000 存储，`GiB`、`MiB` 等 Binary storage unit 表示 base-1024 存储。Unit symbol form（`GB`、`MiB`）和 Unit full-word form（`gigabyte`、`mebibyte`）都可作为输入；结果使用 Unit symbol form。

| 验收场景 | 查询 | 预期结果 |
| --- | --- | --- |
| 从 Byte base unit 转为 Decimal storage unit | `32 bytes to gb` | `0.000000032 GB` |
| 从 Byte base unit 转为 Binary storage unit | `32 bytes to gib` | `0.0000000298023224 GiB` |
| Equals-question conversion syntax | `32 bytes =? gb` | `0.000000032 GB` |
| Compact byte input | `32bytes =? gb` | `0.000000032 GB` |
| Unit symbol form | `1 GB to MiB` | `953.67431640625 MiB` |
| Unit full-word form | `1 gigabyte to gibibyte` | `0.9313225746154785 GiB` |
| Byte aliases 使用 symbolized output | `32 b to bytes` | `32 B` |

## 技巧

- 使用 `to`、`in` 或 `=?` 明确转换意图。
- 存储转换中，`gb` 表示 Decimal storage unit `GB`；如需 Binary storage unit `GiB`，请使用 `gib`。
- 进制转换需要整数和目标进制。
- 货币汇率会在后台刷新。首次执行加密货币查询时，转换器会先请求明确授权；确认后才会访问 CoinGecko，并在 Wox 运行期间每分钟刷新价格。授权只保存在当前设备。
- 实时数据源不可用时，货币和加密货币转换可能使用兜底值。
- 未指定目标货币时，根据系统地区选择默认货币；也可以在查询末尾显式指定目标。
