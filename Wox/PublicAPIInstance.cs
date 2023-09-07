using System;
using System.Collections.Generic;
using Wox.Plugin;

namespace Wox;

public class PublicAPIInstance : IPublicAPI
{
    public void PushResults(Query query, PluginMetadata plugin, List<Result> results)
    {
        throw new NotImplementedException();
    }

    public void ChangeQuery(string query, bool requery = false)
    {
        throw new NotImplementedException();
    }

    public void RestarApp()
    {
        throw new NotImplementedException();
    }

    public void HideApp()
    {
        throw new NotImplementedException();
    }

    public void ShowApp()
    {
        throw new NotImplementedException();
    }

    public void SaveAppAllSettings()
    {
        throw new NotImplementedException();
    }

    public void ReloadAllPluginData()
    {
        throw new NotImplementedException();
    }

    public void CheckForNewUpdate()
    {
        throw new NotImplementedException();
    }

    public void ShowMsg(string title, string subTitle = "", string iconPath = "")
    {
        throw new NotImplementedException();
    }

    public void ShowMsg(string title, string subTitle, string iconPath, bool useMainWindowAsOwner = true)
    {
        throw new NotImplementedException();
    }

    public void OpenSettingDialog()
    {
        throw new NotImplementedException();
    }

    public void InstallPlugin(string path)
    {
        throw new NotImplementedException();
    }

    public string GetTranslation(string key)
    {
        throw new NotImplementedException();
    }

    public List<PluginMetadata> GetAllPlugins()
    {
        throw new NotImplementedException();
    }
}