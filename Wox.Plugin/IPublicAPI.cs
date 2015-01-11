﻿using System;
using System.Collections.Generic;
using System.Windows.Documents;

namespace Wox.Plugin
{
    public interface IPublicAPI
    {

        void PushResults(Query query,PluginMetadata plugin, List<Result> results);

        bool ShellRun(string cmd, bool runAsAdministrator = false);

        void ChangeQuery(string query, bool requery = false);

        void CloseApp();

        void HideApp();

        void ShowApp();

        void ShowMsg(string title, string subTitle, string iconPath);

        void OpenSettingDialog();

        void StartLoadingBar();

        void StopLoadingBar();

        void InstallPlugin(string path);

        void ReloadPlugins();

        List<PluginPair> GetAllPlugins();

        event WoxKeyDownEventHandler BackKeyDownEvent;
    }
}
