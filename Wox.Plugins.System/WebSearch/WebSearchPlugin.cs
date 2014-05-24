using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using Wox.Core;
using Wox.Core.Data.UserSettings;
using Wox.Plugins.System.SuggestionSources;

namespace Wox.Plugins.System.WebSearch
{
    public class WebSearchPlugin : BaseSystemPlugin, ISettingProvider
    {
        private PluginInitContext context;

        public override bool IsAvailable(Query query)
        {
            return !string.IsNullOrEmpty(query.Command);
        }

        public override List<Result> Query(Query query)
        {
            List<Result> results = new List<Result>();

            Core.Data.UserSettings.WebSearch webSearch =
                UserSettingStorage.Instance.WebSearches.FirstOrDefault(o => o.ActionWord == query.Command && o.Enabled);

            if (webSearch != null)
            {
                string keyword = query.Arguments.Length > 0 ? query.Tail : "";
                string title = keyword;
                string subtitle = "Search " + webSearch.Title;
                if (string.IsNullOrEmpty(keyword))
                {
                    title = subtitle;
                    subtitle = null;
                }
                context.PushResults(query, new List<Result>()
                {
                    new Result()
                    {
                        Title = title,
                        SubTitle = subtitle,
                        Score = 6,
                        IcoPath = webSearch.IconPath,
                        Action = (c) =>
                        {
                            Process.Start(webSearch.Url.Replace("{q}", keyword));
                            return true;
                        }
                    }
                });

                if (!string.IsNullOrEmpty(keyword))
                {
                    ISuggestionSource sugg = new Google();
                    var result = sugg.GetSuggestions(keyword);
                    if (result != null)
                    {
                        context.PushResults(query, result.Select(o => new Result()
                        {
                            Title = o,
                            SubTitle = subtitle,
                            Score = 5,
                            IcoPath = webSearch.IconPath,
                            Action = (c) =>
                            {
                                Process.Start(webSearch.Url.Replace("{q}", o));
                                return true;
                            }
                        }).ToList());
                    }
                }
            }

            return results;
        }

        public override void Init(PluginInitContext context)
        {
            this.context = context;

            if (UserSettingStorage.Instance.WebSearches == null)
                UserSettingStorage.Instance.WebSearches = UserSettingStorage.Instance.LoadDefaultWebSearches();
        }

        public override string Name
        {
            get { return "Web Searches"; }
        }

        public override string IcoPath
        {
            get { return @"Images\app.png"; }
        }

        public override string Description
        {
            get { return base.Description; }
        }

        #region ISettingProvider Members

        public global::System.Windows.Controls.Control CreateSettingPanel()
        {
            return new WebSearchesSetting();
        }

        #endregion
    }
}
