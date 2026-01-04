# Converter Plugin

Converter handles unit, currency, crypto, and number-base conversions, plus simple expressions.

## What it supports

- **Units**: length, weight, temperature, time
- **Number base**: bin, oct, dec, hex
- **Currencies**: USD, EUR, GBP, JPY, CNY, AUD, CAD
- **Crypto**: BTC, ETH, USDT, BNB
- **Math**: `+ - * /` with mixed units

## Quick start

```
1km to m
100lb to kg
10 to bin
1 btc to usd
100 usd + 50 usd
```

## Tips

- Use `to` or `in` to set the target unit.
- Base conversion only supports integers and needs an explicit target (e.g., `255 to hex`).
- If you enter only a currency/crypto value, Wox converts it to your default locale currency.

## Rates

- Currency rates update about hourly; crypto prices update about every minute.
- Starts with approximate values and refreshes in the background.
- When offline, results use the last cached values.
