﻿using System;
using System.Diagnostics;
using System.Timers;
using System.Windows;
using Wox.Core;
using Wox.Core.Plugin;
using Wox.Core.Resource;
using Wox.Helper;
using Wox.Infrastructure;
using Wox.Infrastructure.Http;
using Wox.Infrastructure.Image;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.UserSettings;
using Wox.ViewModel;
using Stopwatch = Wox.Infrastructure.Stopwatch;

namespace Wox
{
    public partial class App : IDisposable, ISingleInstanceApp
    {
        public static PublicAPIInstance API { get; private set; }
        private const string Unique = "Wox_Unique_Application_Mutex";
        private static bool _disposed;
        private Settings _settings;

        [STAThread]
        public static void Main()
        {
            RegisterAppDomainExceptions();

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
            Stopwatch.Normal("|App.OnStartup|Startup cost", () =>
            {
                Log.Info("|App.OnStartup|Begin Wox startup ----------------------------------------------------");
                RegisterDispatcherUnhandledException();

                ImageLoader.Initialize();
                Alphabet.Initialize();

                var settingVM = new SettingWindowViewModel();
                _settings = settingVM.Settings;

                PluginManager.LoadPlugins(_settings.PluginSettings);
                var mainVM = new MainViewModel(_settings);
                var window = new MainWindow(_settings, mainVM);
                API = new PublicAPIInstance(settingVM, mainVM);
                PluginManager.InitializePlugins(API);

                Current.MainWindow = window;
                Current.MainWindow.Title = Constant.Wox;

                // happlebao todo temp fix for instance code logic
                // remove all dictionaries defined in xaml e.g.g App.xaml
                Current.Resources.MergedDictionaries.Clear();
                // load plugin before change language, because plugin language also needs be changed
                InternationalizationManager.Instance.Settings = _settings;
                InternationalizationManager.Instance.ChangeLanguage(_settings.Language);
                // main windows needs initialized before theme change because of blur settigns
                ThemeManager.Instance.Settings = _settings;
                ThemeManager.Instance.ChangeTheme(_settings.Theme);

                Http.Proxy = _settings.Proxy;

                RegisterExitEvents();

                AutoStartup();
                AutoUpdates();

                mainVM.MainWindowVisibility = _settings.HideOnStartup ? Visibility.Hidden : Visibility.Visible;
                Log.Info("|App.OnStartup|End Wox startup ----------------------------------------------------  ");
            });
        }


        private void AutoStartup()
        {
            if (_settings.StartWoxOnSystemStartup)
            {
                if (!SettingWindow.StartupSet())
                {
                    SettingWindow.SetStartup();
                }
            }
        }

        private void AutoUpdates()
        {
            if (_settings.AutoUpdates)
            {
                // check udpate every 5 hours
                var timer = new Timer(1000 * 60 * 60 * 5);
                timer.Elapsed += (s, e) =>
                {
                    Updater.UpdateApp();
                };
                timer.Start();

                // check updates on startup
                Updater.UpdateApp();
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

            AppDomain.CurrentDomain.UnhandledException += ErrorReporting.UnhandledExceptionHandle;
            AppDomain.CurrentDomain.FirstChanceException += (s, e) =>
            {
                Log.Exception("|App.RegisterAppDomainExceptions|First Chance Exception:", e.Exception);
            };
        }

        public void Dispose()
        {
            // if sessionending is called, exit proverbially be called when log off / shutdown
            // but if sessionending is not called, exit won't be called when log off / shutdown
            if (!_disposed)
            {
                Current.Dispatcher.Invoke(() => ((MainViewModel)Current.MainWindow?.DataContext)?.Save());

                PluginManager.Save();
                ImageLoader.Save();
                Alphabet.Save();

                _disposed = true;
            }
        }

        public void OnSecondAppStarted()
        {
            Current.MainWindow.Visibility = Visibility.Visible;
        }
    }
}