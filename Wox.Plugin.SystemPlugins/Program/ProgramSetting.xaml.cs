﻿using System;
using System.Threading;
using System.Windows;
using System.Windows.Controls;
using Wox.Infrastructure.Storage.UserSettings;

namespace Wox.Plugin.SystemPlugins.Program
{
    /// <summary>
    /// Interaction logic for ProgramSetting.xaml
    /// </summary>
    public partial class ProgramSetting : UserControl
    {
        public ProgramSetting()
        {
            InitializeComponent();
            Loaded += Setting_Loaded;
        }

        private void Setting_Loaded(object sender, RoutedEventArgs e)
        {
            programSourceView.ItemsSource = UserSettingStorage.Instance.ProgramSources;
        }

        private void ReIndexing()
        {
            programSourceView.Items.Refresh();
            ThreadPool.QueueUserWorkItem(t =>
            {
                Dispatcher.Invoke(new Action(() => { indexingPanel.Visibility = Visibility.Visible; }));
                Programs.IndexPrograms();
                Dispatcher.Invoke(new Action(() => { indexingPanel.Visibility = Visibility.Hidden; }));
            });
        }

        private void btnAddProgramSource_OnClick(object sender, RoutedEventArgs e)
        {
            var folderBrowserDialog = new System.Windows.Forms.FolderBrowserDialog();
            if (folderBrowserDialog.ShowDialog() == System.Windows.Forms.DialogResult.OK)
            {
                string path = folderBrowserDialog.SelectedPath;

                UserSettingStorage.Instance.ProgramSources.Add(new ProgramSource()
                {
                    Location = path,
                    Type = "FileSystemProgramSource",
                    Enabled = true
                });
                UserSettingStorage.Instance.Save();
                ReIndexing();
            }
        }

        private void btnDeleteProgramSource_OnClick(object sender, RoutedEventArgs e)
        {
            ProgramSource selectedProgramSource = programSourceView.SelectedItem as ProgramSource;
            if (selectedProgramSource != null)
            {
                if (MessageBox.Show("Are your sure to delete " + selectedProgramSource.Location, "Delete ProgramSource",
                    MessageBoxButton.YesNo) == MessageBoxResult.Yes)
                {
                    UserSettingStorage.Instance.ProgramSources.Remove(selectedProgramSource);
                    UserSettingStorage.Instance.Save();
                    ReIndexing();
                }
            }
            else
            {
                MessageBox.Show("Please select a program source");
            }
        }

        private void btnEditProgramSource_OnClick(object sender, RoutedEventArgs e)
        {
            ProgramSource selectedProgramSource = programSourceView.SelectedItem as ProgramSource;
            if (selectedProgramSource != null)
            {
                //todo: update
                var folderBrowserDialog = new System.Windows.Forms.FolderBrowserDialog();
                if (folderBrowserDialog.ShowDialog() == System.Windows.Forms.DialogResult.OK)
                {
                    string path = folderBrowserDialog.SelectedPath;
                    selectedProgramSource.Location = path;
                    UserSettingStorage.Instance.Save();
                    ReIndexing();
                }
            }
            else
            {
                MessageBox.Show("Please select a program source");
            }
        }

        private void btnReindex_Click(object sender, RoutedEventArgs e)
        {
            ReIndexing();
        }

        private void BtnProgramSuffixes_OnClick(object sender, RoutedEventArgs e)
        {
            
            ProgramSuffixes p = new ProgramSuffixes();
            p.ShowDialog();
        }

        private void programSourceView_DragEnter(object sender, DragEventArgs e)
        {
            if (e.Data.GetDataPresent(DataFormats.FileDrop))
            {
                e.Effects = DragDropEffects.Link;
            }
            else
            {
                e.Effects = DragDropEffects.None;
            }
        }

        private void programSourceView_Drop(object sender, DragEventArgs e)
        {
            string[] files = (string[])e.Data.GetData(DataFormats.FileDrop);

            if (files != null && files.Length > 0)
            {
                foreach (string s in files)
                {
                    if (System.IO.Directory.Exists(s) == true)
                    {
                        UserSettingStorage.Instance.ProgramSources.Add(new ProgramSource()
                        {
                            Location = s,
                            Type = "FileSystemProgramSource",
                            Enabled = true
                        });

                        UserSettingStorage.Instance.Save();
                        ReIndexing();
                    }
                }
            }
        }
    }
}