using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using AppxPackaing;
using Shell;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
using IStream = AppxPackaing.IStream;
using Rect = System.Windows.Rect;
using NLog;
using System.Collections.Concurrent;
using Microsoft.Win32;
using System.Xml;

namespace Wox.Plugin.Program.Programs
{
    [Serializable]
    public class UWP
    {
        public string FullName { get; }
        public string FamilyName { get; }
        public string Name { get; set; }
        public string Location { get; set; }

        public Application[] Apps { get; set; }

        public PackageVersion Version { get; set; }

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public UWP(string id, string location)
        {
            FullName = id;
            string[] parts = id.Split(new char[] { '_' }, StringSplitOptions.RemoveEmptyEntries);
            FamilyName = $"{parts[0]}_{parts[parts.Length - 1]}";
            Location = location;
        }


        /// <exception cref="ArgumentException"
        private void InitializeAppInfo()
        {
            var path = Path.Combine(Location, "AppxManifest.xml");
            using (var reader = XmlReader.Create(path))
            {
                bool success = reader.ReadToFollowing("Package");
                if (!success) { throw new ArgumentException($"Cannot read Package key from {path}"); }

                Version = PackageVersion.Unknown;
                for (int i = 0; i < reader.AttributeCount; i++)
                {
                    string schema = reader.GetAttribute(i);
                    if (schema != null)
                    {
                        if (schema == "http://schemas.microsoft.com/appx/manifest/foundation/windows10")
                        {
                            Version = PackageVersion.Windows10;
                        }
                        else if (schema == "http://schemas.microsoft.com/appx/2013/manifest")
                        {
                            Version = PackageVersion.Windows81;
                        }
                        else if (schema == "http://schemas.microsoft.com/appx/2010/manifest")
                        {
                            Version = PackageVersion.Windows8;
                        }
                        else
                        {
                            continue;
                        }
                    }
                }
                if (Version == PackageVersion.Unknown)
                {
                    throw new ArgumentException($"Unknowen schema version {path}");
                }

                success = reader.ReadToFollowing("Identity");
                if (!success) { throw new ArgumentException($"Cannot read Identity key from {path}"); }
                if (success)
                {
                    Name = reader.GetAttribute("Name");
                }

                success = reader.ReadToFollowing("Applications");
                if (!success) { throw new ArgumentException($"Cannot read Applications key from {path}"); }
                success = reader.ReadToDescendant("Application");
                if (!success) { throw new ArgumentException($"Cannot read Applications key from {path}"); }
                List<Application> apps = new List<Application>();
                do
                {
                    string id = reader.GetAttribute("Id");

                    reader.ReadToFollowing("uap:VisualElements");
                    string displayName = reader.GetAttribute("DisplayName");
                    string description = reader.GetAttribute("Description");
                    string backgroundColor = reader.GetAttribute("BackgroundColor");
                    string appListEntry = reader.GetAttribute("AppListEntry");

                    if (appListEntry == "none")
                    {
                        continue;
                    }

                    string logoUri = string.Empty;
                    if (Version == PackageVersion.Windows10)
                    {
                        logoUri = reader.GetAttribute("Square44x44Logo");
                    }
                    else if (Version == PackageVersion.Windows81)
                    {
                        logoUri = reader.GetAttribute("Square30x30Logo");
                    }
                    else if (Version == PackageVersion.Windows8)
                    {
                        logoUri = reader.GetAttribute("SmallLogo");
                    }
                    else
                    {
                        throw new ArgumentException($"Unknowen schema version {path}");
                    }

                    if (string.IsNullOrEmpty(displayName) || string.IsNullOrEmpty(id))
                    {
                        continue;
                    }

                    string userModelId = $"{FamilyName}!{id}";
                    Application app = new Application(this, userModelId, FullName, Name, displayName, description, logoUri, backgroundColor);

                    apps.Add(app);
                } while (reader.ReadToFollowing("Application"));
                Apps = apps.ToArray();
            }
        }

