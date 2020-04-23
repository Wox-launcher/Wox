using System;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using JetBrains.Annotations;

namespace Wox.Infrastructure
{
    public static class Constant
    {
        public const string Wox = "Wox";
        public const string Plugins = "Plugins";

        private static Assembly Assembly;
        public static string ProgramDirectory;
        public static string ExecutablePath;
        public static string ApplicationDirectory;
        public static string RootDirectory;

        public static string PreinstalledDirectory;
        public const string Issue = "https://github.com/Wox-launcher/Wox/issues/new";
        public static string Version;

        public static readonly int ThumbnailSize = 64;
        public static string ImagesDirectory;
        public static string DefaultIcon;
        public static string ErrorIcon;

        public static string PythonPath;
        public static string EverythingSDKPath;

        public static void Initialize()
        {
            Assembly = Assembly.GetExecutingAssembly();
            Version = FileVersionInfo.GetVersionInfo(Assembly.Location.NonNull()).ProductVersion;
            ProgramDirectory = Directory.GetParent(Assembly.Location.NonNull()).ToString();

            ApplicationDirectory = Directory.GetParent(ProgramDirectory).ToString();
            RootDirectory = Directory.GetParent(ApplicationDirectory).ToString();
            ExecutablePath = Path.Combine(ProgramDirectory, Wox + ".exe");
            ImagesDirectory = Path.Combine(ProgramDirectory, "Images");
            PreinstalledDirectory = Path.Combine(ProgramDirectory, Plugins);
            DefaultIcon = Path.Combine(ImagesDirectory, "app.png");
            ErrorIcon = Path.Combine(ImagesDirectory, "app_error.png");

        }
    }
}
