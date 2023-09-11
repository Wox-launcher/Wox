using System;
using System.Threading.Tasks;
using Avalonia;
using Avalonia.ReactiveUI;
using Wox.Core.Utils;

namespace Wox;

internal class Program
{
    // Initialization code. Don't use any Avalonia, third-party APIs or any
    // SynchronizationContext-reliant code before AppMain is called: things aren't initialized
    // yet and stuff might break.
    [STAThread]
    public static void Main(string[] args)
    {
        try
        {
            TaskScheduler.UnobservedTaskException += (sender, e) =>
            {
                Logger.Error("Caught UnobservedTaskException", e.Exception);
                e.SetObserved();
            };
            BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);
        }
        catch (Exception e)
        {
            // here we can work with the exception, for example add it to our log file
            Logger.Error("Failed to start Wox", e);
            throw;
        }
    }

    public static AppBuilder BuildAvaloniaApp()
    {
        return AppBuilder.Configure<App>()
            .UsePlatformDetect()
            .WithInterFont()
            .LogToTrace()
            .UseReactiveUI();
    }
}