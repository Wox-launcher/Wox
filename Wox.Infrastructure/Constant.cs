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
        public static readonly string WoxExecutable = $"{Wox}.exe";
        public const string Plugins = "Plugins";

        private static Assembly Assembly = Assembly.GetExecutingAssembly();
        public static string ExecutablePath = Path.Combine(Path.GetDirectoryName(Assembly.Location), WoxExecutable);
        public static string Version = FileVersionInfo.GetVersionInfo(ExecutablePath).ProductVersion;

        public static string ProgramDirectory = Directory.GetParent(ExecutablePath).ToString();
        public static string ApplicationDirectory = Directory.GetParent(ProgramDirectory).ToString();
        public static string RootDirectory = Directory.GetParent(ApplicationDirectory).ToString();

        public static string PreinstalledDirectory = Path.Combine(ProgramDirectory, Plugins);
        public const string Issue = "https://github.com/Wox-launcher/Wox/issues/new";

        public static readonly int ThumbnailSize = 64;
        public static string ImagesDirectory = Path.Combine(ProgramDirectory, "Images");
        public static string DefaultIcon = Path.Combine(ImagesDirectory, "app.png");
        public static string ErrorIcon = Path.Combine(ImagesDirectory, "app_error.png");

        public static string PythonPath;
        public static string EverythingSDKPath;
    }
}
