# Converter Plugin

Converter handles units, currencies, crypto prices, number bases, dates, time zones, and simple math with typed values.

## Quick Start

```text
1km to m
100lb to kg
32 bytes to gb
32bytes =? gb
1 GB to MiB
1 btc to usd
100 usd + 50 usd
```

Converter listens globally. Use `calculator` as an explicit keyword if another global result is taking priority.

![Converter plugin result list](/images/system-plugin-converter.png)

## Supported Work

| Type | Examples |
| --- | --- |
| Units | length, weight, temperature, time |
| Number base | binary, octal, decimal, hexadecimal |
| Currency | common fiat currencies |
| Crypto | common crypto symbols such as BTC, ETH, USDT, and BNB |
| Time | timestamps, dates, durations, and time zones |
| Math | `+`, `-`, `*`, `/` with compatible values |

## Storage Conversion Language

Storage queries use `B` as the Byte base unit, decimal units such as `GB` for base-1000 storage, and binary units such as `GiB` or `MiB` for base-1024 storage. Unit symbol form (`GB`, `MiB`) and Unit full-word form (`gigabyte`, `mebibyte`) are both accepted; results use Unit symbol form.

| Acceptance scenario | Query | Expected result |
| --- | --- | --- |
| Decimal storage unit from Byte base unit | `32 bytes to gb` | `0.000000032 GB` |
| Binary storage unit from Byte base unit | `32 bytes to gib` | `0.0000000298023224 GiB` |
| Equals-question conversion syntax | `32 bytes =? gb` | `0.000000032 GB` |
| Compact byte input | `32bytes =? gb` | `0.000000032 GB` |
| Unit symbol form | `1 GB to MiB` | `953.67431640625 MiB` |
| Unit full-word form | `1 gigabyte to gibibyte` | `0.9313225746154785 GiB` |
| Byte aliases render as symbol output | `32 b to bytes` | `32 B` |

## Tips

- Use `to`, `in`, or `=?` to make conversion intent explicit.
- For storage conversion, `gb` means Decimal storage unit `GB`; use `gib` for Binary storage unit `GiB`.
- Base conversion expects an integer and a target base.
- Currency and crypto rates refresh in the background and may use cached values while offline.
- Set your default currency in plugin settings if fallback conversions are not what you expect.
