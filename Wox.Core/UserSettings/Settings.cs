using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Drawing;
using Newtonsoft.Json;
using Wox.Infrastructure.Hotkey;
using Wox.Plugin;

namespace Wox.Core.UserSettings
{
    [JsonObject(MemberSerialization.OptOut)]
    public class Settings : BaseModel
    {
        public string Language { get; set; } = "en";
        public string Theme { get; set; } = "Dark";
        public string QueryBoxFont { get; set; } = FontFamily.GenericSansSerif.Name;
        public string QueryBoxFontStyle { get; set; }
        public string QueryBoxFontWeight { get; set; }
        public string QueryBoxFontStretch { get; set; }
        public string ResultFont { get; set; } = FontFamily.GenericSansSerif.Name;
        public string ResultFontStyle { get; set; }
        public string ResultFontWeight { get; set; }
        public string ResultFontStretch { get; set; }

        public bool AutoUpdates { get; set; } = true;

        public int MaxResultsToShow { get; set; } = 6;
        public int ActivateTimes { get; set; }

        // Order defaults to 0 or -1, so 1 will let this property appear last
        [JsonProperty(Order = 1)]
        public PluginsSettings PluginSettings { get; set; } = new PluginsSettings();

        public string Hotkey { get; set; } = "Alt + Space";
        public List<CustomHotkey> CustomHotkeys { get; set; } = new List<CustomHotkey>
        {
            new CustomHotkey { Hotkey = "Ctrl+R", Query = ">" }
        };

        public bool StartWoxOnSystemStartup { get; set; } = true;
        public bool HideOnStartup { get; set; }
        public bool HideWhenDeactive { get; set; }
        public bool RememberLastLaunchLocation { get; set; }
        public bool IgnoreHotkeysOnFullscreen { get; set; }

        public string ProxyServer { get; set; }
        public bool ProxyEnabled { get; set; }
        public int ProxyPort { get; set; }
        public string ProxyUserName { get; set; }
        public string ProxyPassword { get; set; }

    }
}