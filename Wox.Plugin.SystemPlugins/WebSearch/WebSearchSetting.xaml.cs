using System;
using System.Linq;
using System.Windows;
using System.Windows.Media.Imaging;
using Wox.Core.Data.Storage.UserSettings;
using MessageBox = System.Windows.MessageBox;

namespace Wox.Plugin.SystemPlugins
{
    public partial class WebSearchSetting : Window
    {
        private WebSearchesSetting settingWindow;
        private bool update;
        private WebSearch updateWebSearch;

        public WebSearchSetting(WebSearchesSetting settingWidow)
        {
            this.settingWindow = settingWidow;
            InitializeComponent();
        }

        public void UpdateItem(WebSearch webSearch)
        {
            updateWebSearch = UserSettingStorage.Instance.WebSearches.FirstOrDefault(o => o == webSearch);
            if (updateWebSearch == null || string.IsNullOrEmpty(updateWebSearch.Url))
            {
                MessageBox.Show("Invalid web search");
                Close();
                return;
            }

            update = true;
            lblAdd.Text = "Update";
            tbIconPath.Text = webSearch.IconPath;
            ShowIcon(webSearch.IconPath);
            cbEnable.IsChecked = webSearch.Enabled;
            tbTitle.Text = webSearch.Title;
            tbUrl.Text = webSearch.Url;
            tbActionword.Text = webSearch.ActionWord;
        }

        private void ShowIcon(string path)
        {
            try
            {
                imgIcon.Source = new BitmapImage(new Uri(path));
            }
            catch (Exception)
            {
            }
        }

        private void BtnCancel_OnClick(object sender, RoutedEventArgs e)
        {
            Close();
        }

        private void btnAdd_OnClick(object sender, RoutedEventArgs e)
        {
            string title = tbTitle.Text;
            if (string.IsNullOrEmpty(title))
            {
                MessageBox.Show("Please input Title field");
                return;
            }

            string url = tbUrl.Text;
            if (string.IsNullOrEmpty(url))
            {
                MessageBox.Show("Please input URL field");
                return;
            }

            string action = tbActionword.Text;
            if (string.IsNullOrEmpty(action))
            {
                MessageBox.Show("Please input ActionWord field");
                return;
            }


            if (!update)
            {
                if (UserSettingStorage.Instance.WebSearches.Exists(o => o.ActionWord == action))
                {
                    MessageBox.Show("ActionWord has existed, please input a new one.");
                    return;
                }
                UserSettingStorage.Instance.WebSearches.Add(new WebSearch()
                {
                    ActionWord = action,
                    Enabled = cbEnable.IsChecked ?? false,
                    IconPath = tbIconPath.Text,
                    Url = url,
                    Title = title
                });
                MessageBox.Show(string.Format("Add {0} web search successfully!", title));
            }
            else
            {
                if (UserSettingStorage.Instance.WebSearches.Exists(o => o.ActionWord == action && o != updateWebSearch))
                {
                    MessageBox.Show("ActionWord has existed, please input a new one.");
                    return;
                }
                updateWebSearch.ActionWord = action;
                updateWebSearch.IconPath = tbIconPath.Text;
                updateWebSearch.Enabled = cbEnable.IsChecked ?? false;
                updateWebSearch.Url = url;
                updateWebSearch.Title= title;
                MessageBox.Show(string.Format("Update {0} web search successfully!", title));
            }
            UserSettingStorage.Instance.Save();
            settingWindow.ReloadWebSearchView();
            Close();
        }

        private void BtnSelectIcon_OnClick(object sender, RoutedEventArgs e)
        {
            var dlg = new Microsoft.Win32.OpenFileDialog
            {
                DefaultExt = ".png",
                Filter =
                    "JPEG Files (*.jpeg)|*.jpeg|PNG Files (*.png)|*.png|JPG Files (*.jpg)|*.jpg|GIF Files (*.gif)|*.gif"
            };

            bool? result = dlg.ShowDialog();
            if (result == true)
            {
                string filename = dlg.FileName;
                tbIconPath.Text = filename;
                ShowIcon(filename);
            }
        }
    }
}
