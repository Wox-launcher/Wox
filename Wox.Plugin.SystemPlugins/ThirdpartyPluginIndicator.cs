using System;
using System.Collections.Generic;
using System.Linq;
using Wox.Core.Data.UserSettings;

namespace Wox.Plugin.SystemPlugins
{
    public class ThirdpartyPluginIndicator : BaseSystemPlugin
    {
        private List<PluginPair> allPlugins = new List<PluginPair>();
        private Action<string> changeQuery;

        public override bool IsAvailable(Query query)
        {
            return true;
        }

        public override List<Result> Query(Query query)
        {
            List<Result> results = new List<Result>();

            foreach (PluginMetadata metadata in allPlugins.Select(o => o.Metadata))
            {
                if (metadata.ActionKeyword.StartsWith(query.Raw))
                {
                    PluginMetadata metadataCopy = metadata;
                    Result result = new Result
                    {
                        Title = metadata.ActionKeyword,
                        SubTitle = string.Format("Activate {0} plugin", metadata.Name),
                        Score = 50,
                        IcoPath = "Images/work.png",
                        Action = (c) =>
                        {
                            changeQuery(metadataCopy.ActionKeyword + " ");
                            return false;
                        },
                    };
                    results.Add(result);
                }
            }

            results.AddRange(UserSettingStorage.Instance.WebSearches.Where(o => o.ActionWord.StartsWith(query.Raw) && o.Enabled).Select(n => new Result()
            {
                Title = n.ActionWord,
                SubTitle = string.Format("Activate {0} web search", n.ActionWord),
                Score = 50,
                IcoPath = "Images/work.png",
                Action = (c) =>
                {
                    changeQuery(n.ActionWord + " ");
                    return false;
                }
            }));

            return results;
        }

        public override void Init(PluginInitContext context)
        {
            allPlugins = context.Plugins;
            changeQuery = context.ChangeQuery;
        }


        public override string Name
        {
            get { return "Plugins"; }
        }

        public override string IcoPath
        {
            get { return @"Images\work.png"; }
        }

        public override string Description
        {
            get { return base.Description; }
        }
    }
}
