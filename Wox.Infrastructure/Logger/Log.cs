using System;
using System.Diagnostics;
using System.IO;
using System.Runtime.CompilerServices;
using NLog;
using NLog.Config;
using NLog.Targets;
using Wox.Infrastructure.UserSettings;

namespace Wox.Infrastructure.Logger
{
    public static class Log
    {
        public const string DirectoryName = "Logs";

        public static string CurrentLogDirectory { get; }

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
                FileName = CurrentLogDirectory.Replace(@"\", "/") + "/${shortdate}.txt"
            };
            var consoleTarget = new NLog.Targets.ConsoleTarget();
#if DEBUG
            configuration.AddRule(LogLevel.Debug, LogLevel.Fatal, fileTarget);
#else
            configuration.AddRule(LogLevel.Info, LogLevel.Fatal, fileTarget);
#endif
            LogManager.Configuration = configuration;
        }

        private static void ExceptionInternal(string classAndMethod, string message, System.Exception e)
        {
            var logger = LogManager.GetLogger(classAndMethod);

            System.Diagnostics.Debug.WriteLine($"ERROR|{classAndMethod}|{message}");

            logger.Error("-------------------------- Begin exception --------------------------");
            logger.Error(message);

            do
            {
                logger.Error($"Exception full name:\n <{e.GetType().FullName}>");
                logger.Error($"Exception message:\n <{e.Message}>");
                logger.Error($"Exception stack trace:\n <{e.StackTrace}>");
                logger.Error($"Exception source:\n <{e.Source}>");
                logger.Error($"Exception target site:\n <{e.TargetSite}>");
                logger.Error($"Exception HResult:\n <{e.HResult}>");
                e = e.InnerException;
            } while (e != null);

            logger.Error("-------------------------- End exception --------------------------");
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

        public static void WoxError(this NLog.Logger logger, string message, System.Exception exception, [CallerMemberName] string methodName = "")
        {
#if DEBUG
            throw exception;
#else
            logger.Error(exception, $"{methodName}|{message}");
#endif
        }
    }
}