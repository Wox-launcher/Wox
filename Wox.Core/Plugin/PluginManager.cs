using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using NLog;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.Storage;
using Wox.Infrastructure.UserSettings;
using Wox.Plugin;

namespace Wox.Core.Plugin
{
    /// <summary>
    /// The entry for managing Wox plugins
    /// </summary>
    public static class PluginManager
    {
        private static IEnumerable<PluginPair> _contextMenuPlugins;

        /// <summary>
        /// Directories that will hold Wox plugin directory
        /// </summary>

        public static List<PluginPair> AllPlugins { get; private set; }
        public static readonly List<PluginPair> GlobalPlugins = new List<PluginPair>();
        public static readonly Dictionary<string, PluginPair> NonGlobalPlugins = new Dictionary<string, PluginPair>();

        public static IPublicAPI API { private set; get; }

        // todo this should not be public, the indicator function should be embeded 
        public static PluginsSettings Settings;
        private static List<PluginMetadata> _metadatas;
        private static readonly string[] Directories = { Constant.PreinstalledDirectory, DataLocation.PluginsDirectory };

        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

        private static void ValidateUserDirectory()
        {
            if (!Directory.Exists(DataLocation.PluginsDirectory))
            {
                Directory.CreateDirectory(DataLocation.PluginsDirectory);
            }
        }

        private static void DeletePythonBinding()
        {
            const string binding = "wox.py";
            var directory = DataLocation.PluginsDirectory;
            foreach (var subDirectory in Directory.GetDirectories(directory))
            {
                var path = Path.Combine(subDirectory, binding);
                if (File.Exists(path))
                {
                    File.Delete(path);
                }
            }
        }

        public static void Save()
        {
            foreach (var plugin in AllPlugins)
            {
                var savable = plugin.Plugin as ISavable;
                savable?.Save();
            }
        }

        public static void ReloadData()
        {
            foreach (var plugin in AllPlugins)
            {
                var reloadablePlugin = plugin.Plugin as IReloadable;
                reloadablePlugin?.ReloadData();
            }
        }

        static PluginManager()
        {
            ValidateUserDirectory();
            // force old plugins use new python binding
            DeletePythonBinding();
        }

        /// <summary>
        /// because InitializePlugins needs API, so LoadPlugins needs to be called first
        /// todo The API should be removed
        /// </summary>
        /// <param name="settings"></param>
        public static void LoadPlugins(PluginsSettings settings)
        {
            _metadatas = PluginConfig.Parse(Directories);
            Settings = settings;
            Settings.UpdatePluginSettings(_metadatas);
            AllPlugins = PluginsLoader.Plugins(_metadatas, Settings);
        }

        /// <summary>
        /// Call initialize for all plugins
        /// </summary>
        /// <returns>return the list of failed to init plugins or null for none</returns>
        public static void InitializePlugins(IPublicAPI api)
        {
            API = api;
            var failedPlugins = new ConcurrentQueue<PluginPair>();
            Parallel.ForEach(AllPlugins, pair =>
            {
                try
                {
                    var milliseconds = Logger.StopWatchDebug($"Init method time cost for <{pair.Metadata.Name}>", () =>
                    {
                        pair.Plugin.Init(new PluginInitContext
                        {
                            CurrentPluginMetadata = pair.Metadata,
                            API = API
                        });
                    });
                    pair.Metadata.InitTime += milliseconds;
                    Logger.WoxInfo($"Total init cost for <{pair.Metadata.Name}> is <{pair.Metadata.InitTime}ms>");
                }
                catch (Exception e)
                {
                    e.Data.Add(nameof(pair.Metadata.ID), pair.Metadata.ID);
                    e.Data.Add(nameof(pair.Metadata.Name), pair.Metadata.Name);
                    e.Data.Add(nameof(pair.Metadata.PluginDirectory), pair.Metadata.PluginDirectory);
                    e.Data.Add(nameof(pair.Metadata.Website), pair.Metadata.Website);
                    Logger.WoxError($"Fail to Init plugin: {pair.Metadata.Name}", e);
                    pair.Metadata.Disabled = true;
                    failedPlugins.Enqueue(pair);
                }
            });

            _contextMenuPlugins = GetPluginsForInterface<IContextMenu>();
            foreach (var plugin in AllPlugins)
            {
                if (IsGlobalPlugin(plugin.Metadata))
                    GlobalPlugins.Add(plugin);

                // Plugins may have multiple ActionKeywords, eg. WebSearch
                plugin.Metadata.ActionKeywords.Where(x => x != Query.GlobalPluginWildcardSign)
                                                .ToList()
                                                .ForEach(x => NonGlobalPlugins[x] = plugin);
            }

            if (failedPlugins.Any())
            {
                var failed = string.Join(",", failedPlugins.Select(x => x.Metadata.Name));
                API.ShowMsg($"Fail to Init Plugins", $"Plugins: {failed} - fail to load and would be disabled, please contact plugin creator for help", "", false);
            }
        }

        public static void InstallPlugin(string path)
        {
            PluginInstaller.Install(path);
        }

