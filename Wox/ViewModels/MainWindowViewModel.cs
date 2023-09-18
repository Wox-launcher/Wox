using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Threading;
using SharpHook;
using SharpHook.Native;

namespace Wox.ViewModels;

public class MainWindowViewModel : ViewModelBase
{
    private Boolean isGlobalRegisterred = false;
    public CoreQueryViewModel CoreQueryViewModel { get; } = new CoreQueryViewModel();

    private Dictionary<KeyCode,Boolean> pressedKeyMap = new Dictionary<KeyCode,Boolean>();

    public void OnDeactivated()
    {
        if (Application.Current != null && Application.Current.ApplicationLifetime != null)
        {
            var woxMainWindow = ((IClassicDesktopStyleApplicationLifetime)Application.Current.ApplicationLifetime)
                .MainWindow;
            if (woxMainWindow != null) woxMainWindow.Hide();
        }
    }
    
    public void ToggleWindowVisible()
    {
        if (Application.Current != null && Application.Current.ApplicationLifetime != null)
        {
            var woxMainWindow = ((IClassicDesktopStyleApplicationLifetime)Application.Current.ApplicationLifetime)
                .MainWindow;
            if (woxMainWindow != null) woxMainWindow.IsVisible=!woxMainWindow.IsVisible;
        }
    }
    
    public void StartMonitorGlobalKey()
    {
        if (!isGlobalRegisterred)
        {
            Task.Run(async () => { await RunGlobalKeyHook(); });
            isGlobalRegisterred = true;
        }
    }
    
    private async Task RunGlobalKeyHook()
    {
        var hook = new SimpleGlobalHook();
        hook.KeyPressed+= (((sender, args) =>
        {
            pressedKeyMap[args.Data.KeyCode] = true;
            pressedKeyMap.TryGetValue(KeyCode.VcLeftAlt, out var isLeftAltPressed);
            pressedKeyMap.TryGetValue(KeyCode.VcLeftMeta, out var isLeftMetaPressed);
            pressedKeyMap.TryGetValue(KeyCode.VcSpace, out var isSpacePressed);
            if (isLeftAltPressed && isLeftMetaPressed && isSpacePressed) 
            {
                Dispatcher.UIThread.InvokeAsync(ToggleWindowVisible);
            }
        }));
        hook.KeyReleased+=((sender,args) =>
        {
            pressedKeyMap[args.Data.KeyCode] = false;
        });
        await hook.RunAsync();
    }
}