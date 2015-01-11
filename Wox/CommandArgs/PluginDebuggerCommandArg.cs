﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Windows.Input;

namespace Wox.CommandArgs
{
    public class PluginDebuggerCommandArg : ICommandArg
    {
        public string Command
        {
            get { return "plugindebugger"; }
        }

        public void Execute(IList<string> args)
        {
            if (args.Count > 0)
            {
                var pluginFolderPath = args[0];
                PluginLoader.Plugins.ActivatePluginDebugger(pluginFolderPath);
            }
        }
    }
}
