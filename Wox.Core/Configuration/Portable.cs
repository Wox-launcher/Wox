using Squirrel;
using System;
using Wox.Infrastructure;

namespace Wox.Core.Configuration
{
    public class Portable : IPortable
    {
        private string applicationName;
        private string exeName;
        private string rootAppDirectory;
        private UpdateManager portabilityUpdater;

        public Portable()
        {
            applicationName = Constant.Wox;
            exeName = applicationName + ".exe";
            rootAppDirectory = Constant.RootDirectory;
            portabilityUpdater = new UpdateManager(string.Empty, applicationName, rootAppDirectory);
        }

        public void DisablePortableMode()
        {
            throw new NotImplementedException();
        }

        public void EnablePortableMode()
        {
            try
            {
                RemoveShortcuts();
                RemoveUninstallerEntry();
            }
            catch (Exception e)
            {
                //log and update error message to output above locations where shortcuts may not have been removed
#if DEBUG
                throw;
#else
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
            throw new NotImplementedException();
        }
    }
}
