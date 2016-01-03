﻿using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Reflection;
using Wox.Infrastructure.Storage;
using Wox.Plugin;

namespace Wox.Core.UserSettings
{
    public class UserSettingStorage : JsonStrorage<UserSettingStorage>
    {
        [JsonProperty]
        public bool DontPromptUpdateMsg { get; set; }

        [JsonProperty]
        public int ActivateTimes { get; set; }

        [JsonProperty]
        public bool EnableUpdateLog { get; set; }

        [JsonProperty]
        public string Hotkey { get; set; }

        [JsonProperty]
        public string Language { get; set; }

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
        public double WindowLeft { get; set; }

        [JsonProperty]
        public double WindowTop { get; set; }

        public Dictionary<string, CustomizedPluginConfig> CustomizedPluginConfigs { get; set; }

        [JsonProperty]
        public List<CustomPluginHotkey> CustomPluginHotkeys { get; set; }

        [JsonProperty]
        public bool StartWoxOnSystemStartup { get; set; }

        [Obsolete]
        [JsonProperty]
        public double Opacity { get; set; }

        [Obsolete]
        [JsonProperty]
        public OpacityMode OpacityMode { get; set; }

        [JsonProperty]
        public bool LeaveCmdOpen { get; set; }

        [JsonProperty]
        public bool HideWhenDeactive { get; set; }

        [JsonProperty]
        public bool RememberLastLaunchLocation { get; set; }

        [JsonProperty]
        public bool IgnoreHotkeysOnFullscreen { get; set; }

        [JsonProperty]
        public string ProxyServer { get; set; }

        [JsonProperty]
        public bool ProxyEnabled { get; set; }

        [JsonProperty]
        public int ProxyPort { get; set; }

        [JsonProperty]
        public string ProxyUserName { get; set; }

        [JsonProperty]
        public string ProxyPassword { get; set; }

        [JsonProperty]
        public int MaxResultsToShow { get; set; }

        protected override string ConfigFolder
        {
            get { return Path.Combine(Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location), "Config"); }
        }

        protected override string ConfigName
        {
            get { return "config"; }
        }

        public void IncreaseActivateTimes()
        {
            ActivateTimes++;
            if (ActivateTimes % 15 == 0)
            {
                Save();
            }
        }

        protected override UserSettingStorage LoadDefault()
        {
            DontPromptUpdateMsg = false;
            Theme = "Dark";
            Language = "en";
            CustomizedPluginConfigs = new Dictionary<string, CustomizedPluginConfig>();
            Hotkey = "Alt + Space";
            QueryBoxFont = FontFamily.GenericSansSerif.Name;
            ResultItemFont = FontFamily.GenericSansSerif.Name;
            Opacity = 1;
            OpacityMode = OpacityMode.Normal;
            LeaveCmdOpen = false;
            HideWhenDeactive = false;
            CustomPluginHotkeys = new List<CustomPluginHotkey>();
            RememberLastLaunchLocation = false;
            MaxResultsToShow = 6;
            return this;
        }

        protected override void OnAfterLoad(UserSettingStorage storage)
        {
            if (storage.CustomizedPluginConfigs == null)
            {
                storage.CustomizedPluginConfigs = new Dictionary<string, CustomizedPluginConfig>();
            }
            if (storage.QueryBoxFont == null)
            {
                storage.QueryBoxFont = FontFamily.GenericSansSerif.Name;
            }
            if (storage.ResultItemFont == null)
            {
                storage.ResultItemFont = FontFamily.GenericSansSerif.Name;
            }
            if (storage.Language == null)
            {
                storage.Language = "en";
            }
        }

        public void UpdateActionKeyword(PluginMetadata metadata)
        {
            if (CustomizedPluginConfigs.ContainsKey(metadata.ID))
            {
                var customizedPluginConfig = CustomizedPluginConfigs[metadata.ID];
                customizedPluginConfig.ActionKeywords = metadata.ActionKeywords;
            }
            else
            {
                var customizedPluginConfig = new CustomizedPluginConfig()
                {
                    Disabled = false,
                    ID = metadata.ID,
                    Name = metadata.Name,
                    ActionKeywords = metadata.ActionKeywords
                };

                CustomizedPluginConfigs.Add(metadata.ID, customizedPluginConfig);
            }
            Save();
        }
    }

    public enum OpacityMode
    {
        Normal = 0,
        LayeredWindow = 1,
        DWM = 2
    }
}