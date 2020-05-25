using System.Collections.Generic;
using System.Linq;
using System.Windows.Controls;
using Wox.Infrastructure.Storage;
using Wox.Plugin.BrowserBookmark.Commands;
using Wox.Plugin.BrowserBookmark.Models;
using Wox.Plugin.BrowserBookmark.Views;
using Wox.Infrastructure;
using System.Threading.Tasks;

namespace Wox.Plugin.BrowserBookmark
{
    public class Main : ISettingProvider, IPlugin, IReloadable, IPluginI18n, ISavable
    {
        private PluginInitContext context;

        private List<Bookmark> cachedBookmarks = new List<Bookmark>();
        private object _updateLock = new object();

        private readonly Settings _settings;
        private readonly PluginJsonStorage<Settings> _storage;


        public Main()
        {
            _storage = new PluginJsonStorage<Settings>();
            _settings = _storage.Load();

            //TODO: Let the user select which browser's bookmarks are displayed
            var chromeBookmarks = new ChromeBookmarks().GetBookmarks().Distinct().ToList();
            lock (_updateLock)
            {
                cachedBookmarks = chromeBookmarks;
            }
            Task.Run(() =>
            {
                // firefox bookmarks is slow, since it nened open sqlite connection.
                // use lazy load
                var mozBookmarks = new FirefoxBookmarks().GetBookmarks();
                var cached = mozBookmarks.Concat(cachedBookmarks).Distinct().ToList();
                lock (_updateLock)
                {
                    cachedBookmarks = cached;
                }
            });

        }

        public void Init(PluginInitContext context)
        {
            this.context = context;
        }

        public List<Result> Query(Query query)
        {
            string param = query.Search.TrimStart();

            // Should top results be returned? (true if no search parameters have been passed)
            var topResults = string.IsNullOrEmpty(param);

            lock (_updateLock)
            {

                var returnList = cachedBookmarks;

                if (!topResults)
                {
                    // Since we mixed chrome and firefox bookmarks, we should order them again                
                    returnList = cachedBookmarks.Where(o => Bookmarks.MatchProgram(o, param)).ToList();
                    returnList = returnList.OrderByDescending(o => o.Score).ToList();
                }

                var results = returnList.Select(c => new Result()
                {
                    Title = c.Name,
                    SubTitle = c.Url,
                    PluginDirectory = context.CurrentPluginMetadata.PluginDirectory,
                    IcoPath = @"Images\bookmark.png",
                    Score = 5,
                    Action = (e) =>
                    {
                        if (_settings.OpenInNewBrowserWindow)
                        {
                            c.Url.NewBrowserWindow(_settings.BrowserPath);
                        }
                        else
                        {
                            c.Url.NewTabInBrowser(_settings.BrowserPath);
                        }

                        return true;
                    }
                }).ToList();
                return results;
            }

        }

        public void ReloadData()
        {
            //TODO: Let the user select which browser's bookmarks are displayed
            var chromeBookmarks = new ChromeBookmarks();
            var mozBookmarks = new FirefoxBookmarks();
            var b1 = mozBookmarks.GetBookmarks();
            var b2 = chromeBookmarks.GetBookmarks();
            b1.AddRange(b2);
            var cached = b1.Distinct().ToList();
            lock (_updateLock)
            {
                cachedBookmarks = cached;
            }
        }

        public string GetTranslatedPluginTitle()
        {
            return context.API.GetTranslation("wox_plugin_browserbookmark_plugin_name");
        }

        public string GetTranslatedPluginDescription()
        {
            return context.API.GetTranslation("wox_plugin_browserbookmark_plugin_description");
        }

        public Control CreateSettingPanel()
        {
            return new SettingsControl(_settings);
        }

        public void Save()
        {
            _storage.Save();
        }
    }
}
