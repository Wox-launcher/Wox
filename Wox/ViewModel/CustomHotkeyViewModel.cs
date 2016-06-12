using Wox.Plugin;
using Wox.Infrastructure.Hotkey;

namespace Wox.ViewModel
{
    public class CustomHotkeyViewModel : BaseModel
    {
        public CustomHotkeyModel CustomHotkey { get; set; } = new CustomHotkeyModel();
        public HotkeyViewModel HotkeyViewModel { get; set; } = new HotkeyViewModel();

        public CustomHotkeyViewModel()
        {
            HotkeyViewModel.PropertyChanged += (s, e) =>
            {
                if (e.PropertyName == nameof(ViewModel.HotkeyViewModel.Hotkey))
                {
                    CustomHotkey.Hotkey = HotkeyViewModel.Hotkey;
                    OnPropertyChanged(nameof(CustomHotkey));
                }
            };
        }
    }
}
