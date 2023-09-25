using System.Collections.Generic;
using System.Threading.Tasks;
using Avalonia;
using Avalonia.Controls;
using Avalonia.Threading;
using SharpHook;
using SharpHook.Native;
using Wox.Utils;

namespace Wox.ViewModels;

public class MainWindowViewModel : ViewModelBase
{
    private readonly Dictionary<KeyCode, bool> _pressedKeyMap = new();
    private PixelPoint _currentPixelPoint = new(0, 0);
    private bool _isGlobalRegisterred;
    private Screens? _screens;
    public CoreQueryViewModel CoreQueryViewModel { get; } = new();

    public void OnDeactivated()
    {
        UiHelper.HideWindow();
    }

    public void StartMonitorGlobalKey(Screens? screens)
    {
        if (!_isGlobalRegisterred)
        {
            _isGlobalRegisterred = true;
            Task.Run(async () => { await RunGlobalKeyHook(); });
            _screens = screens;
        }
    }

    private async Task RunGlobalKeyHook()
    {
        var hook = new SimpleGlobalHook();
        //Monitor Key Event
        hook.KeyPressed += (sender, args) =>
        {
            _pressedKeyMap[args.Data.KeyCode] = true;
            _pressedKeyMap.TryGetValue(KeyCode.VcLeftMeta, out var isLeftMetaPressed);
            _pressedKeyMap.TryGetValue(KeyCode.VcSpace, out var isSpacePressed);
            if (isLeftMetaPressed && isSpacePressed)
            {
                var currentScreen = _screens?.ScreenFromPoint(_currentPixelPoint);
                Dispatcher.UIThread.InvokeAsync(() => { UiHelper.ToggleWindowVisible(currentScreen); });
            }
        };
        hook.KeyReleased += (sender, args) => { _pressedKeyMap[args.Data.KeyCode] = false; };
        //Monitor Mouse Event
        hook.MouseMoved += (sender, args) => { _currentPixelPoint = new PixelPoint(args.Data.X, args.Data.Y); };
        await hook.RunAsync();
    }

    public void KeyUp()
    {
        CoreQueryViewModel.MoveUpListBoxSelectedIndex();
    }

    public void KeyDown()
    {
        CoreQueryViewModel.MoveDownListBoxSelectedIndex();
    }

    public void KeyEnter()
    {
        CoreQueryViewModel.AsyncOpenResultAction();
    }
}