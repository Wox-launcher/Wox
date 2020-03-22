using Microsoft.Win32;
using Squirrel;
using System;
using System.IO;
using System.Reflection;
using System.Windows;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
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
#if DEBUG
                // Create shortcuts and uninstaller are not required in debug mode, 
                // otherwise will repoint the path of the actual installed production version to the debug version
#else
                CreateShortcuts();
                CreateUninstallerEntry();
#endif
                IndicateDeletion(DataLocation.PortableDataPath);

                MessageBox.Show("Wox needs to restart to finish disabling portable mode, " +
                    "after the restart your portable data profile will be deleted and roaming data profile kept");

                portabilityUpdater.Dispose();

                UpdateManager.RestartApp();
            }
            catch (Exception e)
            {
                portabilityUpdater.Dispose();
#if !DEBUG
                Log.Exception("Portable", "Error occured while disabling portable mode", e);
#endif
                throw;
            }
        }

        public void EnablePortableMode()
        {
            portabilityUpdater = new UpdateManager(string.Empty, Constant.Wox, Constant.RootDirectory);

            try
            {
                MoveUserDataFolder(DataLocation.RoamingDataPath, DataLocation.PortableDataPath);
#if DEBUG
                // Remove shortcuts and uninstaller are not required in debug mode, 
                // otherwise will delete the actual installed production version
#else
                RemoveShortcuts();
                RemoveUninstallerEntry();
#endif
                IndicateDeletion(DataLocation.RoamingDataPath);

                MessageBox.Show("Wox needs to restart to finish enabling portable mode, " +
                    "after the restart your roaming data profile will be deleted and portable data profile kept");

                portabilityUpdater.Dispose();

                UpdateManager.RestartApp();
            }
            catch (Exception e)
            {
                portabilityUpdater.Dispose();
#if !DEBUG
                Log.Exception("Portable", "Error occured while enabling portable mode", e);
#endif
                throw;
            }
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

        internal void IndicateDeletion(string filePathTodelete)
        {
            using (StreamWriter sw = File.CreateText(filePathTodelete + "\\" + DataLocation.DeletionIndicatorFile)){}
        }

        ///<summary>
        ///This method should be run at first before all methods during start up and should be run before determining which data location
        ///will be used for Wox.
        ///</summary>
        public void PreStartCleanUpAfterPortabilityUpdate()
        {
            // Specify here so this method does not rely on other environment variables to initialise
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

        public bool CanUpdatePortability()
        {
            var roamingLocationExists = DataLocation.RoamingDataPath.LocationExists();
            var portableLocationExists = DataLocation.PortableDataPath.LocationExists();

            if(roamingLocationExists && portableLocationExists)
            {
                MessageBox.Show(string.Format("Wox detected your user data exists both in {0} and " +
                                    "{1}. {2}{2}Please delete {1} in order to proceed. No changes have occured.", 
                                    DataLocation.PortableDataPath, DataLocation.RoamingDataPath, Environment.NewLine));

                return false;
            }

            return true;
        }
    }
}
