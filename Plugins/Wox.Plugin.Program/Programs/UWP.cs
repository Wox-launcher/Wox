using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Security.Principal;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Xml.Linq;
using Windows.ApplicationModel;
using Windows.Management.Deployment;
using AppxPackaing;
using Shell;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
using IStream = AppxPackaing.IStream;
using Rect = System.Windows.Rect;
using NLog;
using System.Collections.Concurrent;
using Windows.UI.Xaml.Controls;

namespace Wox.Plugin.Program.Programs
{
    [Serializable]
    public class UWP
    {
        public string Name { get; }
        public string FullName { get; }
        public string FamilyName { get; }
        public string Location { get; set; }

        public Application[] Apps { get; set; }

        public PackageVersion Version { get; set; }

        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public UWP(Package package)
        {
            Location = package.InstalledLocation.Path;
            Name = package.Id.Name;
            FullName = package.Id.FullName;
            FamilyName = package.Id.FamilyName;
            InitializeAppInfo();
            Apps = Apps.Where(a =>
            {
                var valid =
                    !string.IsNullOrEmpty(a.UserModelId) &&
                    !string.IsNullOrEmpty(a.DisplayName);
                return valid;
            }).ToArray();
        }

        private void InitializeAppInfo()
        {
            var path = Path.Combine(Location, "AppxManifest.xml");

            try
            {
                var namespaces = XmlNamespaces(path);
                InitPackageVersion(namespaces);
            }
            catch (ArgumentException e)
            {
                Logger.WoxError(e.Message);
                Apps = Apps = new List<Application>().ToArray();
                return;
            }


            var appxFactory = new AppxFactory();
            IStream stream;
            const uint noAttribute = 0x80;
            // shared read will slow speed https://docs.microsoft.com/en-us/windows/win32/stg/stgm-constants
            // but cannot find a way to release stearm, so use shared read
            // exclusive read will cause exception during reinexing
            // System.IO.FileLoadException: The process cannot access the file because it is being used by another process
            const Stgm sharedRead = Stgm.Read | Stgm.ShareDenyNone;
            var hResult = SHCreateStreamOnFileEx(path, sharedRead, noAttribute, false, null, out stream);

            if (hResult == Hresult.Ok)
            {
                var reader = appxFactory.CreateManifestReader(stream);
                var manifestApps = reader.GetApplications();
                var apps = new List<Application>();
                while (manifestApps.GetHasCurrent() != 0)
                {
                    var manifestApp = manifestApps.GetCurrent();
                    var appListEntry = manifestApp.GetStringValue("AppListEntry");
                    if (appListEntry != "none")
                    {
                        var app = new Application(manifestApp, this);
                        apps.Add(app);
                    }
                    manifestApps.MoveNext();
                }
                Apps = apps.Where(a => a.AppListEntry != "none").ToArray();
                return;
            }
            else
            {
                var e = Marshal.GetExceptionForHR((int)hResult);
                e.Data.Add(nameof(path), path);
                Logger.WoxError($"Cannot not get UWP details {path}", e);
                Apps = new List<Application>().ToArray();
                return;
            }
        }



        /// http://www.hanselman.com/blog/GetNamespacesFromAnXMLDocumentWithXPathDocumentAndLINQToXML.aspx
        /// <exception cref="ArgumentException"
        private string[] XmlNamespaces(string path)
        {
            XDocument z = XDocument.Load(path);
            if (z.Root != null)
            {
                var namespaces = z.Root.Attributes().
                    Where(a => a.IsNamespaceDeclaration).
                    GroupBy(
                        a => a.Name.Namespace == XNamespace.None ? string.Empty : a.Name.LocalName,
                        a => XNamespace.Get(a.Value)
                    ).Select(
                        g => g.First().ToString()
                    ).ToArray();
                return namespaces;
            }
            else
            {
                throw new ArgumentException("Cannot read XML from {path}");
            }
        }

