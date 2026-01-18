using System.IO;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using Wox.UI.Windows.Models;

namespace Wox.UI.Windows.Services;

public static class ImageService
{
    /// <summary>
    /// 将 WoxImage 转换为 WPF ImageSource
    /// </summary>
    public static ImageSource? ConvertToImageSource(WoxImage? woxImage)
    {
        if (woxImage == null || string.IsNullOrEmpty(woxImage.ImageData))
            return null;

        try
        {
            switch (woxImage.ImageType.ToLowerInvariant())
            {
                case "base64":
                    return ConvertBase64ToImageSource(woxImage.ImageData);

                case "file":
                case "absolute":
                    return ConvertFilePathToImageSource(woxImage.ImageData);

                case "url":
                case "http":
                    return ConvertUrlToImageSource(woxImage.ImageData);

                case "svg":
                    // TODO: 需要集成 SVG 渲染库（如 Svg.Skia）
                    return null;

                default:
                    return null;
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Error converting image: {ex.Message}");
            return null;
        }
    }

    private static ImageSource? ConvertBase64ToImageSource(string base64Data)
    {
        // 移除可能的 data:image/xxx;base64, 前缀
        var cleanBase64 = base64Data;
        if (base64Data.Contains(","))
        {
            cleanBase64 = base64Data.Split(',')[1];
        }

        var imageBytes = Convert.FromBase64String(cleanBase64);
        using var stream = new MemoryStream(imageBytes);
        
        var bitmap = new BitmapImage();
        bitmap.BeginInit();
        bitmap.CacheOption = BitmapCacheOption.OnLoad;
        bitmap.StreamSource = stream;
        bitmap.EndInit();
        bitmap.Freeze(); // 使图像可跨线程访问
        
        return bitmap;
    }

    private static ImageSource? ConvertFilePathToImageSource(string filePath)
    {
        if (!File.Exists(filePath))
            return null;

        var bitmap = new BitmapImage();
        bitmap.BeginInit();
        bitmap.CacheOption = BitmapCacheOption.OnLoad;
        bitmap.UriSource = new Uri(filePath, UriKind.Absolute);
        bitmap.EndInit();
        bitmap.Freeze();
        
        return bitmap;
    }

    private static ImageSource? ConvertUrlToImageSource(string url)
    {
        try
        {
            var bitmap = new BitmapImage();
            bitmap.BeginInit();
            bitmap.CacheOption = BitmapCacheOption.OnLoad;
            bitmap.UriSource = new Uri(url, UriKind.Absolute);
            bitmap.EndInit();
            bitmap.Freeze();
            
            return bitmap;
        }
        catch
        {
            return null;
        }
    }
}
