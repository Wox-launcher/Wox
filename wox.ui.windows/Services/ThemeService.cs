using System;
using System.Globalization;
using System.Text.Json;
using System.Windows;
using System.Windows.Media;

namespace Wox.UI.Windows.Services;

public class ThemeService
{
    private static readonly Lazy<ThemeService> _instance = new(() => new ThemeService());
    public static ThemeService Instance => _instance.Value;

    private ThemeService() { }

    public event EventHandler<bool>? ThemeModeChanged;

    public bool IsDarkTheme { get; private set; } = true;

    /// <summary>
    /// 应用主题（从 wox.core 的主题 JSON）
    /// </summary>
    public void ApplyTheme(string themeJson)
    {
        try
        {
            using var doc = JsonDocument.Parse(themeJson);
            var root = doc.RootElement;

            var appBackground = GetColor(root, "AppBackgroundColor");
            if (appBackground != null)
            {
                var windowBackground = appBackground.Value;
                // Flutter behavior: direct use of color, no artificial alpha scaling
                UpdateResource("ApplicationBackgroundBrush", windowBackground);
                UpdateResource("AppBackgroundBrush", windowBackground);
                UpdateResource("PreviewBackgroundBrush", windowBackground);
                UpdateResource("ResultItemIconBackgroundBrush", ScaleAlpha(windowBackground, 0.6));
            }

            var queryBackground = GetColor(root, "QueryBoxBackgroundColor");
            if (queryBackground != null)
            {
                UpdateResource("QueryBoxBackgroundBrush", queryBackground.Value);
                UpdateResource("CardBackgroundFillColorDefaultBrush", queryBackground.Value);
            }

            var queryFont = GetColor(root, "QueryBoxFontColor");
            if (queryFont != null)
            {
                UpdateResource("QueryBoxFontBrush", queryFont.Value);
                UpdateResource("TextFillColorPrimaryBrush", queryFont.Value);

                var placeholder = ScaleAlpha(queryFont.Value, 0.6);
                UpdateResource("QueryBoxPlaceholderBrush", placeholder);
            }

            var queryCursor = GetColor(root, "QueryBoxCursorColor");
            if (queryCursor != null)
            {
                UpdateResource("QueryBoxCursorBrush", queryCursor.Value);
            }

            var selectionForeground = GetColor(root, "QueryBoxTextSelectionColor");
            if (selectionForeground != null)
            {
                UpdateResource("QueryBoxSelectionForegroundBrush", selectionForeground.Value);
            }

            var selectionBackground = GetColor(root, "QueryBoxTextSelectionBackgroundColor") ?? selectionForeground;
            if (selectionBackground != null)
            {
                UpdateResource("QueryBoxSelectionBackgroundBrush", selectionBackground.Value);
            }

            var resultTitle = GetColor(root, "ResultItemTitleColor");
            if (resultTitle != null)
            {
                UpdateResource("ResultItemTitleBrush", resultTitle.Value);
                UpdateResource("TextFillColorPrimaryBrush", resultTitle.Value);
            }

            var resultSubTitle = GetColor(root, "ResultItemSubTitleColor");
            if (resultSubTitle != null)
            {
                UpdateResource("ResultItemSubTitleBrush", resultSubTitle.Value);
                UpdateResource("TextFillColorSecondaryBrush", resultSubTitle.Value);
            }

            var resultActiveTitle = GetColor(root, "ResultItemActiveTitleColor");
            if (resultActiveTitle != null)
            {
                UpdateResource("ResultItemActiveTitleBrush", resultActiveTitle.Value);
            }

            var resultActiveSubTitle = GetColor(root, "ResultItemActiveSubTitleColor");
            if (resultActiveSubTitle != null)
            {
                UpdateResource("ResultItemActiveSubTitleBrush", resultActiveSubTitle.Value);
            }

            var resultActiveBackground = GetColor(root, "ResultItemActiveBackgroundColor");
            if (resultActiveBackground != null)
            {
                UpdateResource("ResultItemActiveBackgroundBrush", resultActiveBackground.Value);
                UpdateResource("AccentBrush", resultActiveBackground.Value);

                var hoverBackground = WithAlpha(resultActiveBackground.Value, 0.3);
                UpdateResource("ResultItemHoverBackgroundBrush", hoverBackground);
                UpdateResource("CardBackgroundFillColorSecondaryBrush", hoverBackground);
            }

            var toolbarFont = GetColor(root, "ToolbarFontColor");
            if (toolbarFont != null)
            {
                UpdateResource("ToolbarFontBrush", toolbarFont.Value);
            }

            var toolbarBackground = GetColor(root, "ToolbarBackgroundColor");
            if (toolbarBackground != null)
            {
                UpdateResource("ToolbarBackgroundBrush", toolbarBackground.Value);
            }
            else if (appBackground != null)
            {
                UpdateResource("ToolbarBackgroundBrush", appBackground.Value);
            }

            var previewFont = GetColor(root, "PreviewFontColor");
            if (previewFont != null)
            {
                UpdateResource("PreviewFontBrush", previewFont.Value);
            }

            var previewSplitLine = GetColor(root, "PreviewSplitLineColor");
            if (previewSplitLine != null)
            {
                UpdateResource("PreviewSplitLineBrush", previewSplitLine.Value);
                UpdateResource("ControlStrokeColorDefaultBrush", previewSplitLine.Value);
                UpdateResource("ControlStrongStrokeColorDefaultBrush", previewSplitLine.Value);
            }

            var queryCornerRadius = GetInt(root, "QueryBoxBorderRadius");
            if (queryCornerRadius != null)
            {
                UpdateCornerRadius("QueryBoxCornerRadius", queryCornerRadius.Value);
            }

            var resultCornerRadius = GetInt(root, "ResultItemBorderRadius");
            if (resultCornerRadius != null)
            {
                UpdateCornerRadius("ResultItemCornerRadius", resultCornerRadius.Value);
            }

            // Layout & Borders
            var appPadding = GetThickness(root, "AppPaddingLeft", "AppPaddingTop", "AppPaddingRight", "AppPaddingBottom");
            if (appPadding != null) UpdateThicknessResource("AppPadding", appPadding.Value);

            var resultContainerPadding = GetThickness(root, "ResultContainerPaddingLeft", "ResultContainerPaddingTop", "ResultContainerPaddingRight", "ResultContainerPaddingBottom");
            if (resultContainerPadding != null) UpdateThicknessResource("ResultContainerPadding", resultContainerPadding.Value);

            var resultItemPadding = GetThickness(root, "ResultItemPaddingLeft", "ResultItemPaddingTop", "ResultItemPaddingRight", "ResultItemPaddingBottom");
            if (resultItemPadding != null) UpdateThicknessResource("ResultItemPadding", resultItemPadding.Value);

            var toolbarPaddingLeft = GetInt(root, "ToolbarPaddingLeft") ?? 0;
            var toolbarPaddingRight = GetInt(root, "ToolbarPaddingRight") ?? 0;
            if (toolbarPaddingLeft > 0 || toolbarPaddingRight > 0)
            {
                UpdateThicknessResource("ToolbarPadding", new Thickness(toolbarPaddingLeft, 0, toolbarPaddingRight, 0));
            }

            var resultItemBorderLeft = GetInt(root, "ResultItemBorderLeftWidth");
            if (resultItemBorderLeft != null)
            {
                UpdateThicknessResource("ResultItemBorderThickness", new Thickness(resultItemBorderLeft.Value, 0, 0, 0));
            }

            var resultItemActiveBorderLeft = GetInt(root, "ResultItemActiveBorderLeftWidth");
            if (resultItemActiveBorderLeft != null)
            {
                UpdateThicknessResource("ResultItemActiveBorderThickness", new Thickness(resultItemActiveBorderLeft.Value, 0, 0, 0));
            }

            UpdateThemeMode(appBackground);
        }
        catch (Exception ex)
        {
            Logger.Error("Error applying theme", ex);
        }
    }

