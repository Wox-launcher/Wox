using System.Diagnostics;
using Claunia.PropertyList;

namespace Wox.Plugin.App.AppLoader;

public class MacAppLoader : IAppLoader
{
    private readonly List<string> _appDirectories = new()
    {
        "/Applications",
        "/Applications/Utilities",
        "/System/Applications",
        "/System/Library/PreferencePanes"
    };

    private IPublicAPI _api;

    public async Task<List<AppInfo>> GetAllApps(IPublicAPI api)
    {
        _api = api;
        var startTimestamp = Stopwatch.GetTimestamp();
        api.Log("Start to get all mac apps");
        var apps = new List<AppInfo>();
        var appDirectories = _appDirectories.SelectMany(Directory.GetDirectories).Where(d => d.EndsWith(".app") || d.EndsWith(".prefPane"));

        await Parallel.ForEachAsync(appDirectories, new ParallelOptions { MaxDegreeOfParallelism = 5 }, async (appDirectory, token) =>
        {
            api.Log($"Start to get app info from {appDirectory}");
            var appInfo = await GetAppInfo(appDirectory);
            if (appInfo != null)
            {
                api.Log($"Get app=> {appInfo.IconPath}");
                apps.Add(appInfo);
            }
        });

        api.Log($"Get all mac apps cost {Stopwatch.GetElapsedTime(startTimestamp).TotalMilliseconds} ms");

        return apps;
    }

    private async Task<AppInfo?> GetAppInfo(string appDirectory)
    {
        var process = new Process
        {
            StartInfo =
            {
                FileName = "mdls",
                Arguments = $"-name kMDItemDisplayName -raw \"{appDirectory}\"",
                UseShellExecute = false,
                RedirectStandardOutput = true,
                CreateNoWindow = true
            }
        };
        process.Start();
        var name = await process.StandardOutput.ReadToEndAsync();
        await process.WaitForExitAsync();

        if (string.IsNullOrEmpty(name)) return null;
        var appInfo = new AppInfo
        {
            Name = name.Trim(),
            Path = appDirectory,
            IconPath = GetIcon(appDirectory) ?? ""
        };

        return appInfo;
    }

    /**
     * Get macos app icon
     */
    private string? GetIcon(string path)
    {
        _api.Log($"get mac app icon: {path}");
        FileInfo file = new(Path.Combine(path, "Contents/Info.plist"));
        if (file.Exists)
        {
            var rootDict = (NSDictionary)PropertyListParser.Parse(file);
            if (rootDict.TryGetValue("CFBundleIconFile", out var iconName))
            {
                var imageName = iconName.ToString();
                if (!imageName.EndsWith(".icns")) imageName = $"{iconName}.icns";

                return IcnsParser.Load(Path.Combine(path, "Contents/Resources", imageName));
            }
        }

        return null;
    }
}