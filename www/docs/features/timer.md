---
title: "Countdown timers in the launcher"
description: "Start countdown timers from Wox, pin them on the desktop, and keep them across restarts."
---

# Countdown timers in the launcher

<ReleaseStamp />

Wox includes countdown timers you can start from the same window you use to open apps.

![Starting a 1 hour 5 minute timer from Wox](/images/timer.png)

## What it is

```text
timer 5m
timer 1h meeting
timer 1h5m there is a meeting
```

`timer` with a duration starts a countdown. An optional note after the duration becomes the timer label. Running timers persist across Wox restarts.

| Action | Use |
| --- | --- |
| Start and show on desktop | Start the timer and pin an overlay |
| Start in background | Start without an overlay |
| Pause / resume | Hold the remaining time |
| Pin | Show or hide the desktop overlay |

Overlays are not persisted. After a restart, reopen `timer` and pin the timer again if you want the overlay back.

## How to use it

1. Type `timer 25m focus`.
2. Press `Enter` to start and show it on the desktop, or use the Action Panel to start it in the background.
3. Type `timer` later to pause, resume, or delete it.
