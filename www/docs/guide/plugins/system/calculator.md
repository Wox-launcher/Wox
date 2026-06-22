# Calculator Plugin

Calculator evaluates expressions directly in the launcher. It listens globally, so you can type a calculation without a keyword.

## Quick Start

```text
100 + 200
2 ^ 10
(500 - 100) / (10 + 10)
```

Press `Enter` to copy or use the result, depending on the current action. Open the Action Panel to copy either the raw value or the formatted value.

![Calculator plugin result list](/images/system-plugin-calculator.png)

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
