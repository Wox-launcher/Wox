using System;
using System.Diagnostics;
using System.Threading.Tasks;
using System.Windows;
using System.Collections.Generic;
using System.Threading;
using System.Globalization;

using CommandLine;
using NLog;

using Wox.Core;
using Wox.Core.Configuration;
using Wox.Core.Plugin;
using Wox.Core.Resource;
using Wox.Helper;
using Wox.Infrastructure;
using Wox.Infrastructure.Http;
using Wox.Image;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.UserSettings;
using Wox.ViewModel;
using Stopwatch = Wox.Infrastructure.Stopwatch;
using Wox.Infrastructure.Exception;

namespace Wox
{
    public partial class App : IDisposable, ISingleInstanceApp
    {
        public static PublicAPIInstance API { get; private set; }
        private const string Unique = "Wox_Unique_Application_Mutex";
        private static bool _disposed;
        private MainViewModel _mainVM;
        private SettingWindowViewModel _settingsVM;
        private readonly Portable _portable = new Portable();
        private StringMatcher _stringMatcher;

        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

        private class Options
        {
            [Option('q', "query", Required = false, HelpText = "Specify text to query on startup.")]
            public string QueryText { get; set; }
        }

        private void ParseCommandLineArgs(IList<string> args)
        {
            if (args == null)
                return;

            Parser.Default.ParseArguments<Options>(args)
                .WithParsed(o =>
                {
                    if (o.QueryText != null && _mainVM != null)
                        _mainVM.ChangeQueryText(o.QueryText);
                });
        }

        [STAThread]
        public static void Main()
        {
            Thread.CurrentThread.CurrentCulture = CultureInfo.InvariantCulture;
            Thread.CurrentThread.CurrentUICulture = CultureInfo.InvariantCulture;
            if (SingleInstance<App>.InitializeAsFirstInstance(Unique))
            {
                using (var application = new App())
                {
                    application.InitializeComponent();
                    application.Run();
                }
            }
        }

        private void OnStartup(object sender, StartupEventArgs e)
        {
            Logger.StopWatchNormal("Startup cost", () =>
            {
                RegisterAppDomainExceptions();
                RegisterDispatcherUnhandledException();

                Logger.WoxInfo("Begin Wox startup----------------------------------------------------");
                Settings.Initialize();
                Logger.WoxInfo(ExceptionFormatter.RuntimeInfo());

                _portable.PreStartCleanUpAfterPortabilityUpdate();

                ImageLoader.Initialize();

                _settingsVM = new SettingWindowViewModel(_portable);

                _stringMatcher = new StringMatcher();
                StringMatcher.Instance = _stringMatcher;
                _stringMatcher.UserSettingSearchPrecision = Settings.Instance.QuerySearchPrecision;

                PluginManager.LoadPlugins(Settings.Instance.PluginSettings);
                _mainVM = new MainViewModel();
                var window = new MainWindow(_mainVM);
                API = new PublicAPIInstance(_settingsVM, _mainVM);
                PluginManager.InitializePlugins(API);

                Current.MainWindow = window;
                Current.MainWindow.Title = Constant.Wox;

                // todo temp fix for instance code logic
                // load plugin before change language, because plugin language also needs be changed
                InternationalizationManager.Instance.Settings = Settings.Instance;
                InternationalizationManager.Instance.ChangeLanguage(Settings.Instance.Language);
                // main windows needs initialized before theme change because of blur settigns
                ThemeManager.Instance.ChangeTheme(Settings.Instance.Theme);

                Http.Proxy = Settings.Instance.Proxy;

                RegisterExitEvents();

                AutoStartup();

                ParseCommandLineArgs(SingleInstance<App>.CommandLineArgs);
                _mainVM.MainWindowVisibility = Settings.Instance.HideOnStartup ? Visibility.Hidden : Visibility.Visible;

                Logger.WoxInfo($"SDK Info: {ExceptionFormatter.SDKInfo()}");
                Logger.WoxInfo("End Wox startup ----------------------------------------------------  ");
            });
        }


        private void AutoStartup()
        {
            if (Settings.Instance.StartWoxOnSystemStartup)
            {
                if (!SettingWindow.StartupSet())
                {
                    SettingWindow.SetStartup();
                }
            }
        }

        private void RegisterExitEvents()
        {
            AppDomain.CurrentDomain.ProcessExit += (s, e) => Dispose();
            Current.Exit += (s, e) => Dispose();
            Current.SessionEnding += (s, e) => Dispose();
        }

        /// <summary>
        /// let exception throw as normal is better for Debug
        /// </summary>
        [Conditional("RELEASE")]
        private void RegisterDispatcherUnhandledException()
        {
            DispatcherUnhandledException += ErrorReporting.DispatcherUnhandledException;
        }


        /// <summary>
        /// let exception throw as normal is better for Debug
        /// </summary>
        [Conditional("RELEASE")]
        private static void RegisterAppDomainExceptions()
        {
            AppDomain.CurrentDomain.UnhandledException += ErrorReporting.UnhandledExceptionHandleMain;
        }

        public void Dispose()
        {
            // if sessionending is called, exit proverbially be called when log off / shutdown
            // but if sessionending is not called, exit won't be called when log off / shutdown
            if (!_disposed)
            {
                API.SaveAppAllSettings();
                _disposed = true;
            }
        }

        public void OnSecondAppStarted(IList<string> args)
        {
            ParseCommandLineArgs(args);
            Current.MainWindow.Visibility = Visibility.Visible;
        }
    }
}