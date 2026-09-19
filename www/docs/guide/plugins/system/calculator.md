# Calculator Plugin

Calculator evaluates expressions directly in the launcher. It listens globally, so you can type a calculation without a keyword.

![The Calculator plugin](/images/plugin_calculator.png)

## Quick Start

```text
100 + 200
2 ^ 10
(500 - 100) / (10 + 10)
```

Press `Enter` to copy or use the result, depending on the current action. Open the Action Panel to copy either the raw value or the formatted value.

To continue calculating from the current result, select it and press `Shift+Tab`. Wox replaces the query with that result so you can type another operator:

```text
100 + 200
Shift+Tab
300 * 2
```

## Explicit Mode

Use `calculator` when you want calculator history or when another global plugin is competing with the same query:

```text
calculator 12 * 12
calculator
```

## Notes

- Include an operator to trigger global calculation, such as `+`, `-`, `*`, `/`, or `^`.
- Parentheses are supported.
- Thousands separator behavior can be changed in plugin settings.
