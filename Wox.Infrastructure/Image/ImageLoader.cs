using System;
using System.IO;
using System.Linq;
using System.Windows.Media;
using System.Windows.Media.Imaging;
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


        public static void Initialize()
        {
        }

        public static void Save()
        {
        }


        private static ImageSource LoadInternal(string path)
        {
            Log.Debug(nameof(ImageLoader), $"image {path}");
            ImageSource image;
            try
            {
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

                if (Directory.Exists(path) || File.Exists(path))
                {
                    image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize,
                        Constant.ThumbnailSize, ThumbnailOptions.None);
                    image.Freeze();
                    return image;
                }
                else
                {
                    image = GetErrorImage();
                    return image;
                }

            }
            catch (System.Exception e)
            {
                Log.Exception($"|ImageLoader.Load|Failed to get thumbnail for {path}", e);
                image = GetErrorImage();
                return image;
            }
        }

        private static ImageSource GetErrorImage()
        {
            ImageSource image = WindowsThumbnailProvider.GetThumbnail(
                Constant.ErrorIcon, Constant.ThumbnailSize, Constant.ThumbnailSize, ThumbnailOptions.None
            );
            image.Freeze();
            return image;
        }

        public static ImageSource Load(string path)
        {
            var img = LoadInternal(path);
            return img;
        }
    }
}
