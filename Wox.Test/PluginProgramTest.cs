using System.Collections.Generic;
using System.Linq;

using NUnit.Framework;
using Wox.Core.Plugin;
using Wox.Plugin;

namespace Wox.Test
{
    [TestFixture]
    class PluginProgramTest
    {

        private Plugin.Program.Main plugin;

        [OneTimeSetUp]
        public void Setup()
        {
            plugin = new Plugin.Program.Main();
            plugin.loadSettings();
            Plugin.Program.Main.IndexPrograms();
        }

        [TestCase("powershell", "PowerShell")]
        [TestCase("note", "Notepad")]
        [TestCase("this pc", "This PC")]
        public void Win32Test(string QueryText, string ResultTitle)
        {
            Query query = QueryBuilder.Build(QueryText.Trim(), new Dictionary<string, PluginPair>());
            Result result = plugin.Query(query).OrderByDescending(r => r.Score).First();
            Assert.IsTrue(result.Title.StartsWith(ResultTitle));
        }
    }
}
