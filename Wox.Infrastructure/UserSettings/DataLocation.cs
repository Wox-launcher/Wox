using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace Wox.Infrastructure.UserSettings
{
    public static class DataLocation
    {
        public static bool PortableDataLocationInUse;
        public const string PortableFolderName = "UserData";
        public static string PortableDataPath = Path.Combine(Constant.ProgramDirectory, PortableFolderName);
        public static string RoamingDataPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), Constant.Wox);
        public static string DataDirectory()
        {
            if (Directory.Exists(PortableDataPath))
            {
                PortableDataLocationInUse = true;
                return PortableDataPath;
            }
            else
            {
                return RoamingDataPath;
            }
        }

        public static readonly string PluginsDirectory = Path.Combine(DataDirectory(), Constant.Plugins);
    }
}
