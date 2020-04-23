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

        private class ImageResult
        {
            public ImageResult(ImageSource imageSource, ImageType imageType)
            {
                ImageSource = imageSource;
                ImageType = imageType;
            }

            public ImageType ImageType { get; }
            public ImageSource ImageSource { get; }
        }

        private enum ImageType
        {
            File,
            Folder,
            Data,
            ImageFile,
            Error,
            Cache
        }

        private static ImageResult LoadInternal(string path)
        {
            Log.Debug(nameof(ImageLoader), $"image {path}");
            ImageSource image;
            ImageType type;
            try
            {
                if (string.IsNullOrEmpty(path))
                {
                    image = new BitmapImage(new Uri(Constant.ErrorIcon));
                    type = ImageType.Error;
                    image.Freeze();
                    return new ImageResult(image, type);
                }

                if (path.StartsWith("data:", StringComparison.OrdinalIgnoreCase))
                {
                    image = new BitmapImage(new Uri(path));
                    image.Freeze();
                    type = ImageType.Data;
                    return new ImageResult(image, type);
                }

                if (!Path.IsPathRooted(path))
                {
                    path = Path.Combine(Constant.ProgramDirectory, "Images", Path.GetFileName(path));
                }

                if (Directory.Exists(path))
                {
                    /* Directories can also have thumbnails instead of shell icons.
                     * Generating thumbnails for a bunch of folders while scrolling through
                     * results from Everything makes a big impact on performance and 
                     * Wox responsibility. 
                     * - Solution: just load the icon
                     */
                    type = ImageType.Folder;
                    image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize,
                        Constant.ThumbnailSize, ThumbnailOptions.IconOnly);
                    image.Freeze();
                    return new ImageResult(image, type);

                }
                else if (File.Exists(path))
                {
                    var extension = Path.GetExtension(path).ToLower();
                    if (ImageExtensions.Contains(extension))
                    {
                        type = ImageType.ImageFile;
                        /* Although the documentation for GetImage on MSDN indicates that 
                            * if a thumbnail is available it will return one, this has proved to not
                            * be the case in many situations while testing. 
                            * - Solution: explicitly pass the ThumbnailOnly flag
                            */
                        image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize,
                            Constant.ThumbnailSize, ThumbnailOptions.ThumbnailOnly);
                        image.Freeze();
                        return new ImageResult(image, type);
                    }
                    else
                    {
                        type = ImageType.File;
                        image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize,
                            Constant.ThumbnailSize, ThumbnailOptions.None);
                        image.Freeze();
                        return new ImageResult(image, type);
                    }
                }
                else
                {
                    image = new BitmapImage(new Uri(Constant.ErrorIcon));
                    type = ImageType.Error;
                    image.Freeze();
                    return new ImageResult(image, type);
                }

            }
            catch (System.Exception e)
            {
                Log.Exception($"|ImageLoader.Load|Failed to get thumbnail for {path}", e);
                type = ImageType.Error;
                image = new BitmapImage(new Uri(Constant.ErrorIcon));
                image.Freeze();
                return new ImageResult(image, type);
            }
        }

        public static ImageSource Load(string path)
        {
            var imageResult = LoadInternal(path);
            var img = imageResult.ImageSource;
            return img;
        }
    }
}
