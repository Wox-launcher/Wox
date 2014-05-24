using System.Collections.Generic;

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

        public List<Result> Query(Query query)
        {
            return QueryInternal(query);
        }

        public abstract bool IsAvailable(Query query);

        public void Init(PluginInitContext context)
        {
            InitInternal(context);
        }

        protected abstract List<Result> QueryInternal(Query query);

        protected abstract void InitInternal(PluginInitContext context);
    }
}