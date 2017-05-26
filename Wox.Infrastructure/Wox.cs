﻿using System;
using System.Diagnostics;
using System.IO;
using System.Reflection;

namespace Wox.Infrastructure
{
    public static class Constant
    {
        public const string Saber = "Saber";
        public const string Plugins = "Plugins";

        private static readonly Assembly Assembly = Assembly.GetExecutingAssembly();
        public static readonly string ProgramDirectory = Directory.GetParent(Assembly.Location.NonNull()).ToString();
        public static readonly string ExecutablePath = Path.Combine(ProgramDirectory, Saber + ".exe");
        public static readonly string DataDirectory = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), Saber);
        public static readonly string PluginsDirectory = Path.Combine(DataDirectory, Plugins);
        public static readonly string PreinstalledDirectory = Path.Combine(ProgramDirectory, Plugins);
        public const string Repository = "https://github.com/Wox-launcher/Wox";
        public const string Issue = "https://github.com/Wox-launcher/Wox/issues/new";
        public static readonly string Version = FileVersionInfo.GetVersionInfo(Assembly.Location.NonNull()).ProductVersion;

        public static readonly string DefaultIcon = Path.Combine(ProgramDirectory, "Images", "app.png");
        public static readonly string ErrorIcon = Path.Combine(ProgramDirectory, "Images", "app_error.png");

        public static string PythonPath;
        public static string EverythingSDKPath;
    }
}
