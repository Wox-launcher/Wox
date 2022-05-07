using Microsoft.Win32;
using System.Windows;
using System.Windows.Controls;
using Wox.Plugin.BrowserBookmark.Models;

namespace Wox.Plugin.BrowserBookmark.Views
{
    /// <summary>
    /// Interaction logic for BrowserBookmark.xaml
    /// </summary>
    public partial class SettingsControl : UserControl
    {
        private IPublicAPI _woxAPI;
        private readonly Settings _settings;

        public SettingsControl(IPublicAPI woxAPI,Settings settings)
        {
            InitializeComponent();
            _woxAPI = woxAPI;
            _settings = settings;
            browserPathBox.Text = _settings.BrowserPath;
            NewWindowBrowser.IsChecked = _settings.OpenInNewBrowserWindow;
            NewTabInBrowser.IsChecked = !_settings.OpenInNewBrowserWindow;
        }        

        private void OnNewBrowserWindowClick(object sender, RoutedEventArgs e)
        {
            _settings.OpenInNewBrowserWindow = true;
        }

        private void OnNewTabClick(object sender, RoutedEventArgs e)
        {
            _settings.OpenInNewBrowserWindow = false;
        }

        private void OnChooseClick(object sender, RoutedEventArgs e)
        {
            var fileBrowserDialog = new OpenFileDialog();
            fileBrowserDialog.Filter = _woxAPI.GetTranslation("wox_plugin_browserbookmark_plugin_filter");
            fileBrowserDialog.CheckFileExists = true;
            fileBrowserDialog.CheckPathExists = true;
            if (fileBrowserDialog.ShowDialog() == true)
            {
                browserPathBox.Text = fileBrowserDialog.FileName;
                _settings.BrowserPath = fileBrowserDialog.FileName;
            }
        }
    }
}
