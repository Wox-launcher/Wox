using System.Windows.Input;
using Wox.Infrastructure.Hotkey;
using Wox.ViewModel;

namespace Wox
{
    public partial class HotkeyControl
    {
        public HotkeyControl()
        {
            InitializeComponent();
        }

        private void StartCapturing(object sender, KeyboardFocusChangedEventArgs e)
        {
            var globalHotkey = GlobalHotkey.Instance;
            globalHotkey.Capturing = true;
            var vm = (HotkeyViewModel) DataContext;
            // ensure there is only one event subscriber
            globalHotkey.HotkeyCaptured -= vm.OnHotkeyCaptured;
            globalHotkey.HotkeyCaptured += vm.OnHotkeyCaptured;
        }

        private void StopCapturing(object sender, KeyboardFocusChangedEventArgs e)
        {
            var globalHotkey = GlobalHotkey.Instance;
            globalHotkey.Capturing = false;
            var vm = (HotkeyViewModel)DataContext;
            globalHotkey.HotkeyCaptured -= vm.OnHotkeyCaptured;
        }
    }
}
