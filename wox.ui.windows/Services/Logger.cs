using System;
using System.IO;

namespace Wox.UI.Windows.Services;

public static class Logger
{
    private static readonly string LogFilePath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "Wox", "logs", "ui-windows.log"
    );

    static Logger()
    {
        var logDir = Path.GetDirectoryName(LogFilePath);
        if (logDir != null && !Directory.Exists(logDir))
        {
            Directory.CreateDirectory(logDir);
        }
    }

    public static void Log(string message)
    {
        WriteLocal(message);
        TrySendLog("info", message);
    }

    public static void LogLocal(string message)
    {
        WriteLocal(message);
    }

    public static void Error(string message, Exception? ex = null)
    {
        var fullMessage = ex != null ? $"{message}: {ex.Message}\n{ex.StackTrace}" : message;
        WriteLocal($"ERROR: {fullMessage}");
        TrySendLog("error", fullMessage);
    }

    private static void WriteLocal(string message)
    {
        try
        {
            var logMessage = $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss.fff}] {message}";
            File.AppendAllText(LogFilePath, logMessage + Environment.NewLine);
            Console.WriteLine(logMessage);
        }
        catch
        {
            // Ignore logging errors
        }
    }

    private static void TrySendLog(string level, string message)
    {
        try
        {
            var traceId = Guid.NewGuid().ToString();
            WoxApiService.Instance.TrySendLog(traceId, level, message);
        }
        catch
        {
            // Ignore logging errors
        }
    }
}
