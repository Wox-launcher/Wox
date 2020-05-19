using System;
using System.Collections.Generic;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using JetBrains.Annotations;
using Squirrel;
using Newtonsoft.Json;
using Wox.Core.Resource;
using Wox.Infrastructure;
using Wox.Infrastructure.Http;
using Wox.Infrastructure.Logger;
using System.IO;
using NLog;
using Wox.Infrastructure.UserSettings;

namespace Wox.Core
{
    public class Updater
    {
        public string GitHubRepository { get; }
        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

        public Updater(string gitHubRepository)
        {
            GitHubRepository = gitHubRepository;
        }

        public async Task UpdateApp(bool silentIfLatestVersion = true, bool updateToPrereleases = false)
        {
            try
            {
                using (UpdateManager updateManager = await GitHubUpdateManager(GitHubRepository, updateToPrereleases))
                {
                    UpdateInfo newUpdateInfo;
                    try
                    {
                        // UpdateApp CheckForUpdate will return value only if the app is squirrel installed
                        newUpdateInfo = await updateManager.CheckForUpdate().NonNull();
                    }
                    catch (Exception e) when (e is HttpRequestException || e is WebException || e is SocketException)
                    {
                        Logger.WoxError($"Check your connection and proxy settings to api.github.com. {e.Message}");
                        updateManager.Dispose();
                        return;
                    }

                    var newReleaseVersion = Version.Parse(newUpdateInfo.FutureReleaseEntry.Version.ToString());
                    var currentVersion = Version.Parse(Constant.Version);

                    Logger.WoxInfo($"Future Release <{newUpdateInfo.FutureReleaseEntry.Formatted()}>");

                    if (newReleaseVersion <= currentVersion)
                    {
                        if (!silentIfLatestVersion)
                            MessageBox.Show("You already have the latest Wox version");
                        updateManager.Dispose();
                        return;
                    }

                    try
                    {
                        await updateManager.DownloadReleases(newUpdateInfo.ReleasesToApply);
                    }
                    catch (Exception e) when (e is HttpRequestException || e is WebException || e is SocketException)
                    {
                        Logger.WoxError($"Check your connection and proxy settings to github-cloud.s3.amazonaws.com. {e.Message}");
                        updateManager.Dispose();
                        return;
                    }

                    await updateManager.ApplyReleases(newUpdateInfo);

                    await updateManager.CreateUninstallerRegistryEntry();

                    var newVersionTips = NewVersinoTips(newReleaseVersion.ToString());

                    MessageBox.Show(newVersionTips);
                    Logger.WoxInfo($"Update success:{newVersionTips}");


                }
            }
            catch (Exception e) when (e is HttpRequestException || e is WebException || e is SocketException)
            {
                Logger.WoxError($"Please check your connection and proxy settings {e.Message}");
            }
            catch (Exception e)
            {
                Logger.WoxError($"cannot check update {e.Message}");
            }

        }

        [UsedImplicitly]
        private class GithubRelease
        {
            [JsonProperty("prerelease")]
            public bool Prerelease { get; [UsedImplicitly] set; }

            [JsonProperty("published_at")]
            public DateTime PublishedAt { get; [UsedImplicitly] set; }

            [JsonProperty("html_url")]
            public string HtmlUrl { get; [UsedImplicitly] set; }
        }

        /// https://github.com/Squirrel/Squirrel.Windows/blob/master/src/Squirrel/UpdateManager.Factory.cs
        private async Task<UpdateManager> GitHubUpdateManager(string repository, bool updateToPrereleases)
        {
            var uri = new Uri(repository);
            var api = $"https://api.github.com/repos{uri.AbsolutePath}/releases";

            var json = await Http.Get(api);

            var releases = JsonConvert.DeserializeObject<List<GithubRelease>>(json).AsEnumerable();
            if (!updateToPrereleases)
            {
                releases = releases.Where(r => !r.Prerelease);
            }
            var latest = releases.OrderByDescending(r => r.PublishedAt).First();

            var latestUrl = latest.HtmlUrl.Replace("/tag/", "/download/");

            var client = new WebClient { Proxy = Http.WebProxy() };
            var downloader = new FileDownloader(client);

            var manager = new UpdateManager(latestUrl, urlDownloader: downloader);

            return manager;
        }

        public string NewVersinoTips(string version)
        {
            var translater = InternationalizationManager.Instance;
            var tips = string.Format(translater.GetTranslation("newVersionTips"), version);
            return tips;
        }

    }
}
