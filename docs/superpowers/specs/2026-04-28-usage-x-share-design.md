# Wox Usage X Share Design

## Goal

Add an X share button to the Usage settings page so users can share their real Wox usage summary as an image. The generated image must use the user's current local usage statistics only; it must not contain sample, random, or fabricated data.

The first shipping version will generate a 4:5 share card, copy the PNG to the system clipboard, and open X's compose page with a short prefilled text. The user will paste the image into X and publish manually.

## Current Context

The Usage page is implemented in `wox.ui.flutter/wox/lib/modules/setting/views/wox_setting_usage_view.dart`.

Usage data is already loaded through `WoxSettingController.refreshUsageStats()` and stored in `WoxSettingController.usageStats`. The model is `WoxUsageStats`, which includes:

- `totalOpened`
- `totalAppLaunch`
- `totalActions`
- `totalAppsUsed`
- `mostActiveHour`
- `mostActiveDay`
- `topApps`
- `topPlugins`

The UI already depends on `url_launcher` and `path_provider`, so no new package is needed for opening X or writing a temporary PNG.

## Visual Direction

The approved visual direction is a clean vertical 4:5 data card, exported as 1080x1350 PNG.

The card contains:

- Top brand row: Wox icon mark, `Wox Launcher`, and the current year.
- Primary metric card: `Wox opens` from `totalOpened`, with `Peak` hour from `mostActiveHour`.
- Two secondary metric cards: `actions` from `totalActions` and `apps` from `totalAppsUsed`.
- Highlights card: up to three rows based on real usage data.
- Bottom spacing: tight, clean breathing room with only subtle background glow, no trend line or decorative icon strip.

The visual style should be dark, neon-teal, high contrast, and readable in X's feed preview. Text must stay minimal and data-led. Long names must ellipsize instead of changing layout height.

## Data Mapping

All visible numbers and labels must come from `WoxUsageStats`.

Primary metrics:

- `Wox opens`: `stats.totalOpened`
- `Peak`: `stats.mostActiveHour`, formatted as `HH:00`; hidden or shown as `-` when the value is below zero
- `actions`: `stats.totalActions`
- `apps`: `stats.totalAppsUsed`

Highlights:

1. First row: top plugin from `stats.topPlugins.first`, labeled `Top plugin`.
2. Second row: first top plugin whose name or id indicates AI when available, labeled `AI`; otherwise fall back to the second top plugin.
3. Third row: top app from `stats.topApps.first`, labeled `Top app`.

If a highlight source is missing, that row is omitted. The card must never inject example values such as `System Command`, `QQ Music`, or fixed counts unless those are the user's actual stats.

## User Flow

The Usage page header gains a share button next to Refresh.

When clicked:

1. If usage stats are currently loading, the button is disabled.
2. The current `WoxUsageStats` is rendered into an offscreen share card.
3. The card is captured as PNG.
4. The PNG is written to a temporary file for diagnostics and copied to the system image clipboard.
5. X compose opens through `https://x.com/intent/tweet` with a short text such as `My Wox Launcher usage recap`.
6. The user pastes the copied image into X and publishes manually.

The app must not attempt to post to X automatically, upload the image, or submit the tweet.

## Architecture

Keep the feature in the Flutter UI layer.

- Add a small share action in `WoxSettingUsageView`.
- Keep short-lived share state local to the Usage view, because it only controls a single button action.
- Extract the share card widget into a private widget or focused file near the Usage view if the main file becomes too large.
- Use `RepaintBoundary` for PNG capture.
- Use the existing screenshot platform clipboard bridge if it can copy an image file on the current platform. If unsupported, still write the PNG and open X, then show a clear message that the image must be attached manually.
- Use `url_launcher` for opening X.

## Error Handling

The share action should fail softly:

- If PNG encoding fails, log the error and show a short user-facing error.
- If image clipboard copy is unsupported, keep the generated file and open X with a message telling the user to attach the image manually.
- If opening X fails, keep the generated image copied/saved and show a short error.
- If all stats are empty, still allow sharing but render zero values and omit missing highlight rows.

## Localization

Add localized labels for the button and status/error text in the existing language JSON files:

- Share button
- Copied/opened success message
- Image copy unsupported fallback
- Share failed

The generated share image itself can use concise English labels (`Wox opens`, `actions`, `apps`, `Highlights`) because the goal is public sharing to X and the existing visual direction was approved with English copy.

## Verification

Run:

- `dart format --line-length 180` on touched Dart files
- `flutter analyze` for the touched Flutter project

Because this is a user-facing settings feature, add or update a smoke test only if the existing integration smoke harness can exercise the Usage page without requiring native clipboard assertions. The core requirement is manual verification that a real stats card is generated, copied or saved, and X compose opens without automatic posting.
