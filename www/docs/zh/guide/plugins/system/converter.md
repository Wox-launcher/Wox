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
- 货币和加密货币汇率会在后台刷新，离线时可能使用缓存值。
- 如果 fallback 转换的目标货币不符合预期，在插件设置中修改默认货币。
