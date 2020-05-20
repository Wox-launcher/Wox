using NLog;
using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Drawing;
using System.Linq;
using System.Runtime.InteropServices;
using System.ServiceModel;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using Windows.Devices.Scanners;
using Wox.Infrastructure.Logger;
namespace Wox.Image
{
    class EmbededIcon
    {
        private delegate bool EnumResNameDelegate(IntPtr hModule, IntPtr lpszType, IntPtr lpszName, IntPtr lParam);
        [DllImport("kernel32.dll", EntryPoint = "EnumResourceNamesW", CharSet = CharSet.Unicode, SetLastError = true)]
        static extern bool EnumResourceNamesWithID(IntPtr hModule, uint lpszType, EnumResNameDelegate lpEnumFunc, IntPtr lParam);

        [DllImport("kernel32.dll", SetLastError = true)]
        static extern IntPtr LoadLibraryEx(string lpFileName, IntPtr hFile, uint dwFlags);

        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool FreeLibrary(IntPtr hModule);
        private const uint GROUP_ICON = 14;
        [DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
        static extern IntPtr LoadImage(IntPtr hinst, IntPtr lpszName, uint uType, int cxDesired, int cyDesired, uint fuLoad);

        [DllImport("user32.dll", CharSet = CharSet.Auto)]
        extern static bool DestroyIcon(IntPtr handle);

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public static ImageSource GetImage(string key, string path, int iconSize)
        {
            // https://github.com/CoenraadS/Windows-Control-Panel-Items/
            // https://gist.github.com/jnm2/79ed8330ceb30dea44793e3aa6c03f5b

            string iconStringRaw = path.Substring(key.Length);
            var iconString = new List<string>(iconStringRaw.Split(new[] { ',' }, 2));
            IntPtr iconPtr = IntPtr.Zero;
            IntPtr dataFilePointer;
            IntPtr iconIndex;
            uint LOAD_LIBRARY_AS_DATAFILE = 0x00000002;

            Logger.WoxTrace($"{nameof(iconStringRaw)}: {iconStringRaw}");

            if (string.IsNullOrEmpty(iconString[0]))
            {
                var e = new ArgumentException($"iconString empth {path}");
                e.Data.Add(nameof(path), path);
                throw e;
            }

            if (iconString[0][0] == '@')
            {
                iconString[0] = iconString[0].Substring(1);
            }

            dataFilePointer = LoadLibraryEx(iconString[0], IntPtr.Zero, LOAD_LIBRARY_AS_DATAFILE);
            if (iconString.Count == 2)
            {
                // C:\WINDOWS\system32\mblctr.exe,0
                // %SystemRoot%\System32\FirewallControlPanel.dll,-1
                var index = Math.Abs(int.Parse(iconString[1]));
                iconIndex = (IntPtr)index;
                iconPtr = LoadImage(dataFilePointer, iconIndex, 1, iconSize, iconSize, 0);
            }

            if (iconPtr == IntPtr.Zero)
            {
                IntPtr defaultIconPtr = IntPtr.Zero;
                var callback = new EnumResNameDelegate((hModule, lpszType, lpszName, lParam) =>
                {
                    defaultIconPtr = lpszName;
                    return false;
                });
                var result = EnumResourceNamesWithID(dataFilePointer, GROUP_ICON, callback, IntPtr.Zero); //Iterate through resources. 
                if (!result)
                {
                    int error = Marshal.GetLastWin32Error();
                    int userStoppedResourceEnumeration = 0x3B02;
                    if (error != userStoppedResourceEnumeration)
                    {
                        Win32Exception exception = new Win32Exception(error);
                        exception.Data.Add(nameof(path), path);
                        throw exception;
                    }
                }
                iconPtr = LoadImage(dataFilePointer, defaultIconPtr, 1, iconSize, iconSize, 0);
            }

            FreeLibrary(dataFilePointer);
            BitmapSource image;
            if (iconPtr != IntPtr.Zero)
            {
                image = Imaging.CreateBitmapSourceFromHIcon(iconPtr, Int32Rect.Empty, BitmapSizeOptions.FromEmptyOptions());
                image.CloneCurrentValue(); //Remove pointer dependancy.
                image.Freeze();
                DestroyIcon(iconPtr);
                return image;
            }
            else
            {
                var e = new ArgumentException($"iconPtr zero {path}");
                e.Data.Add(nameof(path), path);
                throw e;
            }
        }
    }
}
