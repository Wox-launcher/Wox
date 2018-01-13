using System;
using System.Collections.Concurrent;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.Storage;

namespace Wox.Infrastructure.Image
{
    public static class ImageLoader
    {
        private static readonly ImageCache ImageCache = new ImageCache();
        private static BinaryStorage<ConcurrentDictionary<string, int>> _storage;


        private static readonly string[] ImageExtions =
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
            _storage = new BinaryStorage<ConcurrentDictionary<string, int>> ("Image");
            ImageCache.Usage = _storage.TryLoad(new ConcurrentDictionary<string, int>());

            foreach (var icon in new[] { Constant.DefaultIcon, Constant.ErrorIcon })
            {
                ImageSource img = new BitmapImage(new Uri(icon));
                img.Freeze();
                ImageCache[icon] = img;
            }
            Task.Run(() =>
            {
                Stopwatch.Normal("|ImageLoader.Initialize|Preload images cost", () =>
                {
                    ImageCache.Usage.AsParallel().Where(i => !ImageCache.ContainsKey(i.Key)).ForAll(i =>
                    {
                        var img = Load(i.Key);
                        if (img != null)
                        {
                            ImageCache[i.Key] = img;
                        }
                    });
                });
                Log.Info($"|ImageLoader.Initialize|Number of preload images is <{ImageCache.Usage.Count}>");
            });
        }

        public static void Save()
        {
            ImageCache.Cleanup();
            _storage.Save(ImageCache.Usage);
        }

        private static ImageSource ShellIcon(string fileName)
        {
            try
            {
                // http://blogs.msdn.com/b/oldnewthing/archive/2011/01/27/10120844.aspx
                var shfi = new SHFILEINFO();
                var himl = SHGetFileInfo(
                    fileName,
                    FILE_ATTRIBUTE_NORMAL,
                    ref shfi,
                    (uint)Marshal.SizeOf(shfi),
                    SHGFI_SYSICONINDEX
                );

                if (himl != IntPtr.Zero)
                {
                    var hIcon = ImageList_GetIcon(himl, shfi.iIcon, ILD_NORMAL);
                    // http://stackoverflow.com/questions/1325625/how-do-i-display-a-windows-file-icon-in-wpf
                    var img = Imaging.CreateBitmapSourceFromHIcon(
                        hIcon,
                        Int32Rect.Empty,
                        BitmapSizeOptions.FromEmptyOptions()
                    );
                    DestroyIcon(hIcon);
                    return img;
                }
                else
                {
                    return new BitmapImage(new Uri(Constant.ErrorIcon));
                }
            }
            catch (System.Exception e)
            {
                Log.Exception($"|ImageLoader.ShellIcon|can't get shell icon for <{fileName}>", e);
                return ImageCache[Constant.ErrorIcon];
            }
        }
        
        public static ImageSource Load(string path)
        {
            ImageSource image;
            try
            {
                if (string.IsNullOrEmpty(path))
                {
                    return ImageCache[Constant.ErrorIcon];
                }
                if (ImageCache.ContainsKey(path))
                {
                    return ImageCache[path];
                }
                
                if (path.StartsWith("data:", StringComparison.OrdinalIgnoreCase))
                {
                    return new BitmapImage(new Uri(path));
                }

                if (Path.IsPathRooted(path))
                {
                    if (Directory.Exists(path))
                    {
                        image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize, Constant.ThumbnailSize, ThumbnailOptions.None);
                    }
                    else if (File.Exists(path))
                    {
                        var externsion = Path.GetExtension(path).ToLower();
                        if (ImageExtions.Contains(externsion))
                        {
                            try
                            {
                                image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize, Constant.ThumbnailSize, ThumbnailOptions.ThumbnailOnly);
                            }
                            catch(COMException e)
                            {
                                // failed loading image. probably not really an image or corrupted
                                // force load icon thumbnail
                                image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize, Constant.ThumbnailSize, ThumbnailOptions.IconOnly);
                            }
                        }
                        else
                        {
                            image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize, Constant.ThumbnailSize, ThumbnailOptions.None);
                        }
                    }
                    else
                    {
                        image = ImageCache[Constant.ErrorIcon];
                        path = Constant.ErrorIcon;
                    }
                }
                else
                {
                    var defaultDirectoryPath = Path.Combine(Constant.ProgramDirectory, "Images", Path.GetFileName(path));
                    if (File.Exists(defaultDirectoryPath))
                    {
                        image = WindowsThumbnailProvider.GetThumbnail(path, Constant.ThumbnailSize, Constant.ThumbnailSize, ThumbnailOptions.None);
                    }
                    else
                    {
                        image = ImageCache[Constant.ErrorIcon];
                        path = Constant.ErrorIcon;
                    }
                }
                ImageCache[path] = image;
                image.Freeze();
                
            }
            catch (System.Exception e)
            {
                Log.Exception($"Failed to get thumbnail for {path}", e);

                image = ImageCache[Constant.ErrorIcon];
                path = Constant.ErrorIcon;
            }
            return image;
        }

        private const int NAMESIZE = 80;
        private const int MAX_PATH = 256;
        private const uint SHGFI_SYSICONINDEX = 0x000004000; // get system icon index
        private const uint FILE_ATTRIBUTE_NORMAL = 0x00000080;
        private const uint ILD_NORMAL = 0x00000000;

        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        private struct SHFILEINFO
        {
            readonly IntPtr hIcon;
            internal readonly int iIcon;
            readonly uint dwAttributes;
            [MarshalAs(UnmanagedType.ByValTStr, SizeConst = MAX_PATH)] readonly string szDisplayName;
            [MarshalAs(UnmanagedType.ByValTStr, SizeConst = NAMESIZE)] readonly string szTypeName;
        }
        
        [DllImport("Shell32.dll", CharSet = CharSet.Unicode)]
        private static extern IntPtr SHGetFileInfo(string pszPath, uint dwFileAttributes, ref SHFILEINFO psfi, uint cbFileInfo, uint uFlags);

        [DllImport("User32.dll")]
        private static extern int DestroyIcon(IntPtr hIcon);

        [DllImport("comctl32.dll")]
        private static extern IntPtr ImageList_GetIcon(IntPtr himl, int i, uint flags);
    }
}
