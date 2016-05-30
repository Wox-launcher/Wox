
using Wox.Plugin;

namespace Wox.Infrastructure.Hotkey
{
    public class CustomHotkeyModel : BaseModel
    {
        public HotkeyModel Hotkey { get; set; } = new HotkeyModel();
        public string Query { get; set; } = string.Empty;
    }
}
