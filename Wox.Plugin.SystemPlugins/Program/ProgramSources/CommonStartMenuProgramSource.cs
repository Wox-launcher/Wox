﻿using System;
using System.Runtime.InteropServices;
using System.Text;
using Wox.Infrastructure.Storage.UserSettings;

namespace Wox.Plugin.SystemPlugins.Program.ProgramSources
{
    [Serializable]
    [global::System.ComponentModel.Browsable(false)]
    public class CommonStartMenuProgramSource : FileSystemProgramSource
    {
        [DllImport("shell32.dll")]
        static extern bool SHGetSpecialFolderPath(IntPtr hwndOwner, [Out] StringBuilder lpszPath, int nFolder, bool fCreate);
        const int CSIDL_COMMON_PROGRAMS = 0x17;

        private static string getPath()
        {
            StringBuilder commonStartMenuPath = new StringBuilder(560);
            SHGetSpecialFolderPath(IntPtr.Zero, commonStartMenuPath, CSIDL_COMMON_PROGRAMS, false);

            return commonStartMenuPath.ToString();
        }

        public CommonStartMenuProgramSource()
            : base(getPath())
        {
        }

        public CommonStartMenuProgramSource(ProgramSource source)
            : this()
        {
            this.BonusPoints = source.BonusPoints;
        }

        public override string ToString()
        {
            return typeof(CommonStartMenuProgramSource).Name;
        }
    }
}