    private void UpdateResource(string key, Color color)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            var brush = new SolidColorBrush(color);
            brush.Freeze();
            Application.Current.Resources[key] = brush;
        });
    }

    private void UpdateCornerRadius(string key, int value)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            Application.Current.Resources[key] = new CornerRadius(value);
        });
    }

    private void UpdateThicknessResource(string key, Thickness value)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            Application.Current.Resources[key] = value;
        });
    }

    private static Color? GetColor(JsonElement root, string name)
    {
        if (!root.TryGetProperty(name, out var element))
        {
            return null;
        }

        if (element.ValueKind == JsonValueKind.String)
        {
            var raw = element.GetString();
            if (string.IsNullOrWhiteSpace(raw))
            {
                return null;
            }
            return ParseColor(raw);
        }

        return null;
    }

    private static int? GetInt(JsonElement root, string name)
    {
        if (!root.TryGetProperty(name, out var element))
        {
            return null;
        }

        if (element.ValueKind == JsonValueKind.Number && element.TryGetInt32(out var numberValue))
        {
            return numberValue;
        }

        if (element.ValueKind == JsonValueKind.String
            && int.TryParse(element.GetString(), NumberStyles.Integer, CultureInfo.InvariantCulture, out var stringValue))
        {
            return stringValue;
        }

        return null;
    }

    private static Thickness? GetThickness(JsonElement root, string leftKey, string topKey, string rightKey, string bottomKey)
    {
        var left = GetInt(root, leftKey);
        var top = GetInt(root, topKey);
        var right = GetInt(root, rightKey);
        var bottom = GetInt(root, bottomKey);

        if (left == null && top == null && right == null && bottom == null)
        {
            return null;
        }

        return new Thickness(left ?? 0, top ?? 0, right ?? 0, bottom ?? 0);
    }

    private void UpdateThemeMode(Color? appBackground)
    {
        if (appBackground == null)
        {
            return;
        }

        var isDark = IsDarkColor(appBackground.Value);
        if (IsDarkTheme == isDark)
        {
            return;
        }

        IsDarkTheme = isDark;
        ThemeModeChanged?.Invoke(this, isDark);
    }

    private static bool IsDarkColor(Color color)
    {
        var luminance = (0.299 * color.R + 0.587 * color.G + 0.114 * color.B) / 255.0;
        return luminance < 0.5;
    }

    private static Color ParseColor(string? colorString)
    {
        if (string.IsNullOrEmpty(colorString))
            return Colors.Transparent;

        try
        {
            var trimmed = colorString.Trim();

            if (string.Equals(trimmed, "transparent", StringComparison.OrdinalIgnoreCase))
            {
                return Colors.Transparent;
            }

            if (trimmed.StartsWith("#", StringComparison.Ordinal))
            {
                var hex = trimmed.TrimStart('#');

                if (hex.Length == 6)
                {
                    // #RRGGBB
                    var r = Convert.ToByte(hex.Substring(0, 2), 16);
                    var g = Convert.ToByte(hex.Substring(2, 2), 16);
                    var b = Convert.ToByte(hex.Substring(4, 2), 16);
                    return Color.FromRgb(r, g, b);
                }
                else if (hex.Length == 8)
                {
                    // #AARRGGBB
                    var a = Convert.ToByte(hex.Substring(0, 2), 16);
                    var r = Convert.ToByte(hex.Substring(2, 2), 16);
                    var g = Convert.ToByte(hex.Substring(4, 2), 16);
                    var b = Convert.ToByte(hex.Substring(6, 2), 16);
                    return Color.FromArgb(a, r, g, b);
                }
            }

            if (trimmed.StartsWith("rgba", StringComparison.OrdinalIgnoreCase)
                || trimmed.StartsWith("rgb", StringComparison.OrdinalIgnoreCase))
            {
                var open = trimmed.IndexOf('(');
                var close = trimmed.IndexOf(')');
                if (open > -1 && close > open)
                {
                    var content = trimmed.Substring(open + 1, close - open - 1);
                    var parts = content.Split(',');
                    if (parts.Length >= 3)
                    {
                        var r = ParseComponent(parts[0]);
                        var g = ParseComponent(parts[1]);
                        var b = ParseComponent(parts[2]);
                        var a = 1.0;
                        if (parts.Length >= 4 && double.TryParse(parts[3].Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out var alpha))
                        {
                            a = alpha > 1 ? alpha / 255.0 : alpha;
                        }

                        return Color.FromArgb(ToByte(a * 255.0), r, g, b);
                    }
                }
            }

            return Colors.Transparent;
        }
        catch
        {
            return Colors.Transparent;
        }
    }

    private static byte ParseComponent(string value)
    {
        if (!double.TryParse(value.Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out var result))
        {
            return 0;
        }

        return ToByte(result);
    }

    private static byte ToByte(double value)
    {
        if (value < 0)
        {
            return 0;
        }

        if (value > 255)
        {
            return 255;
        }

        return (byte)Math.Round(value, MidpointRounding.AwayFromZero);
    }

    private static Color WithAlpha(Color color, double alpha)
    {
        var clamped = Math.Max(0, Math.Min(1, alpha));
        return Color.FromArgb(ToByte(clamped * 255.0), color.R, color.G, color.B);
    }

    private static Color ScaleAlpha(Color color, double scale)
    {
        var clamped = Math.Max(0, Math.Min(1, scale));
        return Color.FromArgb(ToByte(color.A * clamped), color.R, color.G, color.B);
    }
}
