using System.Collections.Generic;
using System.Threading.Tasks;
using Avalonia.Threading;
using SharpHook;
using SharpHook.Native;
using Wox.Uitls;
using Wox.Views;

namespace Wox.ViewModels;

public class MainWindowViewModel : ViewModelBase
{
    private readonly Dictionary<KeyCode, bool> _pressedKeyMap = new();
    private bool _isGlobalRegisterred;
    public CoreQueryViewModel CoreQueryViewModel { get; } = new();

    public void OnDeactivated()
    {
        UIHelper.HideWindow();
    }

    public void StartMonitorGlobalKey()
    {
        if (!_isGlobalRegisterred)
        {
            Task.Run(async () => { await RunGlobalKeyHook(); });
            _isGlobalRegisterred = true;
        }
    }

    private async Task RunGlobalKeyHook()
    {
        var hook = new SimpleGlobalHook();
        hook.KeyPressed += (sender, args) =>
        {
            _pressedKeyMap[args.Data.KeyCode] = true;
            _pressedKeyMap.TryGetValue(KeyCode.VcLeftAlt, out var isLeftAltPressed);
            _pressedKeyMap.TryGetValue(KeyCode.VcLeftMeta, out var isLeftMetaPressed);
            _pressedKeyMap.TryGetValue(KeyCode.VcSpace, out var isSpacePressed);
            if (isLeftAltPressed && isLeftMetaPressed && isSpacePressed) Dispatcher.UIThread.InvokeAsync(UIHelper.ToggleWindowVisible);
        };
        hook.KeyReleased += (sender, args) => { _pressedKeyMap[args.Data.KeyCode] = false; };
        await hook.RunAsync();
    }

    public void InputKeyUp()
    {
        CoreQueryViewModel.ResultListBoxKeyUp();
    }

    public void InputKeyDown()
    {
        CoreQueryViewModel.ResultListBoxKeyDown();
    }

    public void InputKeyEnter()
    {
        CoreQueryViewModel.ResultListBoxKeyEnter();
    }
}