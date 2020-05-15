using System;
using System.IO;
using System.Linq;
using System.Windows.Media;
using System.Windows.Media.Imaging;

using Microsoft.WindowsAPICodePack.Shell;
using NLog;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.UserSettings;

namespace Wox.Image
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
        private static ImageSource _defaultFileImage;

        public static void Initialize()
        {
            string defaultFilePath = Path.Combine(Constant.ImagesDirectory, "file.png");
            _defaultFileImage = new BitmapImage(new Uri(defaultFilePath));
            _defaultFileImage.Freeze();

            _cache = new ImageCache();
        }

        private static bool IsSubdirectory(DirectoryInfo di1, DirectoryInfo di2)
        {
            bool isParent = false;
            while (di2.Parent != null)
            {
                if (di2.Parent.FullName == di1.FullName)
                {
                    isParent = true;
                    break;
                }
                else
                {
                    di2 = di2.Parent;
                }
            }
            return isParent;
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
                var bitmapImage = new BitmapImage(new Uri(path))
                {
                    DecodePixelHeight = 32,
                    DecodePixelWidth = 32
                };
                image = bitmapImage;
                return image;
            }

            bool normalImage = ImageExtensions.Any(e => path.EndsWith(e));
            if (!Path.IsPathRooted(path) && normalImage)
            {
                path = Path.Combine(Constant.ProgramDirectory, "Images", Path.GetFileName(path));
            }


            var parent1 = new DirectoryInfo(Constant.ProgramDirectory);
            var parent2 = new DirectoryInfo(DataLocation.DataDirectory());
            var subPath = new DirectoryInfo(path);
            Logger.WoxTrace($"{path} {subPath} {parent1} {parent2}");
            bool imageInsideWoxDirectory = IsSubdirectory(parent1, subPath) || IsSubdirectory(parent2, subPath);
            if (normalImage && imageInsideWoxDirectory)
            {
                image = new BitmapImage(new Uri(path))
                {
                    DecodePixelHeight = 32,
                    DecodePixelWidth = 32
                };
                image.Freeze();
                return image;
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
            ImageSource image = _cache.GetOrAdd(Constant.ErrorIcon, (_) =>
            {
                BitmapImage bitmap = new BitmapImage(new Uri(Constant.ErrorIcon))
                {
                    DecodePixelHeight = 32,
                    DecodePixelWidth = 32
                };
                bitmap.Freeze();
                return bitmap;
            });
            return image;
        }

        public static ImageSource Load(string path)
        {
            Logger.WoxDebug($"load begin {path}");
            var img = _cache.GetOrAdd(path, LoadInternal);
            Logger.WoxTrace($"load end {path}");
            return img;
        }

        /// <summary>
        /// return cache if exist,
        /// or return default image immediatly and use updateImageCallback to return new image
        /// </summary>
        /// <param name="path"></param>
        /// <param name="updateImageCallback"></param>
        /// <returns></returns>
        public static ImageSource Load(string path, Action<ImageSource> updateImageCallback)
        {
            Logger.WoxDebug($"load begin {path}");
            var img = _cache.GetOrAdd(path, _defaultFileImage, LoadInternal, updateImageCallback);
            Logger.WoxTrace($"load end {path}");
            return img;
        }

        public static ImageSource Load(string path, Func<string, ImageSource> imageFactory, Action<ImageSource> updateImageCallback)
        {
            Logger.WoxDebug($"load begin {path}");
            var img = _cache.GetOrAdd(path, _defaultFileImage, imageFactory, updateImageCallback);
            Logger.WoxTrace($"load end {path}");
            return img;
        }

    }
}
