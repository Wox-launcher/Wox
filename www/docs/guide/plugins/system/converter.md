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
| Base64 | decode a pasted token, or `hello to base64` |
| Currency | common fiat currencies |
| Crypto | common crypto symbols such as BTC, ETH, USDT, and BNB |
| Time | timestamps, dates, durations, and time zones |
| Math | `+`, `-`, `*`, `/` with compatible values |

## Expressions and Natural Language

Converter supports nested parentheses, operator precedence, unary signs, powers,
percentages, and compatible compound units. Number input uses the same decimal and
grouping separator settings as Calculator.

```text
(100 USD + 50 EUR) * 2 to CNY
(1h + 30min) / 2 to minutes
12% of (100 USD + 50 USD)
19m + 47%
20% off 80
15% tip on 42
ratio of 3 to 5
square root of 625
cube root of 27
2 power 10
USD1K
8 dollars/hour in gbp
3 teaspoon in ml
2 inches in px at 72 ppi
145 mins to timespan
workhours in 2023
55h in workdays
1/3 to 2 dp
π to 5 digits
21 rounded up to nearest 5
17 rounded down to nearest 3
meters in 10 km
days in 3 weeks
seconds in a day
300 + 20 km
$20 + 30
cmFpbDE2Mw==
aGVsbG8= to text
hello to base64
```

`1/3 to 2 dp` and `π to 5 digits` round the answer to that many decimal places.
`21 rounded up to nearest 5` and `17 rounded down to nearest 3` snap to a multiple of the step.
`meters in 10 km` and `seconds in a day` ask how many of a unit are in a quantity.
A bare number added to a quantity inherits that unit: `300 + 20 km`, `$20 + 30`.
`tip on` returns the tip amount. Workdays mean Monday–Friday, eight hours per day,
without holidays. `m` means minutes unless an explicit length context selects
meters. Use `meters` or `minutes` to remove ambiguity. Partial physical conversion
is not implied: use `km/h in m/s`, rather than `km/h in m`.

## Dates and Time Zones

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
3pm GMT+8 to Paris
7:30am LAX to Japan
Tokyo time
time difference between Seattle and Moscow
difference between PDT & AEST
5.5 minutes as timespan
4.54 hours as timespan
72 days as timespan
5.5 minutes as laptime
03:04:05 + 01:02:03
00:12:05 − 00:04:09
03:04:05 as timespan
3 hours 4 minutes 5 seconds as laptime
12.5 minutes in minutes and seconds
1.4 weeks in hours and minutes
4.5 weeks in days and hours
1 hour 30 minutes at 1.5x
time saved 5 min at 1.5x
```

`GMT+8` and `UTC-7` are fixed offsets. `Tokyo time` means the current time in that place.
`time difference between Seattle and Moscow` is the offset gap between two zones.
When the date is omitted, the source timezone supplies today's date. IST means
India (`Asia/Kolkata`); CET uses `Europe/Paris`, including summer time. Missing or
repeated local times at daylight-saving transitions are rejected. Results show
source/target dates, resolved timezones, and UTC offsets.

Actions copy the formatted answer, raw answer, or question and answer. Raw time
answers retain their timezone offsets.

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
- A standalone Base64 token that decodes to readable text is converted automatically. Use `to text` / `base64 decode` when the token is short, and `to base64` to encode.
- Currency rates refresh in the background. The first crypto query asks for explicit permission before Converter contacts CoinGecko; after confirmation, crypto prices refresh every minute while Wox is running and the choice is remembered only on that device.
- Currency and crypto conversions may use fallback values while live sources are unavailable.
- Without an explicit currency target, Converter chooses the default currency from your locale.
