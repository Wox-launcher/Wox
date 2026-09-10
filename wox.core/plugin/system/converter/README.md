# Converter

Converter handles typed expressions and natural-language conversions. Calculator
retains its numeric parser and history; both use `wox/util/calc` for number
separators and mathematical functions. Converter does not return a second result
for ordinary numeric Calculator expressions.

## Flow and ownership

1. `engine.Catalog.Parse` reads syntax with explicit numeric separators. It does
   not read the clock, prices, settings, or network. A Pratt parser handles
   arithmetic; a single datetime entry handles `time in`, time differences, and
   timezone conversion. Date/clock literals also work inside expressions.
2. The parser resolves only known unit ambiguity (`m`). A conversion target or
   explicit length operand selects meters; otherwise it means minutes. Conflicting
   standalone duration/length context is rejected. `dec`/`oct` are bases in a base
   conversion and months in date syntax; `in` is resolved by its grammar position.
3. The plugin checks parsed crypto dependencies before copying prices. The static
   crypto vocabulary never requires prices. An incomplete query produces no
   consent request. The existing per-device CoinGecko consent remains unchanged.
4. `Evaluate` checks operand types while evaluating against one `Env`: reference
   time, local timezone, default currency, and independent price snapshots. Rates
   are USD per unit, including fiat. Each service publishes a whole refresh under
   its snapshot lock. Evaluation never fetches data.
5. `Format` produces formatted/raw values, the original expression, timezone
   details, and the timestamp of the price snapshot. The plugin owns translations,
   actions, MRU restoration, query history, and lifecycle callbacks.

The old module-priority tokenizer, flattened expression evaluator, and conversion
polling have been removed. `modules` now contains the existing price services;
`core` retains the storage glossary and its boundary tests.

## Extending the engine

- Add a unit and aliases in the catalog. Linear units use an exact scale; absolute
  temperatures also have an offset. Storage aliases reuse the existing glossary.
- Add a city/airport alias in the timezone data; use IANA locations for rules.
- Add a mathematical function to the shared library, or a converter-specific
  function in the evaluator. Do not reparse formatted expression strings.
- Add domain syntax in the relevant parser and a typed operation. Do not register
  globally competing regular expressions. Regexes only recognize bounded literal
  or domain sentence formats. Base64 decode matches a whole-query token or an
  explicit `to text` / `base64 decode` sentence in `engine/base64.go`.
- Add executable examples to `engine/engine_test.go`, including rejected inputs.

## Defined semantics

- Number/Quantity arithmetic uses rational numbers; rounding occurs at output.
  `1/3 to 2 dp` and `π to 5 digits` request that many decimal places. `dp` and
  `digits` are aliases. `21 rounded up to nearest 5` and `17 rounded down to
  nearest 3` snap to a multiple of the step. Intermediate arithmetic stays exact.
  Transcendental functions are approximate and reject non-finite results.
  `meters in 10 km` asks how many of the leading unit are in the following
  quantity; `a`/`an` means one (`seconds in a day`). A bare number added to a
  quantity inherits that unit (`300 + 20 km`, `$20 + 30`).
- `%` is preserved until evaluation. `19m + 47%` increases the quantity by 47%;
  `10% + 20%` is 30%. `of` multiplies by the percentage, `off` subtracts it.
  `15% tip on 42` returns the tip amount (6.3). Ratios display their two operands
  while their numerical value participates in arithmetic.
- Full conversions require equal dimensions. A single currency target may replace
  the money factor (exponent 1) of a rate: `8 dollars/hour in gbp`. Partial physical
  conversions such as `km/h in m` and `m² in ft` are rejected. Use `m/s` or `ft²`.
- `now` uses the captured reference instant. Date and clock literals together
  (such as `2026-06-14 16:00`) use the local timezone. Compound durations such as
  `4d17h` form one operand; days here mean 24 elapsed hours.
- Date differences count calendar days. Instant differences measure elapsed time.
  Clock differences are signed within the same abstract day, without an assumed
  midnight rollover. Clock addition displays a day offset when it crosses midnight.
- A bare number added to a date means days; to a clock it means hours. Instants
  require an explicit duration. Calendar months clamp to the last valid day of the
  target month (`January 31 2020 + 1 month` is 29 February 2020).
- Standalone years use the mean Gregorian year (365.2425 days); date arithmetic
  uses calendar years.
