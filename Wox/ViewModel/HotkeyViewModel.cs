using Wox.Core.Resource;
using Wox.Infrastructure;
using Wox.Infrastructure.Hotkey;
using Wox.Plugin;

namespace Wox.ViewModel
{
    public class HotkeyViewModel : BaseModel
    {
        private readonly GlobalHotkey _globalHotkey = GlobalHotkey.Instance;
        private readonly Internationalization _translater = InternationalizationManager.Instance;
        public HotkeyModel Hotkey { get; set; } = new HotkeyModel();
        public string Text => Hotkey.ToString();

        public void OnHotkeyCaptured(object sender, GlobalHotkey.HotkeyCapturedEventArgs e)
        {
            if (e.Available)               
            {
                _globalHotkey.RemoveHotkey(Hotkey);
                Hotkey = e.Hotkey;
                var message = _translater.GetTranslation("succeed");
                App.API.ShowMsg(message, iconPath: Constant.AppIcon);
            }
            else
            {
                var message = _translater.GetTranslation("hotkeyUnavailable");
                App.API.ShowMsg(message, iconPath: Constant.ErrorIcon);
            }
        }
    }
}
