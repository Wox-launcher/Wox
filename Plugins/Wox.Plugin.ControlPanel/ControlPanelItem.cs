using System.Diagnostics;
using System.Drawing;

namespace Wox.Plugin.ControlPanel
{
    //from:https://raw.githubusercontent.com/CoenraadS/Windows-Control-Panel-Items
    public class ControlPanelItem
    {
        public string LocalizedString { get; private set; }
        public string InfoTip { get; private set; }
        public string GUID { get; private set; }
        public ProcessStartInfo ExecutablePath { get; private set; }
        public string IconPath { get; private set; }
        public int Score { get; set; }

        public ControlPanelItem(string newLocalizedString, string newInfoTip, string newGUID, ProcessStartInfo newExecutablePath, string iconPath)
        {
            LocalizedString = newLocalizedString;
            InfoTip = newInfoTip;
            ExecutablePath = newExecutablePath;
            GUID = newGUID;
            string key = "EmbededIcon:";
            IconPath = $"{key}{iconPath}";
        }
    }
}