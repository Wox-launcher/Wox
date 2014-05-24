﻿using System;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32;

namespace Wox.UAC
{
    public class FileTypeAssociateInstaller
    {
        [DllImport("shell32.dll")]
        private static extern void SHChangeNotify(uint wEventId, uint uFlags, IntPtr dwItem1, IntPtr dwItem2);

        /// <summary>
        /// associate filetype with specified program
        /// </summary>
        /// <param name="filePath"></param>
        /// <param name="fileType"></param>
        /// <param name="iconPath"></param>
        /// <param name="overrides"></param>
        private static void SaveReg(string filePath, string fileType, string iconPath, bool overrides)
        {
            RegistryKey classRootKey = Registry.ClassesRoot.OpenSubKey("", true);
            RegistryKey woxKey = classRootKey.OpenSubKey(fileType, true);
            if (woxKey != null)
            {
                if (!overrides)
                {
                    return;
                }
                classRootKey.DeleteSubKeyTree(fileType);
            }
            classRootKey.CreateSubKey(fileType);
            woxKey = classRootKey.OpenSubKey(fileType, true);
            woxKey.SetValue("", "wox.wox");
            woxKey.SetValue("Content Type", "application/wox");

            RegistryKey iconKey = woxKey.CreateSubKey("DefaultIcon");
            iconKey.SetValue("", iconPath);

            woxKey.CreateSubKey("shell");
            RegistryKey shellKey = woxKey.OpenSubKey("shell", true);
            shellKey.SetValue("", "Open");
            RegistryKey openKey = shellKey.CreateSubKey("open");
            openKey.SetValue("", "Open with wox");

            openKey = shellKey.OpenSubKey("open", true);
            openKey.CreateSubKey("command");
            RegistryKey commandKey = openKey.OpenSubKey("command", true);
            string pathString = "\"" + filePath + "\" \"installPlugin\" \"%1\"";
            commandKey.SetValue("", pathString);

            //refresh cache
            SHChangeNotify(0x8000000, 0, IntPtr.Zero, IntPtr.Zero);
        }

        public void RegisterInstaller()
        {
            string filePath = Path.Combine(Path.GetDirectoryName(System.Windows.Forms.Application.ExecutablePath),  "Wox.exe");
            string iconPath = Path.Combine(Path.GetDirectoryName(System.Windows.Forms.Application.ExecutablePath),  "app.ico");

            SaveReg(filePath, ".wox", iconPath, true);
        }

    }
}
