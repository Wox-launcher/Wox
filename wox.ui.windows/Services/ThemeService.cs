using System.Text.Json;
using System.Windows;
using System.Windows.Media;

namespace Wox.UI.Windows.Services;

public class ThemeService
{
    private static readonly Lazy<ThemeService> _instance = new(() => new ThemeService());
    public static ThemeService Instance => _instance.Value;

    private ThemeService() { }

    /// <summary>
    /// 应用主题（从 wox.core 的主题 JSON）
    /// </summary>
    public void ApplyTheme(string themeJson)
    {
        try
        {
            using var doc = JsonDocument.Parse(themeJson);
            var root = doc.RootElement;

            // 获取主题颜色配置
            if (root.TryGetProperty("QueryBoxBackgroundColor", out var bgColor))
            {
                UpdateResource("QueryBoxBackground", ParseColor(bgColor.GetString()));
            }

            if (root.TryGetProperty("ResultItemActiveBackgroundColor", out var activeBg))
            {
                UpdateResource("ResultItemActiveBackground", ParseColor(activeBg.GetString()));
            }

            // TODO: 根据实际的 Wox 主题 JSON 结构继续添加映射
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Error applying theme: {ex.Message}");
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

    private Color ParseColor(string? colorString)
    {
        if (string.IsNullOrEmpty(colorString))
            return Colors.Transparent;

        try
        {
            // 支持 #RRGGBB 和 #AARRGGBB 格式
            if (colorString.StartsWith("#"))
            {
                colorString = colorString.TrimStart('#')!;

                if (colorString.Length == 6)
                {
                    // #RRGGBB
                    var r = Convert.ToByte(colorString.Substring(0, 2), 16);
                    var g = Convert.ToByte(colorString.Substring(2, 2), 16);
                    var b = Convert.ToByte(colorString.Substring(4, 2), 16);
                    return Color.FromRgb(r, g, b);
                }
                else if (colorString.Length == 8)
                {
                    // #AARRGGBB
                    var a = Convert.ToByte(colorString.Substring(0, 2), 16);
                    var r = Convert.ToByte(colorString.Substring(2, 2), 16);
                    var g = Convert.ToByte(colorString.Substring(4, 2), 16);
                    var b = Convert.ToByte(colorString.Substring(6, 2), 16);
                    return Color.FromArgb(a, r, g, b);
                }
            }

            return Colors.Transparent;
        }
        catch
        {
            return Colors.Transparent;
        }
    }
}
