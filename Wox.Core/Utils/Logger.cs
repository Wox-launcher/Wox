using Serilog;

namespace Wox.Core.Utils;

public static class Logger
{
    private static readonly ILogger SeriLogger = new LoggerConfiguration()
        .WriteTo.File(Path.Combine(DataLocation.LogDirectory, "log.txt"), rollOnFileSizeLimit: true, retainedFileCountLimit: 3, fileSizeLimitBytes: 1024 * 1024 * 100 /*100M*/)
        .CreateLogger();

    public static void Debug(string message)
    {
        SeriLogger.Debug(message);
    }

    public static void Info(string message)
    {
        SeriLogger.Information(message);
    }

    public static void Error(string message)
    {
        SeriLogger.Error(message);
    }

    public static void Error(string message, Exception e)
    {
        SeriLogger.Error(e, message);
    }
}