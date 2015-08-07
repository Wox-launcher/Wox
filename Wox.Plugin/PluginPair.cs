﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;

namespace Wox.Plugin
{
    public class PluginPair
    {
        public IPlugin Plugin { get; set; }
        public PluginMetadata Metadata { get; set; }

        internal long InitTime { get; set; }

        internal long AvgQueryTime { get; set; }

        internal int QueryCount { get; set; }

        public override string ToString()
        {
            return Metadata.Name;
        }
    }
}
