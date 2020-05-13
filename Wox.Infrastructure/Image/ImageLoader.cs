using System;
using System.IO;
using System.Windows.Media;
using System.Windows.Media.Imaging;

using Microsoft.WindowsAPICodePack.Shell;
using NLog;
using Wox.Infrastructure.Logger;

namespace Wox.Infrastructure.Image
{
    public static class ImageLoader
    {
        private static readonly string[] ImageExtensions =
        {
            ".png",
            ".jpg",
            ".jpeg",
            ".gif",
            ".bmp",
            ".tiff",
            ".ico"
        };
        private static ImageCache _cache;

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();


        public static void Initialize()
        {
            _cache = new ImageCache();
        }

        private static ImageSource LoadInternal(string path)
        {
            Logger.WoxDebug($"load from disk {path}");

            ImageSource image;

            if (string.IsNullOrEmpty(path))
            {
                image = GetErrorImage();
                return image;
            }

            if (path.StartsWith("data:", StringComparison.OrdinalIgnoreCase))
            {
                image = new BitmapImage(new Uri(path));
                image.Freeze();
                return image;
            }

            if (!Path.IsPathRooted(path))
            {
                path = Path.Combine(Constant.ProgramDirectory, "Images", Path.GetFileName(path));
            }

            if (Directory.Exists(path))
            {
                // can be extended to support guid things
                ShellObject shell = ShellFile.FromParsingName(path);
                image = shell.Thumbnail.SmallBitmapSource;
                image.Freeze();
                return image;
            }

            if (File.Exists(path))
            {
                try
                {
                    // https://stackoverflow.com/a/1751610/2833083
                    // https://stackoverflow.com/questions/21751747/extract-thumbnail-for-any-file-in-windows
                    // https://docs.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ishellitemimagefactory-getimage
                    ShellFile shell = ShellFile.FromFilePath(path);
                    // https://github.com/aybe/Windows-API-Code-Pack-1.1/blob/master/source/WindowsAPICodePack/Shell/Common/ShellThumbnail.cs#L333
                    // https://github.com/aybe/Windows-API-Code-Pack-1.1/blob/master/source/WindowsAPICodePack/Shell/Common/DefaultShellImageSizes.cs#L46
                    // small is (32, 32)
                    image = shell.Thumbnail.SmallBitmapSource;
                    image.Freeze();
                    return image;
                }
                catch (ShellException e1)
                {
                    try
                    {
                        // sometimes first try will throw exception, but second try will be ok.
                        // so we try twice
                        // Error while extracting thumbnail for C:\\ProgramData\\Microsoft\\Windows\\Start Menu\\Programs\\Steam\\Steam.lnk
                        ShellFile shellFile = ShellFile.FromFilePath(path);
                        image = shellFile.Thumbnail.SmallBitmapSource;
                        image.Freeze();
                        return image;
                    }
                    catch (System.Exception e2)
                    {
                        Logger.WoxError($"Failed to get thumbnail, first, {path}", e1);
                        Logger.WoxError($"Failed to get thumbnail, second, {path}", e2);
                        image = GetErrorImage();
                        return image;
                    }
                }
            }
            else
            {
                image = GetErrorImage();
                return image;
            }


        }

        private static ImageSource GetErrorImage()
        {
            ShellFile shellFile = ShellFile.FromFilePath(Constant.ErrorIcon);
            // small is (32, 32), refer comment above
            ImageSource image = shellFile.Thumbnail.SmallBitmapSource;
            image.Freeze();
            return image;
        }

        public static ImageSource Load(string path)
        {
            Logger.WoxDebug($"load begin {path}");
            var img = _cache.GetOrAdd(path, LoadInternal);
            Logger.WoxTrace($"load end {path}");
            return img;
        }
    }
}
