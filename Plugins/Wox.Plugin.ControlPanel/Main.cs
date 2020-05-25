using NLog;
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;

namespace Wox.Plugin.ControlPanel
{
    public class Main : IPlugin, IPluginI18n
    {
        private PluginInitContext context;
        private List<ControlPanelItem> controlPanelItems = new List<ControlPanelItem>();
        private string iconFolder;
        private string fileType;

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public void Init(PluginInitContext context)
        {
            this.context = context;
            controlPanelItems = ControlPanelList.Create();
        }

        public List<Result> Query(Query query)
        {
            List<Result> results = new List<Result>();

            foreach (var item in controlPanelItems)
            {
                var titleMatch = StringMatcher.FuzzySearch(query.Search, item.LocalizedString);

                item.Score = titleMatch.Score;
                if (item.Score > 0)
                {
                    var result = new Result
                    {
                        Title = item.LocalizedString,
                        TitleHighlightData = titleMatch.MatchData,
                        SubTitle = item.InfoTip,
                        Score = item.Score,
                        IcoPath = item.IconPath,
                        Action = e =>
                        {
                            try
                            {
                                Process.Start(item.ExecutablePath);
                            }
                            catch (Exception ex)
                            {
                                ex.Data.Add(nameof(item.LocalizedString), item.LocalizedString);
                                ex.Data.Add(nameof(item.ExecutablePath), item.ExecutablePath);
                                ex.Data.Add(nameof(item.IconPath), item.IconPath);
                                ex.Data.Add(nameof(item.GUID), item.GUID);
                                Logger.WoxError($"cannot start control panel item {item.ExecutablePath}", ex);
                            }
                            return true;
                        }
                    };

                    

                    results.Add(result);
                }
            }

            List<Result> panelItems = results.OrderByDescending(o => o.Score).Take(5).ToList();
            return panelItems;
        }

        public string GetTranslatedPluginTitle()
        {
            return context.API.GetTranslation("wox_plugin_controlpanel_plugin_name");
        }

        public string GetTranslatedPluginDescription()
        {
            return context.API.GetTranslation("wox_plugin_controlpanel_plugin_description");
        }
    }
}