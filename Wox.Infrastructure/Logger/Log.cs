using System;
using System.Diagnostics;
using System.IO;
using System.Runtime.CompilerServices;
using NLog;
using NLog.Config;
using NLog.Targets;
using Sentry;
using Wox.Infrastructure.Exception;
using Wox.Infrastructure.UserSettings;

namespace Wox.Infrastructure.Logger
{
    public static class Log
    {
        public const string DirectoryName = "Logs";
        public static string CurrentLogDirectory { get; }

        private static string _woxLanguage = "not set";

        static Log()
        {
            CurrentLogDirectory = Path.Combine(DataLocation.DataDirectory(), DirectoryName, Constant.Version);
            if (!Directory.Exists(CurrentLogDirectory))
            {
                Directory.CreateDirectory(CurrentLogDirectory);
            }

            var configuration = new LoggingConfiguration();
            var fileTarget = new FileTarget()
            {
                FileName = CurrentLogDirectory.Replace(@"\", "/") + "/${shortdate}.txt",
            };
            var consoleTarget = new NLog.Targets.ConsoleTarget();
#if DEBUG
            configuration.AddRule(LogLevel.Debug, LogLevel.Fatal, fileTarget);
#else
            configuration.AddRule(LogLevel.Info, LogLevel.Fatal, fileTarget);
#endif
            LogManager.Configuration = configuration;
        }


        public static void WoxTrace(this NLog.Logger logger, string message, [CallerMemberName] string methodName = "")
        {
            // need change logging manually to see trace log
            if (logger.IsTraceEnabled)
            {
                Debug.WriteLine($"DEBUG|{logger.Name}|{methodName}|{message}");
                logger.Trace($"{methodName}|{message}");
            }

        }

        public static void WoxDebug(this NLog.Logger logger, string message, [CallerMemberName] string methodName = "")
        {
            Debug.WriteLine($"DEBUG|{logger.Name}|{methodName}|{message}");
            logger.Debug($"{methodName}|{message}");
        }


        public static void WoxInfo(this NLog.Logger logger, string message, [CallerMemberName] string methodName = "")
        {
            Debug.WriteLine($"INFO|{logger.Name}|{methodName}|{message}");
            logger.Info($"{methodName}|{message}");
        }

        public static void WoxError(this NLog.Logger logger, string message, [CallerMemberName] string methodName = "")
        {
            Debug.WriteLine($"ERROR|{logger.Name}|{methodName}|{message}");
            logger.Error($"{methodName}|{message}");
        }

        public static void WoxError(
            this NLog.Logger logger, string message, System.Exception exception, bool throwException = true, bool sendException = true, [CallerMemberName] string methodName = "")
        {
            Debug.WriteLine($"ERROR|{logger.Name}|{methodName}|{message}");
            logger.Error($"{methodName}|{message}|{ExceptionFormatter.FormattedException(exception)}");
            if (sendException)
            {
                SendException(exception);
            }

#if DEBUG
            if (throwException)
            {
                throw exception;
            }
#endif
        }

        public static void updateSettingsInfo(string woxLanguage)
        {
            _woxLanguage = woxLanguage;

            SentrySdk.ConfigureScope(scope =>
            {
                scope.SetTag("woxLanguage", _woxLanguage);
            });
        }

        public static void SendException(System.Exception exception)
        {
#if !DEBUG
            string pluginDiretoryKey = nameof(Plugin.PluginPair.Metadata.PluginDirectory);
            if (exception.Data.Contains(pluginDiretoryKey))
            {
                string pluginDirectory = exception.Data[pluginDiretoryKey] as string;
                bool debug = pluginDirectory.Contains(@"\Output\Release") || pluginDirectory.Contains(@"\Output\Release");
                bool thirdParty = !pluginDirectory.Contains(Constant.ProgramDirectory);
                if (debug || thirdParty)
                {
                    return;
                }
            }

            SentrySdk.WithScope(scope =>
            {
                scope.Level = Sentry.Protocol.SentryLevel.Error;
                SentrySdk.CaptureException(exception);
            });
#endif
        }

    }

}