        public static Application[] All()
        {
            ConcurrentBag<Application> bag = new ConcurrentBag<Application>();
            Parallel.ForEach(PackageFoldersFromRegistry(), (package, state) =>
            {
                try
                {
                    package.InitializeAppInfo();
                    foreach (var a in package.Apps)
                    {
                        bag.Add(a);
                    }
                }
                catch (Exception e)
                {
                    e.Data.Add(nameof(package.FullName), package.FullName);
                    e.Data.Add(nameof(package.Location), package.Location);
                    Logger.WoxError($"Cannot parse UWP {package.Location}", e);
                }
            }
            );
            return bag.ToArray();
        }

        public static List<UWP> PackageFoldersFromRegistry()
        {

            var actiable = new HashSet<string>();
            string activableReg = @"Software\Classes\ActivatableClasses\Package";
            var activableRegSubkey = Registry.CurrentUser.OpenSubKey(activableReg);
            foreach (string name in activableRegSubkey.GetSubKeyNames())
            {
                actiable.Add(name);
            }

            var packages = new List<UWP>();
            string packageReg = @"Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\Repository\Packages";
            var packageRegSubkey = Registry.CurrentUser.OpenSubKey(packageReg);
            foreach (var name in packageRegSubkey.GetSubKeyNames())
            {
                var packageKey = packageRegSubkey.OpenSubKey(name);
                var framework = packageKey.GetValue("Framework");
                if (framework != null)
                {
                    if ((int)framework == 1)
                    {
                        continue;
                    }
                }
                var valueFolder = packageKey.GetValue("PackageRootFolder");
                var valueID = packageKey.GetValue("PackageID");
                if (valueID != null && valueFolder != null && actiable.Contains(valueID))
                {
                    string location = (string)valueFolder;
                    string id = (string)valueID;
                    UWP uwp = new UWP(id, location);
                    packages.Add(uwp);
                }
            }

            // only exception windows.immersivecontrolpanel_10.0.2.1000_neutral_neutral_cw5n1h2txyewy
            string settingsID = actiable.First(a => a.StartsWith("windows.immersivecontrolpanel"));
            string settingsLocation = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Windows), "ImmersiveControlPanel");
            UWP swttings = new UWP(settingsID, settingsLocation);
            packages.Add(swttings);

