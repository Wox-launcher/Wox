
using System;
using System.ComponentModel;
using System.Collections.Generic;
using System.IO;
using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Imaging;

namespace Wox.Infrastructure.Image 
{
    static class WindowsThumbnailCacheAPI 
    {
        private static readonly IThumbnailCache _cache;

        public static readonly HashSet<string> NotCachableExtensions;

        static WindowsThumbnailCacheAPI() {
            NotCachableExtensions = new HashSet<string>(StringComparer.OrdinalIgnoreCase) {
                ".lnk",
                ".exe",
                ".bat",
                ".cmd",
                ".pif",
                ".appref-ms"
            };

            Guid CLSID_LocalThumbnailCache = new Guid("50EF4544-AC9F-4A8E-B21B-8A26180DB13F");
            _cache = (IThumbnailCache)Activator.CreateInstance(Type.GetTypeFromCLSID(CLSID_LocalThumbnailCache));
        }

        public static ImageSource GetThumbnail(string path, int size) {
            IShellItem item = GetShellItemFromPath(path);
            if (item == null) {
                return null;
            }


            ISharedBitmap bitmap = null;
            IntPtr hBitmap = IntPtr.Zero;
            try {

                
                WTS_CACHEFLAGS cacheFlags;
                WTS_THUMBNAILID thumbid;

                uint ret;

                ret = _cache.GetThumbnail(item, (uint)size, WTS_FLAGS.WTS_EXTRACT, out bitmap, out cacheFlags, out thumbid);

                if (ret != 0 || bitmap == null) {
                    return null;
                }

                hBitmap = IntPtr.Zero;
                bitmap.Detach(out hBitmap);


                var iSrc = Imaging.CreateBitmapSourceFromHBitmap(
                    hBitmap, IntPtr.Zero, Int32Rect.Empty,
                    BitmapSizeOptions.FromEmptyOptions()
                );

                iSrc.Freeze();
                
                return iSrc;    
            } finally {
                if (item != null) Marshal.ReleaseComObject(item);
                if (hBitmap != IntPtr.Zero) { DeleteObject(hBitmap); }
                if (bitmap != null) Marshal.ReleaseComObject(bitmap); 
            }
        }


        private static IShellItem GetShellItemFromPath(string path) {
            if (!Path.IsPathRooted(path)) {
                path = Path.Combine(Environment.CurrentDirectory, path);
            }

            path = Path.GetFullPath(path); //converts / to \

            IShellItem item = null;
            IntPtr idl;
            uint atts = 0;

            if ((SHILCreateFromPath(path, out idl, ref atts) == 0) &&
                (SHCreateShellItem(IntPtr.Zero, IntPtr.Zero, idl, out item) == 0)) {

                return item;
            } else {
                return null;
            }
        }


        [ComImport]
        [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        [Guid("F676C15D-596A-4ce2-8234-33996F445DB1")]
        private interface IThumbnailCache {
            uint GetThumbnail(
                [In] IShellItem pShellItem,
                [In] uint cxyRequestedThumbSize,
                [In] WTS_FLAGS flags /*default:  WTS_FLAGS.WTS_EXTRACT*/,
                [Out][MarshalAs(UnmanagedType.Interface)] out ISharedBitmap ppvThumb,
                [Out] out WTS_CACHEFLAGS pOutFlags,
                [Out] out WTS_THUMBNAILID pThumbnailID
            );

            void GetThumbnailByID(
                [In, MarshalAs(UnmanagedType.Struct)] WTS_THUMBNAILID thumbnailID,
                [In] uint cxyRequestedThumbSize,
                [Out][MarshalAs(UnmanagedType.Interface)] out ISharedBitmap ppvThumb,
                [Out] out WTS_CACHEFLAGS pOutFlags
            );
        }

        [Flags]
        enum WTS_FLAGS : uint {
            WTS_EXTRACT = 0x00000000,
            WTS_INCACHEONLY = 0x00000001,
            WTS_FASTEXTRACT = 0x00000002,
            WTS_SLOWRECLAIM = 0x00000004,
            WTS_FORCEEXTRACTION = 0x00000008,
            WTS_EXTRACTDONOTCACHE = 0x00000020,
            WTS_SCALETOREQUESTEDSIZE = 0x00000040,
            WTS_SKIPFASTEXTRACT = 0x00000080,
            WTS_EXTRACTINPROC = 0x00000100
        }

        [Flags]
        enum WTS_CACHEFLAGS : uint {
            WTS_DEFAULT = 0x00000000,
            WTS_LOWQUALITY = 0x00000001,
            WTS_CACHED = 0x00000002
        }

        [StructLayout(LayoutKind.Sequential, Size = 16), Serializable]
        struct WTS_THUMBNAILID {
            [MarshalAs(UnmanagedType.ByValArray, SizeConst = 16)]
            byte[] rgbKey;
        }

        [ComImport()]
        [Guid("091162a4-bc96-411f-aae8-c5122cd03363")]
        [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        private interface ISharedBitmap {
            uint Detach(
                [Out] out IntPtr phbm
            );

            uint GetFormat(
                [Out]  out WTS_ALPHATYPE pat
            );

            uint GetSharedBitmap(
                [Out] out IntPtr phbm
            );

            uint GetSize(
                [Out, MarshalAs(UnmanagedType.Struct)] out SIZE pSize
            );

            uint InitializeBitmap(
                [In]  IntPtr hbm,
                [In]  WTS_ALPHATYPE wtsAT
            );
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct SIZE {
            private int cx;
            private int cy;

            private SIZE(int cx, int cy) {
                this.cx = cx;
                this.cy = cy;
            }
        }

        private enum WTS_ALPHATYPE : uint {
            WTSAT_UNKNOWN = 0,
            WTSAT_RGB = 1,
            WTSAT_ARGB = 2
        }

        [ComImport]
        [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        [Guid("43826d1e-e718-42ee-bc55-a1e261c37bfe")]
        private interface IShellItem {
            void BindToHandler(IntPtr pbc,
                [MarshalAs(UnmanagedType.LPStruct)]Guid bhid,
                [MarshalAs(UnmanagedType.LPStruct)]Guid riid,
                out IntPtr ppv);

            void GetParent(out IShellItem ppsi);

            void GetDisplayName(SIGDN sigdnName, out IntPtr ppszName);

            void GetAttributes(uint sfgaoMask, out uint psfgaoAttribs);

            void Compare(IShellItem psi, uint hint, out int piOrder);
        }

        private enum SIGDN : uint {
            NORMALDISPLAY = 0,
            PARENTRELATIVEPARSING = 0x80018001,
            PARENTRELATIVEFORADDRESSBAR = 0x8001c001,
            DESKTOPABSOLUTEPARSING = 0x80028000,
            PARENTRELATIVEEDITING = 0x80031001,
            DESKTOPABSOLUTEEDITING = 0x8004c000,
            FILESYSPATH = 0x80058000,
            URL = 0x80068000
        }

        [DllImport("shell32.dll", PreserveSig = true)]
        private static extern int SHCreateShellItem(IntPtr pidlParent, IntPtr psfParent, IntPtr pidl, out IShellItem ppsi);

        [DllImport("shell32.dll")]
        private static extern int SHILCreateFromPath([MarshalAs(UnmanagedType.LPWStr)] string pszPath, out IntPtr ppIdl, ref uint rgflnOut);

        [DllImport("gdi32.dll")]
        private static extern bool DeleteObject(IntPtr handle);

    }
}
