using System.Diagnostics;
using System.Text;

namespace Wox.Plugin.App.AppLoader;

public static class IcnsParser
{
    private static readonly string CacheFolder = Path.Combine(Path.GetTempPath(), "wox", "app", "imageCache");

    static IcnsParser()
    {
        if (!Directory.Exists(CacheFolder)) Directory.CreateDirectory(CacheFolder);
    }

    private static string MD5(this string s)
    {
        using var provider = System.Security.Cryptography.MD5.Create();
        var builder = new StringBuilder();

        foreach (var b in provider.ComputeHash(Encoding.UTF8.GetBytes(s)))
            builder.Append(b.ToString("x2").ToLower());

        return builder.ToString();
    }

    /// <summary>
    ///     User osx command to parse icns icon
    ///     sips -s format png app.icns --out png_file.png
    /// </summary>
    /// <param name="path">icons icon path</param>
    /// <returns></returns>
    public static string? Load(string path)
    {
        var pathHash = path.MD5();
        var cacheFilePath = Path.Combine(CacheFolder, $"{pathHash}.png");
        if (File.Exists(cacheFilePath)) return cacheFilePath;

        using Process pProcess = new();
        pProcess.StartInfo.FileName = "sips";
        pProcess.StartInfo.Arguments = @$"-s format png ""{path}"" --out ""{Path.Combine(CacheFolder, pathHash)}.png""";
        pProcess.StartInfo.UseShellExecute = false;
        pProcess.StartInfo.RedirectStandardOutput = true;
        pProcess.StartInfo.WindowStyle = ProcessWindowStyle.Hidden;
        pProcess.StartInfo.CreateNoWindow = true;
        pProcess.Start();
        var output = pProcess.StandardOutput.ReadToEnd();
        pProcess.WaitForExit();
        if (pProcess.ExitCode == 0)
            if (File.Exists(cacheFilePath))
                return cacheFilePath;

        return null;
    }
}