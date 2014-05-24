using System.Collections.Generic;
using Wox.Core;
using Wox.Plugins;

namespace Wox.Plugin.SystemPlugins
{
    public abstract class BaseSystemPlugin : ISystemPlugin
    {
        public string PluginDirectory { get; set; }

        public virtual string IcoPath
        {
            get { return null; }
        }

        public virtual string Name
        {
            get { return "System workflow"; }
        }

        public virtual string Description
        {
            get { return "System workflow"; }
        }

        public abstract bool IsAvailable(Query query);
        public abstract List<Result> Query(Query query);
        public abstract void Init(PluginInitContext context);
    }
}