using System;
using Wox.Plugin;

namespace Wox;

public class PublicAPIInstance : IPublicAPI
{
    public void ChangeQuery(string query)
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

    public void ShowMsg(string title, string description = "", string iconPath = "")
    {
        throw new NotImplementedException();
    }

    public string GetTranslation(string key)
    {
        throw new NotImplementedException();
    }
}