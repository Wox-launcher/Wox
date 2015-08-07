﻿using System.Collections.Generic;
using System.Linq;
using System.Windows;
using System.Windows.Forms;
using MessageBox = System.Windows.MessageBox;
using UserControl = System.Windows.Controls.UserControl;

namespace Wox.Plugin.Folder
{

    public partial class FileSystemSettings : UserControl
    {
        private IPublicAPI woxAPI;

        public FileSystemSettings(IPublicAPI woxAPI)
        {
            this.woxAPI = woxAPI;
            InitializeComponent();
            lbxFolders.ItemsSource = FolderStorage.Instance.FolderLinks;
        }

        private void btnDelete_Click(object sender, RoutedEventArgs e)
        {
            var selectedFolder = lbxFolders.SelectedItem as FolderLink;
            if (selectedFolder != null)
            {
                string msg = string.Format(woxAPI.GetTranslation("wox_plugin_folder_delete_folder_link"), selectedFolder.Path);

                if (MessageBox.Show(msg, string.Empty, MessageBoxButton.YesNo) == MessageBoxResult.Yes)
                {
                    FolderStorage.Instance.FolderLinks.Remove(selectedFolder);
                    lbxFolders.Items.Refresh();
                    FolderStorage.Instance.Save();
                }
            }
            else
            {
                string warning = woxAPI.GetTranslation("wox_plugin_folder_select_folder_link_warning");
                MessageBox.Show(warning);
            }
        }

        private void btnEdit_Click(object sender, RoutedEventArgs e)
        {
            var selectedFolder = lbxFolders.SelectedItem as FolderLink;
            if (selectedFolder != null)
            {
                var folderBrowserDialog = new FolderBrowserDialog();
                folderBrowserDialog.SelectedPath = selectedFolder.Path;
                if (folderBrowserDialog.ShowDialog() == DialogResult.OK)
                {
                    var link = FolderStorage.Instance.FolderLinks.First(x => x.Path == selectedFolder.Path);
                    link.Path = folderBrowserDialog.SelectedPath;

                    FolderStorage.Instance.Save();
                }

                lbxFolders.Items.Refresh();
            }
            else
            {
                string warning = woxAPI.GetTranslation("wox_plugin_folder_select_folder_link_warning");
                MessageBox.Show(warning);
            }
        }

        private void btnAdd_Click(object sender, RoutedEventArgs e)
        {
            var folderBrowserDialog = new System.Windows.Forms.FolderBrowserDialog();
            if (folderBrowserDialog.ShowDialog() == System.Windows.Forms.DialogResult.OK)
            {
                var newFolder = new FolderLink()
                {
                    Path = folderBrowserDialog.SelectedPath
                };

                if (FolderStorage.Instance.FolderLinks == null)
                {
                    FolderStorage.Instance.FolderLinks = new List<FolderLink>();
                }

                FolderStorage.Instance.FolderLinks.Add(newFolder);
                FolderStorage.Instance.Save();
            }

            lbxFolders.Items.Refresh();
        }

        private void lbxFolders_Drop(object sender, System.Windows.DragEventArgs e)
        {
            string[] files = (string[])e.Data.GetData(System.Windows.DataFormats.FileDrop);

            if (files != null && files.Count() > 0)
            {
                if (FolderStorage.Instance.FolderLinks == null)
                {
                    FolderStorage.Instance.FolderLinks = new List<FolderLink>();
                }

                foreach (string s in files)
                {
                    if (System.IO.Directory.Exists(s) == true)
                    {
                        var newFolder = new FolderLink()
                        {
                            Path = s
                        };

                        FolderStorage.Instance.FolderLinks.Add(newFolder);
                        FolderStorage.Instance.Save();
                    }

                    lbxFolders.Items.Refresh();
                }
            }
        }

        private void lbxFolders_DragEnter(object sender, System.Windows.DragEventArgs e)
        {
            if (e.Data.GetDataPresent(System.Windows.DataFormats.FileDrop))
            {
                e.Effects = System.Windows.DragDropEffects.Link;
            }
            else
            {
                e.Effects = System.Windows.DragDropEffects.None;
            }
        }
    }
}
