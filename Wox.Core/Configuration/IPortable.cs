
namespace Wox.Core.Configuration
{
    public interface IPortable
    {
        void EnablePortableMode();
        void DisablePortableMode();
        void RemoveShortcuts();
        void RemoveUninstallerEntry();
        bool IsPortableModeEnabled();
        void MoveUserDataFolder(string fromLocation, string toLocation);
        void VerifyUserDataAfterMove(string fromLocation, string toLocation);
    }
}