            return packages;
        }

        public override string ToString()
        {
            return FullName;
        }

        public override bool Equals(object obj)
        {
            if (obj is UWP uwp)
            {
                return FullName.Equals(uwp.FullName);
            }
            else
            {
                return false;
            }
        }

        public override int GetHashCode()
        {
            return FullName.GetHashCode();
        }

        [Serializable]
        public class Application : IProgram
        {
            public string DisplayName { get; set; }
            public string Description { get; set; }
            public string UserModelId { get; set; }
            public string BackgroundColor { get; set; }

            public string Name => DisplayName;
            public string Location => Package.Location;

            public bool Enabled { get; set; }

            public string LogoPath { get; set; }
            public UWP Package { get; set; }

            public Result Result(string query, IPublicAPI api)
            {
                var result = new Result
                {
                    SubTitle = Package.Location,
                    Icon = Logo,
                    ContextData = this,
                    Action = e =>
                    {
                        Launch(api);
                        return true;
                    }
                };

                string title;
                if (Description.Length >= DisplayName.Length &&
                    Description.Substring(0, DisplayName.Length) == DisplayName)
                {
                    title = Description;
                    result.Title = title;
                    var match = StringMatcher.FuzzySearch(query, title);
                    result.Score = match.Score;
                    result.TitleHighlightData = match.MatchData;
                }
                else if (!string.IsNullOrEmpty(Description))
                {
                    title = $"{DisplayName}: {Description}";
                    var match1 = StringMatcher.FuzzySearch(query, DisplayName);
                    var match2 = StringMatcher.FuzzySearch(query, title);
                    if (match1.Score > match2.Score)
                    {
                        result.Score = match1.Score;
                        result.TitleHighlightData = match1.MatchData;
                    }
                    else
                    {
                        result.Score = match2.Score;
                        result.TitleHighlightData = match2.MatchData;
                    }
                    result.Title = title;

                }
                else
                {
                    title = DisplayName;
                    result.Title = title;
                    var match = StringMatcher.FuzzySearch(query, title);
                    result.Score = match.Score;
                    result.TitleHighlightData = match.MatchData;
                }



                return result;
            }

            public List<Result> ContextMenus(IPublicAPI api)
            {
                var contextMenus = new List<Result>
                {
                    new Result
                    {
                        Title = api.GetTranslation("wox_plugin_program_open_containing_folder"),

                        Action = _ =>
                        {
                            Main.StartProcess(Process.Start, new ProcessStartInfo(Package.Location));

                            return true;
                        },

                        IcoPath = "Images/folder.png"
                    }
                };
                return contextMenus;
            }

            private async void Launch(IPublicAPI api)
            {
                var appManager = new ApplicationActivationManager();
                uint unusedPid;
                const string noArgs = "";
                const ACTIVATEOPTIONS noFlags = ACTIVATEOPTIONS.AO_NONE;
                await Task.Run(() =>
                {
                    try
                    {
                        appManager.ActivateApplication(UserModelId, noArgs, noFlags, out unusedPid);
                    }
                    catch (Exception)
                    {
                        var name = "Plugin: Program";
                        var message = $"Can't start UWP: {DisplayName}";
                        api.ShowMsg(name, message, string.Empty);
                    }
                });
            }

            public Application(UWP package, string userModelID, string fullName, string name, string displayname, string description, string logoUri, string backgroundColor)
            {
                UserModelId = userModelID;
                Enabled = true;
                Package = package;
                DisplayName = ResourcesFromPri(fullName, name, displayname);
                Description = ResourcesFromPri(fullName, name, description);
                LogoPath = PathFromUri(fullName, name, Location, logoUri);
                BackgroundColor = backgroundColor;
            }

            internal string ResourcesFromPri(string packageFullName, string packageName, string resourceReference)
            {
                const string prefix = "ms-resource:";
                string result = "";
                Logger.WoxTrace($"package: <{packageFullName}> res ref: <{resourceReference}>");
                if (!string.IsNullOrWhiteSpace(resourceReference) && resourceReference.StartsWith(prefix))
                {


                    string key = resourceReference.Substring(prefix.Length);
                    string parsed;
                    // DisplayName
                    // Microsoft.ScreenSketch_10.1907.2471.0_x64__8wekyb3d8bbwe -> ms-resource:AppName/Text
                    // Microsoft.OneConnect_5.2002.431.0_x64__8wekyb3d8bbwe -> ms-resource:/OneConnectStrings/OneConnect/AppDisplayName
                    // ImmersiveControlPanel -> ms-resource:DisplayName
                    // Microsoft.ConnectivityStore_1.1604.4.0_x64__8wekyb3d8bbwe -> ms-resource://Microsoft.ConnectivityStore/MSWifiResources/AppDisplayName
                    if (key.StartsWith("//"))
                    {
                        parsed = $"{prefix}{key}";
                    }
                    else
                    {
                        if (!key.StartsWith("/"))
                        {
                            key = $"/{key}";
                        }

                        if (!key.ToLower().Contains("resources") && key.Count(c => c == '/') < 3)
                        {
                            key = $"/Resources{key}";
                        }
                        parsed = $"{prefix}//{packageName}{key}";
                    }
                    Logger.WoxTrace($"resourceReference {resourceReference} parsed <{parsed}> package <{packageFullName}>");
                    try
                    {
                        result = ResourceFromPriInternal(packageFullName, parsed);
                    }
                    catch (Exception e)
                    {
                        e.Data.Add(nameof(resourceReference), resourceReference);
                        e.Data.Add(nameof(ResourcesFromPri) + nameof(parsed), parsed);
                        e.Data.Add(nameof(ResourcesFromPri) + nameof(packageFullName), packageFullName);
                        throw e;
                    }
                }
                else
                {
                    result = resourceReference;
                }
                Logger.WoxTrace($"package: <{packageFullName}> pri resource result: <{result}>");
                return result;
            }

            private string PathFromUri(string packageFullName, string packageName, string packageLocation, string fileReference)
            {
                // all https://msdn.microsoft.com/windows/uwp/controls-and-patterns/tiles-and-notifications-app-assets
                // windows 10 https://msdn.microsoft.com/en-us/library/windows/apps/dn934817.aspx
                // windows 8.1 https://msdn.microsoft.com/en-us/library/windows/apps/hh965372.aspx#target_size
                // windows 8 https://msdn.microsoft.com/en-us/library/windows/apps/br211475.aspx

                Logger.WoxTrace($"package: <{packageFullName}> file ref: <{fileReference}>");
                string path = Path.Combine(packageLocation, fileReference);
                if (File.Exists(path))
                {
                    // for 28671Petrroll.PowerPlanSwitcher_0.4.4.0_x86__ge82akyxbc7z4
                    return path;
                }
                else
                {
                    // https://docs.microsoft.com/en-us/windows/uwp/app-resources/pri-apis-scenario-1
                    string parsed = $"ms-resource:///Files/{fileReference.Replace("\\", "/")}";
                    try
                    {
                        string result = ResourceFromPriInternal(packageFullName, parsed);
                        Logger.WoxTrace($"package: <{packageFullName}> pri file result: <{result}>");
                        return result;
                    }
                    catch (Exception e)
                    {
                        e.Data.Add(nameof(fileReference), fileReference);
                        e.Data.Add(nameof(PathFromUri) + nameof(parsed), parsed);
                        e.Data.Add(nameof(PathFromUri) + nameof(packageFullName), packageFullName);
                        throw e;
                    }
                }
            }

            /// https://docs.microsoft.com/en-us/windows/win32/api/shlwapi/nf-shlwapi-shloadindirectstring
            /// use makepri to check whether the resource can be get, the error message is usually useless
            /// makepri.exe dump /if "a\resources.pri" /of b.xml 
            private string ResourceFromPriInternal(string packageFullName, string parsed)
            {
                Logger.WoxTrace($"package: <{packageFullName}> pri parsed: <{parsed}>");
                // following error probally due to buffer to small
                // '200' violates enumeration constraint of '100 120 140 160 180'.
                // 'Microsoft Corporation' violates pattern constraint of '\bms-resource:.{1,256}'.
                var outBuffer = new StringBuilder(512);
                string source = $"@{{{packageFullName}? {parsed}}}";
                var capacity = (uint)outBuffer.Capacity;
                var hResult = SHLoadIndirectString(source, outBuffer, capacity, IntPtr.Zero);
                if (hResult == Hresult.Ok)
                {
                    var loaded = outBuffer.ToString();
                    if (!string.IsNullOrEmpty(loaded))
                    {
                        return loaded;
                    }
                    else
                    {
                        Logger.WoxError($"Can't load null or empty result pri {source} in uwp location {Package.Location}");
                        return string.Empty;
                    }
                }
                else
                {
                    var e = Marshal.GetExceptionForHR((int)hResult);
                    e.Data.Add(nameof(source), source);
                    e.Data.Add(nameof(packageFullName), packageFullName);
                    e.Data.Add(nameof(parsed), parsed);
                    Logger.WoxError($"Load pri failed {source} location {Package.Location}", e);
                    return string.Empty;
                }
            }

            internal string LogoUriFromManifest(IAppxManifestApplication app)
            {
                var logoKeyFromVersion = new Dictionary<PackageVersion, string>
                {
                    { PackageVersion.Windows10, "Square44x44Logo" },
                    { PackageVersion.Windows81, "Square30x30Logo" },
                    { PackageVersion.Windows8, "SmallLogo" },
                };
                if (logoKeyFromVersion.ContainsKey(Package.Version))
                {
                    var key = logoKeyFromVersion[Package.Version];
                    var logoUri = app.GetStringValue(key);
                    return logoUri;
                }
                else
                {
                    return string.Empty;
                }
            }

            public ImageSource Logo()
            {
                var logo = ImageFromPath(LogoPath);
                var plated = PlatedImage(logo);

                // todo magic! temp fix for cross thread object
                plated.Freeze();
                return plated;
            }


            private BitmapImage ImageFromPath(string path)
            {
                if (File.Exists(path))
                {
                    var image = new BitmapImage(new Uri(path));
                    return image;
                }
                else
                {
                    Logger.WoxError($"|Unable to get logo for {UserModelId} from {path} and located in {Package.Location}");
                    return new BitmapImage(new Uri(Constant.ErrorIcon));
                }
            }

            private ImageSource PlatedImage(BitmapImage image)
            {
                if (!string.IsNullOrEmpty(BackgroundColor) && BackgroundColor != "transparent")
                {
                    var width = image.Width;
                    var height = image.Height;
                    var x = 0;
                    var y = 0;

                    var group = new DrawingGroup();

                    var converted = ColorConverter.ConvertFromString(BackgroundColor);
                    if (converted != null)
                    {
                        var color = (Color)converted;
                        var brush = new SolidColorBrush(color);
                        var pen = new Pen(brush, 1);
                        var backgroundArea = new Rect(0, 0, width, width);
                        var rectabgle = new RectangleGeometry(backgroundArea);
                        var rectDrawing = new GeometryDrawing(brush, pen, rectabgle);
                        group.Children.Add(rectDrawing);

                        var imageArea = new Rect(x, y, image.Width, image.Height);
                        var imageDrawing = new ImageDrawing(image, imageArea);
                        group.Children.Add(imageDrawing);

                        // http://stackoverflow.com/questions/6676072/get-system-drawing-bitmap-of-a-wpf-area-using-visualbrush
                        var visual = new DrawingVisual();
                        var context = visual.RenderOpen();
                        context.DrawDrawing(group);
                        context.Close();
                        const int dpiScale100 = 96;
                        var bitmap = new RenderTargetBitmap(
                            Convert.ToInt32(width), Convert.ToInt32(height),
                            dpiScale100, dpiScale100,
                            PixelFormats.Pbgra32
                        );
                        bitmap.Render(visual);
                        return bitmap;
                    }
                    else
                    {
                        Logger.WoxError($"Unable to convert background string {BackgroundColor} to color for {Package.Location}");
                        return new BitmapImage(new Uri(Constant.ErrorIcon));
                    }
                }
                else
                {
                    // todo use windows theme as background
                    return image;
                }
            }

            public override string ToString()
            {
                return $"{DisplayName}: {Description}";
            }
        }

        public enum PackageVersion
        {
            Windows10,
            Windows81,
            Windows8,
            Unknown
        }

        [Flags]
        private enum Stgm : uint
        {
            Read = 0x0,
            ShareExclusive = 0x10,
            ShareDenyNone = 0x40,
        }

        private enum Hresult : uint
        {
            Ok = 0x0000,
        }

        [DllImport("shlwapi.dll", CharSet = CharSet.Unicode)]
        private static extern Hresult SHCreateStreamOnFileEx(string fileName, Stgm grfMode, uint attributes, bool create,
            IStream reserved, out IStream stream);

        [DllImport("shlwapi.dll", CharSet = CharSet.Unicode)]
        private static extern Hresult SHLoadIndirectString(string pszSource, StringBuilder pszOutBuf, uint cchOutBuf,
            IntPtr ppvReserved);
    }
}