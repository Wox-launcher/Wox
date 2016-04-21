﻿using System;
using System.IO;
using System.Linq;
using System.Windows;
using System.Windows.Media.Imaging;
using Microsoft.Win32;
using Wox.Infrastructure;
using Wox.Infrastructure.Exception;

namespace Wox.Plugin.WebSearch
{
    public partial class WebSearchSetting : Window
    {
        private const string _imageDirectoryName = "Images";
        private string _pluginDirectory = WoxDirectroy.Executable;
        private readonly WebSearchesSetting _settingWindow;
        private bool _isUpdate;
        private WebSearch _updateWebSearch;
        private readonly PluginInitContext _context;
        private readonly WebSearchPlugin _plugin;
        private Settings _settings;

        public WebSearchSetting(WebSearchesSetting settingWidow, Settings settings)
        {
            _plugin = settingWidow.Plugin;
            _context = settingWidow.Context;
            _settingWindow = settingWidow;
            InitializeComponent();
            _settings = settings;
        }

        public void UpdateItem(WebSearch webSearch)
        {
            _updateWebSearch = _settings.WebSearches.FirstOrDefault(o => o == webSearch);
            if (_updateWebSearch == null || string.IsNullOrEmpty(_updateWebSearch.Url))
            {

                string warning = _context.API.GetTranslation("wox_plugin_websearch_invalid_web_search");
                MessageBox.Show(warning);
                Close();
                return;
            }

            _isUpdate = true;
            lblAdd.Text = "Update";
            tbIconPath.Text = webSearch.IconPath;
            ShowIcon(webSearch.IconPath);
            cbEnable.IsChecked = webSearch.Enabled;
            tbTitle.Text = webSearch.Title;
            tbUrl.Text = webSearch.Url;
            tbActionword.Text = webSearch.ActionKeyword;
        }

        private void ShowIcon(string path)
        {
            imgIcon.Source = new BitmapImage(new Uri(Path.Combine(_pluginDirectory, path), UriKind.Absolute));
        }

        private void BtnCancel_OnClick(object sender, RoutedEventArgs e)
        {
            Close();
        }

        /// <summary>
        /// Confirm button for both add and update
        /// </summary>
        private void btnConfirm_OnClick(object sender, RoutedEventArgs e)
        {
            string title = tbTitle.Text;
            if (string.IsNullOrEmpty(title))
            {
                string warning = _context.API.GetTranslation("wox_plugin_websearch_input_title");
                MessageBox.Show(warning);
                return;
            }

            string url = tbUrl.Text;
            if (string.IsNullOrEmpty(url))
            {
                string warning = _context.API.GetTranslation("wox_plugin_websearch_input_url");
                MessageBox.Show(warning);
                return;
            }

            string newActionKeyword = tbActionword.Text.Trim();
            if (_isUpdate)
            {
                try
                {
                    _plugin.NotifyActionKeywordsUpdated(_updateWebSearch.ActionKeyword, newActionKeyword);
                }
                catch (WoxPluginException exception)
                {
                    MessageBox.Show(exception.Message);
                    return;
                }

                _updateWebSearch.ActionKeyword = newActionKeyword;
                _updateWebSearch.IconPath = tbIconPath.Text;
                _updateWebSearch.Enabled = cbEnable.IsChecked ?? false;
                _updateWebSearch.Url = url;
                _updateWebSearch.Title = title;
            }
            else
            {
                try
                {
                    _plugin.NotifyActionKeywordsAdded(newActionKeyword);
                }
                catch (WoxPluginException exception)
                {
                    MessageBox.Show(exception.Message);
                    return;
                }
                _settings.WebSearches.Add(new WebSearch
                {
                    ActionKeyword = newActionKeyword,
                    Enabled = cbEnable.IsChecked ?? false,
                    IconPath = tbIconPath.Text,
                    Url = url,
                    Title = title
                });
            }

            _settingWindow.ReloadWebSearchView();
            Close();
        }

        private void BtnSelectIcon_OnClick(object sender, RoutedEventArgs e)
        {
            if (!Directory.Exists(_pluginDirectory))
            {
                _pluginDirectory =
                    Path.GetDirectoryName(WoxDirectroy.Executable);
            }

            var dlg = new OpenFileDialog
            {
                InitialDirectory = Path.Combine(_pluginDirectory, _imageDirectoryName),
                Filter = "Image files (*.jpg, *.jpeg, *.gif, *.png, *.bmp) |*.jpg; *.jpeg; *.gif; *.png; *.bmp"
            };

            bool? result = dlg.ShowDialog();
            if (result == true)
            {
                string filename = dlg.FileName;
                if (filename != null)
                {
                    tbIconPath.Text = Path.Combine(_imageDirectoryName, Path.GetFileName(filename));
                    ShowIcon(tbIconPath.Text);
                }
            }
        }
    }
}
