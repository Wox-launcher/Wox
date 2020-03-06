using Microsoft.Win32;
using Squirrel;
using System;
using System.IO;
using System.Reflection;
using System.Windows;
using Wox.Infrastructure;
using Wox.Infrastructure.UserSettings;
using Wox.Plugin.SharedCommands;

namespace Wox.Core.Configuration
{
    public class Portable : IPortable
    {
        private UpdateManager portabilityUpdater;

        public void DisablePortableMode()
        {
            portabilityUpdater = new UpdateManager(string.Empty, Constant.Wox, Constant.RootDirectory);

            try
            {
                MoveUserDataFolder(DataLocation.PortableDataPath, DataLocation.RoamingDataPath);
                CreateShortcuts();
                CreateUninstallerEntry();
                IndicateDeletion(DataLocation.PortableDataPath);

                MessageBox.Show("Wox needs to restart to finish disabling portable mode, " +
                    "after the restart your portable data profile will be deleted and roaming data profile kept");

                portabilityUpdater.Dispose();
                // CHANGE TO PRIVATE/INTERNAL METHODS

                UpdateManager.RestartApp();
            }
            catch (Exception e)
            {
                //log and update error message to output above locations where shortcuts may not have been removed
#if DEBUG
                portabilityUpdater.Dispose();
                throw;
#else
                portabilityUpdater.Dispose();
                throw;// PRODUCTION LOGGING AND CONTINUE
                
#endif
            }
        }

        public void EnablePortableMode()
        {
            portabilityUpdater = new UpdateManager(string.Empty, Constant.Wox, Constant.RootDirectory);

            try
            {
                MoveUserDataFolder(DataLocation.RoamingDataPath, DataLocation.PortableDataPath);
                RemoveShortcuts();
                RemoveUninstallerEntry();
                IndicateDeletion(DataLocation.RoamingDataPath);

                MessageBox.Show("Wox needs to restart to finish enabling portable mode, " +
                    "after the restart your roaming data profile will be deleted and portable data profile kept");

                portabilityUpdater.Dispose();

                UpdateManager.RestartApp();
            }
            catch (Exception e)
            {
                //log and update error message to output above locations where shortcuts may not have been removed
#if DEBUG
                portabilityUpdater.Dispose();
                throw;
#else
                portabilityUpdater.Dispose();
                throw;// PRODUCTION LOGGING AND CONTINUE
                
#endif
            }
        }

        public bool IsPortableModeEnabled()
        {
            throw new NotImplementedException();
        }

        public void RemoveShortcuts()
        {
            var exeName = Constant.Wox + ".exe";
            portabilityUpdater.RemoveShortcutsForExecutable(exeName, ShortcutLocation.StartMenu);
            portabilityUpdater.RemoveShortcutsForExecutable(exeName, ShortcutLocation.Desktop);
            portabilityUpdater.RemoveShortcutsForExecutable(exeName, ShortcutLocation.Startup);
        }

        public void RemoveUninstallerEntry()
        {
            portabilityUpdater.RemoveUninstallerRegistryEntry();
        }

        public void MoveUserDataFolder(string fromLocation, string toLocation)
        {
            FilesFolders.Copy(fromLocation, toLocation);
            VerifyUserDataAfterMove(fromLocation, toLocation);
        }

        public void VerifyUserDataAfterMove(string fromLocation, string toLocation)
        {
            FilesFolders.VerifyBothFolderFilesEqual(fromLocation, toLocation);
        }

        public void CreateShortcuts()
        {
            var exeName = Constant.Wox + ".exe";
            portabilityUpdater.CreateShortcutsForExecutable(exeName, ShortcutLocation.StartMenu, false);
            portabilityUpdater.CreateShortcutsForExecutable(exeName, ShortcutLocation.Desktop, false);
            portabilityUpdater.CreateShortcutsForExecutable(exeName, ShortcutLocation.Startup, false);
        }

        public void CreateUninstallerEntry()
        {
            var uninstallRegSubKey = @"Software\Microsoft\Windows\CurrentVersion\Uninstall";
            // NB: Sometimes the Uninstall key doesn't exist
            using (var parentKey =
                RegistryKey.OpenBaseKey(RegistryHive.CurrentUser, RegistryView.Default)
                    .CreateSubKey("Uninstall", RegistryKeyPermissionCheck.ReadWriteSubTree)) {; }

            var key = RegistryKey.OpenBaseKey(RegistryHive.CurrentUser, RegistryView.Default)
                .CreateSubKey(uninstallRegSubKey + "\\" + Constant.Wox, RegistryKeyPermissionCheck.ReadWriteSubTree);
            key.SetValue("DisplayIcon", Constant.ApplicationDirectory + "\\app.ico", RegistryValueKind.String);

            portabilityUpdater.CreateUninstallerRegistryEntry();
        }

        public void IndicateDeletion(string filePathTodelete)
        {
            using (StreamWriter sw = File.CreateText(filePathTodelete + "\\" + DataLocation.DeletionIndicatorFile)){}
        }

        public void CleanUpFolderAfterPortabilityUpdate()
        {
            var portableDataPath = Path.Combine(Directory.GetParent(Assembly.GetExecutingAssembly().Location.NonNull()).ToString(), "UserData");
            var roamingDataPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Wox");

            bool DataLocationPortableDeleteRequired = false;
            bool DataLocationRoamingDeleteRequired = false;

            if ((roamingDataPath + "\\" + DataLocation.DeletionIndicatorFile).FileExits())
                DataLocationRoamingDeleteRequired = true;

            if ((portableDataPath + "\\" + DataLocation.DeletionIndicatorFile).FileExits())
                DataLocationPortableDeleteRequired = true;

            if (DataLocationRoamingDeleteRequired)
            {
                if(roamingDataPath.LocationExists())
                    MessageBox.Show("Wox detected you restarted after enabling portable mode, " +
                                    "your roaming data profile will now be deleted");

                FilesFolders.RemoveFolderIfExists(roamingDataPath);

                return;
            }

            if(DataLocationPortableDeleteRequired)
            {
                MessageBox.Show("Wox detected you restarted after disabling portable mode, " +
                                    "your portable data profile will now be deleted");

                FilesFolders.RemoveFolderIfExists(portableDataPath);

                return;
            }
        }
    }
}
