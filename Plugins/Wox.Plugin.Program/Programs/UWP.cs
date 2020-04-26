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
using Wox.Plugin.Program.Logger;
using IStream = AppxPackaing.IStream;
using Rect = System.Windows.Rect;
using NLog;

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
            const Stgm exclusiveRead = Stgm.Read | Stgm.ShareExclusive;
            var hResult = SHCreateStreamOnFileEx(path, exclusiveRead, noAttribute, false, null, out stream);

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
                var applications = CurrentUserPackages().AsParallel().SelectMany(p =>
                {
                    UWP u;
                    try
                    {
                        u = new UWP(p);
                    }
#if !DEBUG
                    catch (Exception e)
                    {
                        ProgramLogger.LogException($"|UWP|All|{p.InstalledLocation}|An unexpected error occured and "
                                                        + $"unable to convert Package to UWP for {p.Id.FullName}", e);
                        return new Application[] { };
                    }
#endif
#if DEBUG //make developer aware and implement handling
                    catch
                    {
                        throw;
                    }
#endif
                    return u.Apps;
                }).ToArray();

                var updatedListWithoutDisabledApps = applications
                                                        .Where(t1 => !Main._settings.DisabledProgramSources
                                                                        .Any(x => x.UniqueIdentifier == t1.UniqueIdentifier))
                                                        .Select(x => x);

                return updatedListWithoutDisabledApps.ToArray();
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
                        ProgramLogger.LogException("UWP", "CurrentUserPackages", $"id", "An unexpected error occured and "
                                                   + $"unable to verify if package is valid", e);
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
            public string UniqueIdentifier { get; set; }
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

            private int Score(string query)
            {
                var displayNameMatch = StringMatcher.FuzzySearch(query, DisplayName);
                var descriptionMatch = StringMatcher.FuzzySearch(query, Description);
                var score = new[] { displayNameMatch.Score, descriptionMatch.Score }.Max();
                return score;
            }

            public Result Result(string query, IPublicAPI api)
            {
                var score = Score(query);
                if (score <= 0)
                { // no need to create result if score is 0
                    return null;
                }

                var result = new Result
                {
                    SubTitle = Package.Location,
                    Icon = Logo,
                    Score = score,
                    ContextData = this,
                    Action = e =>
                    {
                        Launch(api);
                        return true;
                    }
                };

                if (Description.Length >= DisplayName.Length &&
                    Description.Substring(0, DisplayName.Length) == DisplayName)
                {
                    result.Title = Description;
                    result.TitleHighlightData = StringMatcher.FuzzySearch(query, Description).MatchData;
                }
                else if (!string.IsNullOrEmpty(Description))
                {
                    var title = $"{DisplayName}: {Description}";
                    result.Title = title;
                    result.TitleHighlightData = StringMatcher.FuzzySearch(query, title).MatchData;
                }
                else
                {
                    result.Title = DisplayName;
                    result.TitleHighlightData = StringMatcher.FuzzySearch(query, DisplayName).MatchData;
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

            public Application(IAppxManifestApplication manifestApp, UWP package)
            {
                UserModelId = manifestApp.GetAppUserModelId();
                UniqueIdentifier = manifestApp.GetAppUserModelId();
                DisplayName = manifestApp.GetStringValue("DisplayName");
                Description = manifestApp.GetStringValue("Description");
                BackgroundColor = manifestApp.GetStringValue("BackgroundColor");
                Package = package;
                DisplayName = ResourcesFromPri(package.FullName, package.Name, DisplayName);
                Description = ResourcesFromPri(package.FullName, package.Name, Description);
                LogoUri = LogoUriFromManifest(manifestApp);
                LogoPath = FilesFromPri(package.FullName, package.Name, LogoUri);

                Enabled = true;
            }

            internal string ResourcesFromPri(string packageFullName, String packageName, string resourceReference)
            {
                const string prefix = "ms-resource:";
                string result = "";
                Logger.WoxDebug($"package: <{packageFullName}> res ref: <{resourceReference}>");
                if (!string.IsNullOrWhiteSpace(resourceReference) && resourceReference.StartsWith(prefix))
                {


                    string key = resourceReference.Substring(prefix.Length);
                    string parsed;
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

                        if (!key.ToLower().Contains("resources"))
                        {
                            key = $"/Resources{key}";
                        }
                        parsed = $"{prefix}//{packageName}{key}";
                    }

                    result = ResourceFromPriInternal(packageFullName, parsed);
                }
                else
                {
                    result = resourceReference;
                }
                Logger.WoxDebug($"package: <{packageFullName}> pri resource result: <{result}>");
                return result;
            }

            private string FilesFromPri(string packageFullName, string packageName, string fileReference)
            {
                // all https://msdn.microsoft.com/windows/uwp/controls-and-patterns/tiles-and-notifications-app-assets
                // windows 10 https://msdn.microsoft.com/en-us/library/windows/apps/dn934817.aspx
                // windows 8.1 https://msdn.microsoft.com/en-us/library/windows/apps/hh965372.aspx#target_size
                // windows 8 https://msdn.microsoft.com/en-us/library/windows/apps/br211475.aspx

                Logger.WoxDebug($"package: <{packageFullName}> file ref: <{fileReference}>");
                string parsed = $"ms-resource://{packageName}/Files/{fileReference.Replace("\\", "/")}";
                string result = ResourceFromPriInternal(packageFullName, parsed);
                Logger.WoxDebug($"package: <{packageFullName}> pri file result: <{result}>");
                return result;
            }

            /// https://docs.microsoft.com/en-us/windows/win32/api/shlwapi/nf-shlwapi-shloadindirectstring
            /// use makepri to check whether the resource can be get, the error message is usually useless
            /// makepri.exe dump /if "a\resources.pri" /of b.xml 
            private string ResourceFromPriInternal(string packageFullName, string parsed)
            {
                Logger.WoxDebug($"package: <{packageFullName}> pri parsed: <{parsed}>");
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
                        ProgramLogger.LogException($"|UWP|ResourceFromPriInternal|{Package.Location}|Can't load null or empty result "
                                                    + $"pri {source} in uwp location {Package.Location}", new NullReferenceException());
                        return string.Empty;
                    }
                }
                else
                {
                    var e = Marshal.GetExceptionForHR((int)hResult);
                    ProgramLogger.LogException($"|UWP|ResourceFromPriInternal|{Package.Location}|Load pri failed {source} with HResult {hResult} and location {Package.Location}", e);
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
                    ProgramLogger.LogException($"|UWP|ImageFromPath|{path}" +
                                                    $"|Unable to get logo for {UserModelId} from {path} and" +
                                                    $" located in {Package.Location}", new FileNotFoundException());
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
                        ProgramLogger.LogException($"|UWP|PlatedImage|{Package.Location}" +
                                                    $"|Unable to convert background string {BackgroundColor} " +
                                                    $"to color for {Package.Location}", new InvalidOperationException());

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