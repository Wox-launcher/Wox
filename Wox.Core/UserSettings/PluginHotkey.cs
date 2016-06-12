using Wox.Plugin;

namespace Wox.Core.UserSettings
{
    public class CustomHotkey : BaseModel
    {
        public string Hotkey { get; set; }
        public string Query { get; set; }
    }
}