- `32f to c` converts absolute temperature. Subtracting absolute temperatures
  produces a temperature difference. Adding a temperature literal to an absolute
  temperature treats the right literal as a difference. Use `deltaC`/`deltaF`
  explicitly otherwise. Absolute temperatures cannot be multiplied or used with
  relative percentage adjustments.
- Workdays are Monday–Friday, eight hours per day. Date offsets in workdays also
  skip New Year's Day, Independence Day, Thanksgiving, Christmas, and Boxing Day.
  `workhours in 2023` is 2080 hours. No new persisted settings are introduced.
- Omitted timezone dates use the source zone's today. IST explicitly means
  Asia/Kolkata. CET/CEST preserve the Europe/Paris alias and use seasonal offsets.
  `GMT+8` / `UTC-7` are fixed offsets. `Tokyo time` is `time in Tokyo`.
  `time difference between Seattle and Moscow` is the absolute offset gap at the
  reference instant. Explicit local times that are missing or repeated at a DST
  transition are rejected. Embedded tzdata makes this independent of OS timezone
  installation.
- `timespan` is a formatting target, not a unit. `as` is a synonym of `to`.
  `as laptime` and HH:MM:SS literals (two colons) are durations. `in hours and
  minutes` splits a duration into those units. `at 1.5x` scales playback time;
  `time saved 5 min at 1.5x` is the time removed. Existing `1h → minutes`,
  `1 week → days`, and `10 days → weeks` shortcuts remain presentation choices.
- Parsing follows Calculator's effective separator settings. Existing physical,
  duration, and storage rows remain ungrouped; raw values preserve necessary units
  and timezone offsets. Three copy actions return formatted, raw, or question/answer.

- Soulver sentence forms that fit a single query are supported: word operators
  (`plus`, `multiplied by`), percentages (`10% on 200`, `20 is 10% of what`,
  `50 to 75 is what %`), lists (`average of`, `gcd of`), comparisons, `if then
  else`, proportions, compound interest, playback/laptime/timespan, video
  timecode (`03:10:20:05 at 30 fps`), cooking densities, holidays, bitwise
  operators, pace, file-transfer time, inflation phrases, income tax, time zones,
  and comments / ignored words on one line. Clock literals with am/pm subtract as
  an absolute same-day interval; 24-hour clocks stay signed. Mixed currencies keep
  the last unit. Formatted dimensionless and money values of 100,000 or more use
  SI compact symbols (`3.3M`, `$7B`); rates such as `182,621.25/year` stay
  expanded. Sheet variables, line references, live weather, Wolfram, historical
  FX, and custom units stay out of Converter.
- `$30 × 4 days` is an implicit daily rate. `m × m` is area in meters.
- A whole-query Base64 token that decodes to printable UTF-8 is shown as text.
  `to base64` / `as base64` encode; `to text` / `base64 decode` force decode.

These are Wox's explicit defaults where Raycast's documentation does not specify
an output policy (not a claim of undocumented behavioral equivalence). The
executable Soulver corpus lives in `engine/soulver_test.go`.

## Limits and verification

Input is limited to 4096 bytes, nesting to 64 levels, and constructed nodes to
1024. Integer powers have exponent magnitude at most 4096; rational numerators
and denominators have at most 16384 bits. Unit exponents are bounded to ±16.
Power growth is checked before allocating its result. Calendar outputs are
limited to years 1–9999; elapsed-time operations reject duration overflow.

The executable corpus covers the explicit examples from the
[Raycast manual](https://manual.raycast.com/calculator) and
[feature page](https://www.raycast.com/core-features/calculator), Wox regressions,
compositions, locale inputs, errors, and timezone transitions. Before switching,
the old integration query corpus was compared through both engines; clock parsing
and base-format differences were corrected. No production shadow mode remains.

```powershell
$env:WOX_TEST_ENABLE_NETWORK='false'
go test ./plugin/system/calculator ./util/calc ./plugin/system/converter/...
go test ./plugin/system/converter/engine -fuzz=FuzzParseEvaluate -fuzztime=10s
go test -tags sqlite_fts5 ./test -run 'TestConverter|TestCalculatorTime|TestTimeZoneConversions|TestCalculatorBasic|TestCalculatorSeparators'
```

Network fetch tests retain the existing live-service checks; setting
`WOX_TEST_ENABLE_NETWORK=false` skips those requests, not the deterministic price
snapshot or conversion tests.
