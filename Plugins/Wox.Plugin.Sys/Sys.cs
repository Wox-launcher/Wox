using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Windows.Forms;
using Wox.Infrastructure;
using Control = System.Windows.Controls.Control;

namespace Wox.Plugin.Sys
{
    public class Sys : IPlugin, ISettingProvider, IPluginI18n
    {
        List<Result> availableResults = new List<Result>();
        private PluginInitContext context;

        enum RecycleFlag : uint
        {
            SHERB_NOCONFIRMATION = 0x00000001, // No confirmation, when emptying
            SHERB_NOPROGRESSUI = 0x00000002, // No progress tracking window during the emptying of the recycle bin

            // If we don't want the trashcan to play any sound when emptying we should uncomment the code below! 
            // SHERB_NOSOUND = 0x00000004 // No sound when the emptying of the recycle bin is complete
        }


        #region DllImport

        internal const int EWX_LOGOFF = 0x00000000;
        internal const int EWX_SHUTDOWN = 0x00000001;
        internal const int EWX_REBOOT = 0x00000002;
        internal const int EWX_FORCE = 0x00000004;
        internal const int EWX_POWEROFF = 0x00000008;
        internal const int EWX_FORCEIFHUNG = 0x00000010;
        [DllImport("user32")]
        private static extern bool ExitWindowsEx(uint uFlags, uint dwReason);
        [DllImport("user32")]
        private static extern void LockWorkStation();
        [DllImport("Shell32.dll", CharSet = CharSet.Unicode)]
        private static extern uint SHEmptyRecycleBin(System.IntPtr hwnd, string pszRootPath, RecycleFlag dwFlags);

        #endregion

        public Control CreateSettingPanel()
        {
            return new SysSettings(availableResults);
        }

        public List<Result> Query(Query query)
        {
            List<Result> results = new List<Result>();
            foreach (Result availableResult in availableResults)
            {
                int titleScore = StringMatcher.Match(availableResult.Title, query.Search);
                int subTitleScore = StringMatcher.Match(availableResult.SubTitle, query.Search);
                if (titleScore > 0 || subTitleScore > 0)
                {
                    availableResult.Score = titleScore > 0 ? titleScore : subTitleScore;
                    results.Add(availableResult);
                }
            }
            return results;
        }

        public void Init(PluginInitContext context)
        {
            this.context = context;
            LoadCommands();
        }

        private void LoadCommands()
        {
            availableResults.AddRange(new Result[] {
                new Result
                {
                    Title = "Shutdown",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_shutdown_computer"),
                    IcoPath = "Images\\exit.png",
                    Action = (c) =>
                    {
                        if (MessageBox.Show("Are you sure you want to shut the computer down?","Shutdown Computer?",MessageBoxButtons.YesNo,MessageBoxIcon.Warning) == DialogResult.Yes) {
                            Process.Start("shutdown", "/s /t 0");
                        }
                        return true;
                    }
                },
                new Result
                {
                    Title = "Restart",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_restart_computer"),
                    IcoPath = "Images\\restartcomp.png",
                    Action = (c) =>
                    {
                        if (MessageBox.Show("Are you sure you want to restart the computer?","Restart Computer?",MessageBoxButtons.YesNo,MessageBoxIcon.Warning) == DialogResult.Yes) {
                            Process.Start("shutdown", "/r /t 0");
                        }
                        return true;
                    }
                },
                new Result
                {
                    Title = "Log off",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_log_off"),
                    IcoPath = "Images\\logoff.png",
                    Action = (c) => ExitWindowsEx(EWX_LOGOFF, 0)
                },
                new Result
                {
                    Title = "Lock",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_lock"),
                    IcoPath = "Images\\lock.png",
                    Action = (c) =>
                    {
                        LockWorkStation();
                        return true;
                    }
                },
                new Result
                {
                    Title = "Sleep",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_sleep"),
                    IcoPath = "Images\\sleep.png",
                    Action = (c) => Application.SetSuspendState(PowerState.Suspend, false, false)
                },
                new Result
                {
                    Title = "Empty Recycle Bin",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_emptyrecyclebin"),
                    IcoPath = "Images\\recyclebin.png",
                    Action = (c) =>
                    {
                        if (MessageBox.Show("Are you sure that you want to permanently delete all items in your recycle bin?","Empty recycle bin?",MessageBoxButtons.YesNo,MessageBoxIcon.Warning) == DialogResult.Yes) {
                                uint result = SHEmptyRecycleBin(System.IntPtr.Zero, null, RecycleFlag.SHERB_NOCONFIRMATION | RecycleFlag.SHERB_NOPROGRESSUI);
                        }
                        return true;
                    }
                },
                new Result
                {
                    Title = "Exit",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_exit"),
                    IcoPath = "Images\\app.png",
                    Action = (c) =>
                    {
                        context.API.CloseApp();
                        return true;
                    }
                },
                new Result
                {
                    Title = "Restart Wox",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_restart"),
                    IcoPath = "Images\\restart.png",
                    Action = (c) =>
                    {
                        ProcessStartInfo Info = new ProcessStartInfo();
                        Info.Arguments = "/C ping 127.0.0.1 -n 1 && \"" + Application.ExecutablePath + "\"";
                        Info.WindowStyle = ProcessWindowStyle.Hidden;
                        Info.CreateNoWindow = true;
                        Info.FileName = "cmd.exe";
                        Process.Start(Info);
                        context.API.CloseApp();
                        return true;
                    }
                },
                new Result
                {
                    Title = "Settings",
                    SubTitle = context.API.GetTranslation("wox_plugin_sys_setting"),
                    IcoPath = "Images\\app.png",
                    Action = (c) =>
                    {
                        context.API.OpenSettingDialog();
                        return true;
                    }
                }
            });
        }

        public string GetLanguagesFolder()
        {
            return Path.Combine(Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location), "Languages");

        }

        public string GetTranslatedPluginTitle()
        {
            return context.API.GetTranslation("wox_plugin_sys_plugin_name");
        }

        public string GetTranslatedPluginDescription()
        {
            return context.API.GetTranslation("wox_plugin_sys_plugin_description");
        }
    }
}
