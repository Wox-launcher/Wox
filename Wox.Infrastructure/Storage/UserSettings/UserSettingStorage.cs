﻿using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Windows.Forms;
using Newtonsoft.Json;
using Wox.Core.Data;

namespace Wox.Infrastructure.Storage.UserSettings
{
    public class UserSettingStorage : BaseStorage<UserSettingStorage>
    {
        [JsonProperty]
        public string Hotkey { get; set; }

        [JsonProperty]
        public string Theme { get; set; }

        [JsonProperty]
        public string QueryBoxFont { get; set; }

        [JsonProperty]
        public string QueryBoxFontStyle { get; set; }

        [JsonProperty]
        public string QueryBoxFontWeight { get; set; }

        [JsonProperty]
        public string QueryBoxFontStretch { get; set; }

        [JsonProperty]
        public string ResultItemFont { get; set; }

        [JsonProperty]
        public string ResultItemFontStyle { get; set; }

        [JsonProperty]
        public string ResultItemFontWeight { get; set; }

        [JsonProperty]
        public string ResultItemFontStretch { get; set; }

        [JsonProperty]
        public bool ReplaceWinR { get; set; }

        [JsonProperty]
        public List<WebSearch> WebSearches { get; set; }

        [JsonProperty]
        public List<ProgramSource> ProgramSources { get; set; }

		[JsonProperty]
		public List<FolderLink> FolderLinks { get; set; }	//Aaron

        [JsonProperty]
        public List<CustomPluginHotkey> CustomPluginHotkeys { get; set; }

        [JsonProperty]
        public bool StartWoxOnSystemStartup { get; set; }

        [JsonProperty]
        public bool EnablePythonPlugins { get; set; }

		[JsonProperty]
		public bool EnableBookmarkPlugin { get; set; }

        [JsonProperty]
        public double Opacity { get; set; }

        [JsonProperty]
        public OpacityMode OpacityMode { get; set; }

		[JsonProperty]
		public bool LeaveCmdOpen { get; set; }

        [JsonProperty]
        public bool HideWhenDeactive { get; set; }

        public List<WebSearch> LoadDefaultWebSearches()
        {
            List<WebSearch> webSearches = new List<WebSearch>();

            WebSearch googleWebSearch = new WebSearch()
            {
                Title = "Google",
                ActionWord = "g",
                IconPath = Path.GetDirectoryName(Application.ExecutablePath) + @"\Images\websearch\google.png",
                Url = "https://www.google.com/search?q={q}",
                Enabled = true
            };
            webSearches.Add(googleWebSearch);


            WebSearch wikiWebSearch = new WebSearch()
            {
                Title = "Wikipedia",
                ActionWord = "wiki",
                IconPath = Path.GetDirectoryName(Application.ExecutablePath) + @"\Images\websearch\wiki.png",
                Url = "http://en.wikipedia.org/wiki/{q}",
                Enabled = true
            };
            webSearches.Add(wikiWebSearch);

            return webSearches;
        }

        public List<ProgramSource> LoadDefaultProgramSources()
        {
            var list = new List<ProgramSource>();
            list.Add(new ProgramSource()
            {
                BonusPoints = 0,
                Enabled = true,
                Type = "CommonStartMenuProgramSource"
            });
            list.Add(new ProgramSource()
            {
                BonusPoints = 0,
                Enabled = true,
                Type = "UserStartMenuProgramSource"
            });
            list.Add(new ProgramSource()
            {
                BonusPoints = -10,
                Enabled = true,
                Type = "AppPathsProgramSource"
            });
            return list;
        }

        protected override string ConfigName
        {
            get { return "config"; }
        }

        protected override void LoadDefaultConfig()
        {
            EnablePythonPlugins = true;
			EnableBookmarkPlugin = true;
            Theme = "Dark";
            ReplaceWinR = true;
            WebSearches = LoadDefaultWebSearches();
            ProgramSources = LoadDefaultProgramSources();
            Hotkey = "Alt + Space";
            QueryBoxFont = FontFamily.GenericSansSerif.Name;
            ResultItemFont = FontFamily.GenericSansSerif.Name;
            Opacity = 1;
	        OpacityMode = OpacityMode.Normal;
	        LeaveCmdOpen = false;
            HideWhenDeactive = false;
        }
    }
}
