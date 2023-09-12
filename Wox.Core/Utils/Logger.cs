using Serilog;

namespace Wox.Core.Utils;

public static class Logger
{
    private static readonly ILogger SeriLogger = new LoggerConfiguration()
        .WriteTo.File(Path.Combine(DataLocation.LogDirectory, "log.txt"), rollOnFileSizeLimit: true, retainedFileCountLimit: 3, fileSizeLimitBytes: 1024 * 1024 * 100 /*100M*/)
        .MinimumLevel.Debug()
        .CreateLogger();

    private static string GetMessage(string message)
    {
        var currentThreadId = Environment.CurrentManagedThreadId.ToString().PadLeft(4, '0');
        return $"[{currentThreadId}] {message}";
    }

    public static void Debug(string message)
    {
        SeriLogger.Debug(GetMessage(message));
    }

    public static void Info(string message)
    {
        SeriLogger.Information(GetMessage(message));
    }

    public static void Error(string message)
    {
        SeriLogger.Error(GetMessage(message));
    }

    public static void Error(string message, Exception e)
    {
        SeriLogger.Error(e, GetMessage(message));
    }
}