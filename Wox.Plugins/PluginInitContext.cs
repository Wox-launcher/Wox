﻿using System;
using System.Collections.Generic;
using Wox.Core;

namespace Wox.Plugins
{
    public class PluginInitContext
    {
        public List<PluginPair> Plugins { get; set; }
        public PluginMetadata CurrentPluginMetadata { get; set; }


        public Action<string> ChangeQuery { get; set; }
        public Action CloseApp { get; set; }
        public Action HideApp { get; set; }
        public Action ShowApp { get; set; }
        public Action<string, string, string> ShowMsg { get; set; }
        public Action OpenSettingDialog { get; set; }

        public Action<string> ShowCurrentResultItemTooltip { get; set; }

        /// <summary>
        /// reload all plugins
        /// </summary>
        public Action ReloadPlugins { get; set; }

        public Action<string> InstallPlugin { get; set; }

        public Action StartLoadingBar { get; set; }
        public Action StopLoadingBar { get; set; }

        public Func<string, bool> ShellRun { get; set; }

        public Action<Query, List<Result>> PushResults { get; set; }
    }
}
