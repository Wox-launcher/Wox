
namespace Wox.Core.Configuration
{
    public interface IPortable
    {
        void EnablePortableMode();
        void DisablePortableMode();
        void RemoveShortcuts();
        void RemoveUninstallerEntry();
        void CreateShortcuts();
        void CreateUninstallerEntry();
        bool IsPortableModeEnabled();
        void MoveUserDataFolder(string fromLocation, string toLocation);
        void VerifyUserDataAfterMove(string fromLocation, string toLocation);
        bool CanUpdatePortability();
    }
}