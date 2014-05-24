using System.Collections.Generic;

namespace Wox.Plugin
{
    public interface IPlugin
    {
        List<Result> Query(Query query);
        bool IsAvailable(Query query);

        // todo: inject context via .ctor
        void Init(PluginInitContext context); 
    }
}