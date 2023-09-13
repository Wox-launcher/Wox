using System;
using Wox.Plugin;

namespace Wox;

public class PublicAPIInstance : IPublicAPI
{
    public void ChangeQuery(string query)
    {
    }

    public void HideApp()
    {
    }

    public void ShowApp()
    {
    }

    public void ShowMsg(string title, string description = "", string iconPath = "")
    {
    }

    public void Log(string msg)
    {
        // should be directly used in PluginHostBase
        throw new NotImplementedException();
    }

    public string GetTranslation(string key)
    {
        return key + "- to be implemented";
    }
}