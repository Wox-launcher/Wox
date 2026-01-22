using System;
using System.Collections.Generic;
using System.Linq;
using Wox.UI.Windows.Models;

namespace Wox.UI.Windows.Services;

public sealed class FlickerStatus
{
    public bool Flicker { get; }
    public string Reason { get; }
    public int Events { get; }

    public FlickerStatus(bool flicker, string reason, int events)
    {
        Flicker = flicker;
        Reason = reason;
        Events = events;
    }
}

public sealed class AdjustDelayResult
{
    public int NewDelay { get; }
    public FlickerStatus Status { get; }

    public AdjustDelayResult(int newDelay, FlickerStatus status)
    {
        NewDelay = newDelay;
        Status = status;
    }
}

public sealed class WindowFlickerDetector
{
    private int _lastAppliedHeight;
    private readonly List<ResizeRecord> _resizeRecords = new();

    private int _stableNonFlickerCount;

    public int FlickerWindowMs { get; init; } = 300;
    public int FlickerMinEvents { get; init; } = 2;
    public int FlickerMinDirectionChanges { get; init; } = 1;
    public int StableDecreaseRequired { get; init; } = 10;
    public int MinDelay { get; init; } = 100;
    public int MaxDelay { get; init; } = 300;
    public int Step { get; init; } = 25;

    public void RecordResize(int height)
    {
        var now = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        if (_lastAppliedHeight == 0)
        {
            _lastAppliedHeight = height;
        }

        var delta = height - _lastAppliedHeight;
        _resizeRecords.Add(new ResizeRecord(now, height, delta));
        _lastAppliedHeight = height;
        Compact(now);
    }

    public FlickerStatus IsWindowFlickering()
    {
        var now = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        var windowStart = now - FlickerWindowMs;
        var recent = _resizeRecords.Where(r => r.Ts >= windowStart).ToList();

        if (recent.Count < FlickerMinEvents)
        {
            return new FlickerStatus(false, "not_enough_events", recent.Count);
        }

        int directionReversals = 0;
        int? lastNonZeroSign = null;

        int minH = recent[0].Height;
        int maxH = recent[0].Height;

        foreach (var record in recent)
        {
            var sign = record.Delta == 0 ? 0 : record.Delta > 0 ? 1 : -1;
            if (sign != 0)
            {
                if (lastNonZeroSign.HasValue && sign != lastNonZeroSign.Value)
                {
                    directionReversals++;
                }
                lastNonZeroSign = sign;
            }

            if (record.Height < minH) minH = record.Height;
            if (record.Height > maxH) maxH = record.Height;
        }

        var swingPx = maxH - minH;
        var magnitudeThreshold = (int)(UIConstants.RESULT_ITEM_BASE_HEIGHT * 2);

        if (directionReversals >= FlickerMinDirectionChanges && swingPx >= magnitudeThreshold)
        {
            return new FlickerStatus(true, "direction_change", recent.Count);
        }

        return new FlickerStatus(false, "below_threshold", recent.Count);
    }

    public AdjustDelayResult AdjustClearDelay(int currentDelay)
    {
        var status = IsWindowFlickering();
        var next = currentDelay;

        if (status.Flicker)
        {
            _stableNonFlickerCount = 0;
            next = currentDelay + Step;
        }
        else
        {
            if (status.Reason is "below_threshold" or "not_enough_events")
            {
                _stableNonFlickerCount++;
                if (_stableNonFlickerCount >= StableDecreaseRequired)
                {
                    next = currentDelay - Step;
                    _stableNonFlickerCount = 0;
                }
                else
                {
                    next = currentDelay;
                }
            }
            else
            {
                next = currentDelay;
            }
        }

        if (next < MinDelay) next = MinDelay;
        if (next > MaxDelay) next = MaxDelay;
        return new AdjustDelayResult(next, status);
    }

    private void Compact(long nowMs)
    {
        var cutoff = nowMs - (FlickerWindowMs * 3);
        while (_resizeRecords.Count > 0 && _resizeRecords[0].Ts < cutoff)
        {
            _resizeRecords.RemoveAt(0);
        }
    }

    private sealed class ResizeRecord
    {
        public long Ts { get; }
        public int Height { get; }
        public int Delta { get; }

        public ResizeRecord(long ts, int height, int delta)
        {
            Ts = ts;
            Height = height;
            Delta = delta;
        }
    }
}
