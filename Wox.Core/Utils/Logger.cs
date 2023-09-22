using Microsoft.Extensions.Logging;
using Serilog;
using ILogger = Serilog.ILogger;

namespace Wox.Core.Utils;

public static class Logger
{
    private static readonly ILogger SeriLogger = new LoggerConfiguration()
        .WriteTo.File(
            Path.Combine(DataLocation.LogDirectory, "log.txt"),
            outputTemplate: "{Timestamp:yyyy-MM-dd HH:mm:ss.fff} [{Level:u3}] {Message:lj}{NewLine}{Exception}",
            rollOnFileSizeLimit: true,
            retainedFileCountLimit: 3,
            fileSizeLimitBytes: 1024 * 1024 * 100 /*100M*/)
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

    public static void Warn(string message)
    {
        SeriLogger.Warning(GetMessage(message));
    }

    public static void Error(string message, Exception e)
    {
        SeriLogger.Error(e, GetMessage(message));
    }

    public static Microsoft.Extensions.Logging.ILogger GetILogger()
    {
        return new LoggerFactory().AddSerilog(SeriLogger).CreateLogger("Logger");
    }
}