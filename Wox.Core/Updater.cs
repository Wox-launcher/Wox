﻿using System;
using System.Diagnostics;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Threading.Tasks;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using Squirrel;
using Wox.Core.UserSettings;
using Wox.Infrastructure.Http;
using Wox.Infrastructure.Logger;

namespace Wox.Core
{
    public static class Updater
    {
        [Conditional("RELEASE")]
        public static async void UpdateApp()
        {

            var client = new WebClient {Proxy = Http.WebProxy(HttpProxy.Instance)};
            var downloader = new FileDownloader(client);

            try
            {
                // todo 5/9 the return value of UpdateApp() is NULL, fucking useless!
                using (
                    var updater =
                        await UpdateManager.GitHubUpdateManager(Infrastructure.Constant.Github, urlDownloader: downloader))
                {
                    await updater.UpdateApp();
                }
            }
            catch (HttpRequestException he)
            {
                Log.Exception(he);
            }
            catch (WebException we)
            {
                Log.Exception(we);
            }
            catch (SocketException sc)
            {
                Log.Info("Socket exception happened!, which method cause this exception??");
                Log.Exception(sc);
            }
            catch (Exception exception)
            {
                const string info = "Update.exe not found, not a Squirrel-installed app?";
                if (exception.Message == info)
                {
                    Log.Warn(info);
                }
                else
                {
                    throw;
                }
            }
        }


        public static async Task<string> NewVersion()
        {
            const string githubAPI = @"https://api.github.com/repos/wox-launcher/wox/releases/latest";

            string response;
            try
            {
                response = await Http.Get(githubAPI, HttpProxy.Instance);
            }
            catch (WebException e)
            {
                Log.Warn("Can't connect to github api to check new version");
                Log.Exception(e);
                return string.Empty;
            }

            if (!string.IsNullOrEmpty(response))
            {
                JContainer json;
                try
                {
                    json = (JContainer)JsonConvert.DeserializeObject(response);
                }
                catch (JsonSerializationException e)
                {
                    Log.Exception(e);
                    return string.Empty;
                }
                var version = json?["tag_name"]?.ToString();
                if (!string.IsNullOrEmpty(version))
                {
                    return version;
                }
                else
                {
                    Log.Warn("Can't find tag_name from Github API response");
                    return string.Empty;
                }
            }
            else
            {
                Log.Warn("Can't get response from Github API");
                return string.Empty;
            }
        }

        public static int NumericVersion(string version)
        {
            var newVersion = version.Replace("v", ".").Replace(".", "").Replace("*", "");
            return int.Parse(newVersion);
        }
    }
}
