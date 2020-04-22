using System;
using System.Collections.Generic;
using System.Linq;
using NUnit.Framework;
using Wox.Core;
using Wox.Core.Configuration;
using Wox.Core.Plugin;
using Wox.Infrastructure;
using Wox.Infrastructure.Image;
using Wox.Infrastructure.UserSettings;
using Wox.Plugin;
using Wox.ViewModel;

namespace Wox.Test
{
    [TestFixture]
    class PluginManagerTest
    {
        [Test]
        public void ProgramPluginTest()
        {
            String QueryText = "PowerShell";
            String ResultTitle = "PowerShell";

            // todo remove i18n from application / ui, so it can be tested in a modular way
            var application = new App();

            Constant.Initialize();
            ImageLoader.Initialize();

            Updater updater = new Updater("");
            Portable portable = new Portable();
            SettingWindowViewModel settingsVm = new SettingWindowViewModel(updater, portable);
            Settings settings = settingsVm.Settings;

            Alphabet alphabet = new Alphabet();
            alphabet.Initialize(settings);
            StringMatcher stringMatcher = new StringMatcher(alphabet);
            StringMatcher.Instance = stringMatcher;
            stringMatcher.UserSettingSearchPrecision = settings.QuerySearchPrecision;

            PluginManager.LoadPlugins(settings.PluginSettings);
            MainViewModel mainVm = new MainViewModel(settings, false);
            PublicAPIInstance api = new PublicAPIInstance(settingsVm, mainVm, alphabet);
            PluginManager.InitializePlugins(api);

            Query query = QueryBuilder.Build(QueryText.Trim(), PluginManager.NonGlobalPlugins);
            List<PluginPair> plugins = PluginManager.ValidPluginsForQuery(query);
            var results = plugins.SelectMany(
                    p => PluginManager.QueryForPlugin(p, query)
                )
                .OrderByDescending(r => r.Score)
                .Take(settings.MaxResultsToShow) // check title within the first page
                .Any(r => r.Title == ResultTitle);
        }
    }
}
