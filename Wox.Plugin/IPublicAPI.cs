namespace Wox.Plugin;

/// <summary>
///     Public APIs that plugin can use
/// </summary>
public interface IPublicAPI
{
    /// <summary>
    ///     Change Wox query
    /// </summary>
    /// <param name="query">query text</param>
    void ChangeQuery(string query);

    /// <summary>
    ///     Hide Wox
    /// </summary>
    void HideApp();

    /// <summary>
    ///     Show Wox
    /// </summary>
    void ShowApp();

    /// <summary>
    ///     Show message box
    /// </summary>
    /// <param name="title">Message title</param>
    /// <param name="description">Message description</param>
    /// <param name="iconPath">Message icon path (relative path to your plugin folder)</param>
    void ShowMsg(string title, string description = "", string iconPath = "");

    /// <summary>
    ///     Get translation of current language
    ///     You need to implement IPluginI18n if you want to support multiple languages for your plugin
    /// </summary>
    string GetTranslation(string key);
}