        public static List<Result> QueryForPlugin(PluginPair pair, Query query)
        {
            try
            {
                var metadata = pair.Metadata;
                if (!metadata.Disabled)
                {
                    bool validGlobalQuery = string.IsNullOrEmpty(query.ActionKeyword) && pair.Metadata.ActionKeywords[0] == Query.GlobalPluginWildcardSign;
                    bool validNonGlobalQuery = pair.Metadata.ActionKeywords.Contains(query.ActionKeyword);
                    if (validGlobalQuery || validNonGlobalQuery)
                    {
                        List<Result> results = new List<Result>();
                        var milliseconds = Logger.StopWatchDebug($"Query <{query.RawQuery}> Cost for {metadata.Name}", () =>
                        {
                            results = pair.Plugin.Query(query) ?? new List<Result>();
                            UpdatePluginMetadata(results, metadata, query);
                        });
                        metadata.QueryCount += 1;
                        metadata.AvgQueryTime = metadata.QueryCount == 1 ? milliseconds : (metadata.AvgQueryTime + milliseconds) / 2;
                        return results;
                    }
                    else
                    {
                        return new List<Result>();
                    }
                }
                else
                {
                    return new List<Result>();
                }
            }
            catch (Exception e)
            {
                e.Data.Add(nameof(pair.Metadata.ID), pair.Metadata.ID);
                e.Data.Add(nameof(pair.Metadata.Name), pair.Metadata.Name);
                e.Data.Add(nameof(pair.Metadata.PluginDirectory), pair.Metadata.PluginDirectory);
                e.Data.Add(nameof(pair.Metadata.Website), pair.Metadata.Website);
                Logger.WoxError($"Exception for plugin <{pair.Metadata.Name}> when query <{query}>", e);
                return new List<Result>();
            }
        }

        public static void UpdatePluginMetadata(List<Result> results, PluginMetadata metadata, Query query)
        {
            foreach (var r in results)
            {
                r.PluginDirectory = metadata.PluginDirectory;
                r.PluginID = metadata.ID;
                r.OriginQuery = query;

                string key = "EmbededIcon:";
                // todo, use icon path type enum in the future
                if (!string.IsNullOrEmpty(r.PluginDirectory) && !string.IsNullOrEmpty(r.IcoPath) && !Path.IsPathRooted(r.IcoPath) && !r.IcoPath.StartsWith(key))
                {
                    r.IcoPath = Path.Combine(r.PluginDirectory, r.IcoPath);
                }

                // ActionKeywordAssigned is used for constructing MainViewModel's query text auto-complete suggestions 
                // Plugins may have multi-actionkeywords eg. WebSearches. In this scenario it needs to be overriden on the plugin level 
                if (metadata.ActionKeywords.Count == 1)
                    r.ActionKeywordAssigned = query.ActionKeyword;
            }
        }

        private static bool IsGlobalPlugin(PluginMetadata metadata)
        {
            return metadata.ActionKeywords.Contains(Query.GlobalPluginWildcardSign);
        }

        /// <summary>
        /// get specified plugin, return null if not found
        /// </summary>
        /// <param name="id"></param>
        /// <returns></returns>
        public static PluginPair GetPluginForId(string id)
        {
            return AllPlugins.FirstOrDefault(o => o.Metadata.ID == id);
        }

        public static IEnumerable<PluginPair> GetPluginsForInterface<T>() where T : IFeatures
        {
            return AllPlugins.Where(p => p.Plugin is T);
        }

        public static List<Result> GetContextMenusForPlugin(Result result)
        {
            var pluginPair = _contextMenuPlugins.FirstOrDefault(o => o.Metadata.ID == result.PluginID);
            if (pluginPair != null)
            {
                var metadata = pluginPair.Metadata;
                var plugin = (IContextMenu)pluginPair.Plugin;

                try
                {
                    var results = plugin.LoadContextMenus(result);
                    foreach (var r in results)
                    {
                        r.PluginDirectory = metadata.PluginDirectory;
                        r.PluginID = metadata.ID;
                        r.OriginQuery = result.OriginQuery;
                    }
                    return results;
                }
                catch (Exception e)
                {
                    Logger.WoxError($"Can't load context menus for plugin <{metadata.Name}>", e);
                    return new List<Result>();
                }
            }
            else
            {
                return new List<Result>();
            }

        }

        public static bool ActionKeywordRegistered(string actionKeyword)
        {
            if (actionKeyword != Query.GlobalPluginWildcardSign &&
                NonGlobalPlugins.ContainsKey(actionKeyword))
            {
                return true;
            }
            else
            {
                return false;
            }
        }

        /// <summary>
        /// used to add action keyword for multiple action keyword plugin
        /// e.g. web search
        /// </summary>
        public static void AddActionKeyword(string id, string newActionKeyword)
        {
            var plugin = GetPluginForId(id);
            if (newActionKeyword == Query.GlobalPluginWildcardSign)
            {
                GlobalPlugins.Add(plugin);
            }
            else
            {
                NonGlobalPlugins[newActionKeyword] = plugin;
            }
            plugin.Metadata.ActionKeywords.Add(newActionKeyword);
        }

        /// <summary>
        /// used to add action keyword for multiple action keyword plugin
        /// e.g. web search
        /// </summary>
        public static void RemoveActionKeyword(string id, string oldActionkeyword)
        {
            var plugin = GetPluginForId(id);
            if (oldActionkeyword == Query.GlobalPluginWildcardSign
                && // Plugins may have multiple ActionKeywords that are global, eg. WebSearch
                plugin.Metadata.ActionKeywords
                                    .Where(x => x == Query.GlobalPluginWildcardSign)
                                    .ToList()
                                    .Count == 1)
            {
                GlobalPlugins.Remove(plugin);
            }

            if (oldActionkeyword != Query.GlobalPluginWildcardSign)
                NonGlobalPlugins.Remove(oldActionkeyword);


            plugin.Metadata.ActionKeywords.Remove(oldActionkeyword);
        }

        public static void ReplaceActionKeyword(string id, string oldActionKeyword, string newActionKeyword)
        {
            if (oldActionKeyword != newActionKeyword)
            {
                AddActionKeyword(id, newActionKeyword);
                RemoveActionKeyword(id, oldActionKeyword);
            }
        }
    }
}
