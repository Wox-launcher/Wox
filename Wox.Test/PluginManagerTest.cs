using System;
using System.Collections.Generic;
using System.Linq;
using NUnit.Framework;
using Wox.Core;
using Wox.Core.Configuration;
using Wox.Core.Plugin;
using Wox.Infrastructure;
using Wox.Image;
using Wox.Infrastructure.UserSettings;
using Wox.Plugin;
using Wox.ViewModel;

namespace Wox.Test
{
    [TestFixture]
    class PluginManagerTest
    {
        [OneTimeSetUp]
        public void setUp()
        {
            // todo remove i18n from application / ui, so it can be tested in a modular way
            new App();
            Settings.Initialize();
            ImageLoader.Initialize();

            Portable portable = new Portable();
            SettingWindowViewModel settingsVm = new SettingWindowViewModel(portable);

            StringMatcher stringMatcher = new StringMatcher();
            StringMatcher.Instance = stringMatcher;
            stringMatcher.UserSettingSearchPrecision = Settings.Instance.QuerySearchPrecision;

            PluginManager.LoadPlugins(Settings.Instance.PluginSettings);
            MainViewModel mainVm = new MainViewModel(false);
            PublicAPIInstance api = new PublicAPIInstance(settingsVm, mainVm);
            PluginManager.InitializePlugins(api);

        }

        [TestCase("setting", "Settings")]
        [TestCase("netwo", "Network and Sharing Center")]
        public void BuiltinQueryTest(string QueryText, string ResultTitle)
        {
            
            Query query = QueryBuilder.Build(QueryText.Trim(), PluginManager.NonGlobalPlugins);
            List<PluginPair> plugins = PluginManager.AllPlugins;
            Result result = plugins.SelectMany(
                    p => PluginManager.QueryForPlugin(p, query)
                )
                .OrderByDescending(r => r.Score)
                .First();

            Assert.IsTrue(result.Title.StartsWith(ResultTitle));
        }
    }
}
