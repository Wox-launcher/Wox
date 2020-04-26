using NLog;
using NLog.Config;
using NLog.Targets;
using System;
using System.Diagnostics;
using System.IO;
using System.Runtime.CompilerServices;
using System.Security;
using Wox.Infrastructure;
using Wox.Infrastructure.UserSettings;

namespace Wox.Plugin.Program.Logger
{
    /// <summary>
    /// The Program plugin has seen many issues recorded in the Wox repo related to various loading of Windows programs.
    /// This is a dedicated logger for this Program plugin with the aim to output a more friendlier message and clearer
    /// log that will allow debugging to be quicker and easier.
    /// </summary>
    internal static class ProgramLogger
    {

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        /// <summary>
        /// Logs an exception
        /// </summary>
        internal static void LogException(string classname, string callingMethodName, string loadingProgramPath,
            string interpretationMessage, Exception e)
        {
            Debug.WriteLine($"ERROR|{classname}|{callingMethodName}|{loadingProgramPath}|{interpretationMessage}");

            var innerExceptionNumber = 1;

            var possibleResolution = "Not yet known";
            var errorStatus = "UNKNOWN";

            Logger.Error("------------- BEGIN Wox.Plugin.Program exception -------------");

            do
            {
                var calledMethod = e.TargetSite != null ? e.TargetSite.ToString() : e.StackTrace;

                calledMethod = string.IsNullOrEmpty(calledMethod) ? "Not available" : calledMethod;

                Logger.Error($"\nException full name: {e.GetType().FullName}"
                             + $"\nError status: {errorStatus}"
                             + $"\nClass name: {classname}"
                             + $"\nCalling method: {callingMethodName}"
                             + $"\nProgram path: {loadingProgramPath}"
                             + $"\nInnerException number: {innerExceptionNumber}"
                             + $"\nException message: {e.Message}"
                             + $"\nException error type: HResult {e.HResult}"
                             + $"\nException thrown in called method: {calledMethod}"
                             + $"\nPossible interpretation of the error: {interpretationMessage}"
                             + $"\nPossible resolution: {possibleResolution}");

                innerExceptionNumber++;
                e = e.InnerException;
            } while (e != null);

            Logger.Error("------------- END Wox.Plugin.Program exception -------------");
        }

        /// <summary>
        /// Please follow exception format: |class name|calling method name|loading program path|user friendly message that explains the error
        /// => Example: |Win32|LnkProgram|c:\..\chrome.exe|Permission denied on directory, but Wox should continue
        /// </summary>
        internal static void LogException(string message, Exception e)
        {
            //Index 0 is always empty.
            var parts = message.Split('|');
            if (parts.Length < 4)
            {
                Logger.Error(e, $"fail to log exception in program logger, parts length is too small: {parts.Length}, message: {message}");
            }

            var classname = parts[1];
            var callingMethodName = parts[2];
            var loadingProgramPath = parts[3];
            var interpretationMessage = parts[4];

            LogException(classname, callingMethodName, loadingProgramPath, interpretationMessage, e);
        }

    }
}