        private void InitPackageVersion(string[] namespaces)
        {
            var versionFromNamespace = new Dictionary<string, PackageVersion>
            {
                {"http://schemas.microsoft.com/appx/manifest/foundation/windows10", PackageVersion.Windows10},
                {"http://schemas.microsoft.com/appx/2013/manifest", PackageVersion.Windows81},
                {"http://schemas.microsoft.com/appx/2010/manifest", PackageVersion.Windows8},
            };

            foreach (var n in versionFromNamespace.Keys)
            {
                if (namespaces.Contains(n))
                {
                    Version = versionFromNamespace[n];
                    return;
                }
            }

            throw new ArgumentException($"Unknown package version {string.Join(",", namespaces)}");
        }

        public static Application[] All()
        {
            var windows10 = new Version(10, 0);
            var support = Environment.OSVersion.Version.Major >= windows10.Major;
            if (support)
            {
                ConcurrentBag<Application> bag = new ConcurrentBag<Application>();
                Parallel.ForEach(CurrentUserPackages(), (p, state) =>
                {
                    UWP u;
                    try
                    {
                        u = new UWP(p);
                        foreach (var a in u.Apps)
                        {
                            bag.Add(a);
                        }
                    }
                    catch (Exception e)
                    {
                        e.Data.Add(nameof(p.Id.FullName), p.Id.FullName);
                        Logger.WoxError($"Cannot parse UWP {p.Id.FullName}", e, false, true);
                    }
                }
                );
                return bag.ToArray();
            }
            else
            {
                return new Application[] { };
            }
        }

        private static IEnumerable<Package> CurrentUserPackages()
        {
            var u = WindowsIdentity.GetCurrent().User;

            if (u != null)
            {
                var id = u.Value;
                var m = new PackageManager();
                var ps = m.FindPackagesForUser(id);
                ps = ps.Where(p =>
                {
                    bool valid;
                    try
                    {
                        var f = p.IsFramework;
                        var d = p.IsDevelopmentMode;
                        var path = p.InstalledLocation.Path;
                        valid = !f && !d && !string.IsNullOrEmpty(path);
                    }
                    catch (Exception e)
                    {
                        Type t = u.GetType();
                        if (t.IsSerializable)
                        {
                            e.Data.Add(nameof(u), u);
                        }
                        else
                        {
                            e.Data.Add(nameof(u), "Error: u not Serializable");
                        }
                        e.Data.Add(nameof(p.Id.FullName), p.Id.FullName);
                        Logger.WoxError($"cannot get package {u} {p.Id.FullName}", e);
                        return false;
                    }

                    return valid;
                });
                return ps;
            }
            else
            {
                return new Package[] { };
            }
        }

        public override string ToString()
        {
            return FamilyName;
        }

        public override bool Equals(object obj)
        {
            if (obj is UWP uwp)
            {
                return FamilyName.Equals(uwp.FamilyName);
            }
            else
            {
                return false;
            }
        }

        public override int GetHashCode()
        {
            return FamilyName.GetHashCode();
        }

        [Serializable]
        public class Application : IProgram
        {
            public string AppListEntry { get; set; }
            public string DisplayName { get; set; }
            public string Description { get; set; }
            public string UserModelId { get; set; }
            public string BackgroundColor { get; set; }

            public string Name => DisplayName;
            public string Location => Package.Location;

            public bool Enabled { get; set; }

            public string LogoUri { get; set; }
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
                }
                else if (!string.IsNullOrEmpty(Description))
                {
                    title = $"{DisplayName}: {Description}";
                }
                else
                {
                    title = DisplayName;
                }
                var match = StringMatcher.FuzzySearch(query, title);
                result.Title = title;
                result.Score = match.Score;
                result.TitleHighlightData = match.MatchData;

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

            public Application(IAppxManifestApplication manifestApp, UWP package)
            {
                UserModelId = manifestApp.GetAppUserModelId();
                DisplayName = manifestApp.GetStringValue("DisplayName");
                Description = manifestApp.GetStringValue("Description");
                BackgroundColor = manifestApp.GetStringValue("BackgroundColor");
                Package = package;
                DisplayName = ResourcesFromPri(package.FullName, package.Name, DisplayName);
                Description = ResourcesFromPri(package.FullName, package.Name, Description);
                LogoUri = LogoUriFromManifest(manifestApp);
                LogoPath = PathFromUri(package.FullName, package.Name, package.Location, LogoUri);

                Enabled = true;
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
                    e.Data.Add(nameof(Package.Location), Package.Location);
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