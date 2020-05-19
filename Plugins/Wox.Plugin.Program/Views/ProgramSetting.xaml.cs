using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using Wox.Plugin.Program.Programs;
using System.ComponentModel;
using System.Windows.Data;

namespace Wox.Plugin.Program.Views
{
    /// <summary>
    /// Interaction logic for ProgramSetting.xaml
    /// </summary>
    public partial class ProgramSetting : UserControl
    {
        private PluginInitContext context;
        private Settings _settings;
        private GridViewColumnHeader _lastHeaderClicked;
        private ListSortDirection _lastDirection;

        public ProgramSetting(PluginInitContext context, Settings settings, Win32[] win32s, UWP.Application[] uwps)
        {
            this.context = context;
            InitializeComponent();
            Loaded += Setting_Loaded;
            _settings = settings;
        }

        private void Setting_Loaded(object sender, RoutedEventArgs e)
        {
            programSourceView.ItemsSource = _settings.ProgramSources;
            programIgnoreView.ItemsSource = _settings.IgnoredSequence;
            StartMenuEnabled.IsChecked = _settings.EnableStartMenuSource;
            RegistryEnabled.IsChecked = _settings.EnableRegistrySource;
        }

        private void ReIndexing()
        {
            programSourceView.Items.Refresh();
            Task.Run(() =>
            {
                Dispatcher.Invoke(() => { indexingPanel.Visibility = Visibility.Visible; });
                Main.IndexPrograms();
                Dispatcher.Invoke(() => { indexingPanel.Visibility = Visibility.Hidden; });
            });
        }

        private void btnAddProgramSource_OnClick(object sender, RoutedEventArgs e)
        {
            var add = new AddProgramSource(context, _settings);
            if (add.ShowDialog() ?? false)
            {
                ReIndexing();
            }
        }

        private void btnEditProgramSource_OnClick(object sender, RoutedEventArgs e)
        {
            var selectedProgramSource = programSourceView.SelectedItem as ProgramSource;
            if (selectedProgramSource != null)
            {
                var add = new AddProgramSource(selectedProgramSource, _settings);
                if (add.ShowDialog() ?? false)
                {
                    ReIndexing();
                }
            }
            else
            {
                string msg = context.API.GetTranslation("wox_plugin_program_pls_select_program_source");
                MessageBox.Show(msg);
            }
        }

        private void btnReindex_Click(object sender, RoutedEventArgs e)
        {
            ReIndexing();
        }

        private void BtnProgramSuffixes_OnClick(object sender, RoutedEventArgs e)
        {
            var p = new ProgramSuffixes(context, _settings);
            if (p.ShowDialog() ?? false)
            {
                ReIndexing();
            }
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
            var directories = (string[])e.Data.GetData(DataFormats.FileDrop);

            var directoriesToAdd = new List<ProgramSource>();

            if (directories != null && directories.Length > 0)
            {
                foreach (string directory in directories)
                {
                    if (Directory.Exists(directory))
                    {
                        var source = new ProgramSource
                        {
                            Location = directory,
                        };

                        directoriesToAdd.Add(source);
                    }
                }

                if (directoriesToAdd.Count() > 0)
                {
                    directoriesToAdd.ForEach(x => _settings.ProgramSources.Add(x));
                    ReIndexing();
                }
            }
        }

        private void StartMenuEnabled_Click(object sender, RoutedEventArgs e)
        {
            _settings.EnableStartMenuSource = StartMenuEnabled.IsChecked ?? false;
            ReIndexing();
        }

        private void RegistryEnabled_Click(object sender, RoutedEventArgs e)
        {
            _settings.EnableRegistrySource = RegistryEnabled.IsChecked ?? false;
            ReIndexing();
        }

        private void btnProgramSoureDelete_OnClick(object sender, RoutedEventArgs e)
        {
            var selectedItems = programSourceView
                                .SelectedItems.Cast<ProgramSource>()
                                .ToList();

            if (selectedItems.Count() == 0)
            {
                string msg = context.API.GetTranslation("wox_plugin_program_pls_select_program_source");
                MessageBox.Show(msg);
                return;
            }
            else
            {
                _settings.ProgramSources.RemoveAll(s => selectedItems.Contains(s));
                programSourceView.SelectedItems.Clear();
                ReIndexing();
            }
        }

        private void ProgramSourceView_PreviewMouseRightButtonUp(object sender, MouseButtonEventArgs e)
        {
            programSourceView.SelectedItems.Clear();
        }

        private void GridViewColumnHeaderClickedHandler(object sender, RoutedEventArgs e)
        {
            var headerClicked = e.OriginalSource as GridViewColumnHeader;
            ListSortDirection direction;

            if (headerClicked != null)
            {
                if (headerClicked.Role != GridViewColumnHeaderRole.Padding)
                {
                    if (headerClicked != _lastHeaderClicked)
                    {
                        direction = ListSortDirection.Ascending;
                    }
                    else
                    {
                        if (_lastDirection == ListSortDirection.Ascending)
                        {
                            direction = ListSortDirection.Descending;
                        }
                        else
                        {
                            direction = ListSortDirection.Ascending;
                        }
                    }

                    var columnBinding = headerClicked.Column.DisplayMemberBinding as Binding;
                    var sortBy = columnBinding?.Path.Path ?? headerClicked.Column.Header as string;

                    Sort(sortBy, direction);

                    _lastHeaderClicked = headerClicked;
                    _lastDirection = direction;
                }
            }
        }

        private void Sort(string sortBy, ListSortDirection direction)
        {
            var dataView = CollectionViewSource.GetDefaultView(programSourceView.ItemsSource);

            dataView.SortDescriptions.Clear();
            SortDescription sd = new SortDescription(sortBy, direction);
            dataView.SortDescriptions.Add(sd);
            dataView.Refresh();
        }

        private void btnDeleteIgnored_OnClick(object sender, RoutedEventArgs e)
        {
            IgnoredEntry selectedIgnoredEntry = programIgnoreView.SelectedItem as IgnoredEntry;
            if (selectedIgnoredEntry != null)
            {
                string msg = string.Format(context.API.GetTranslation("wox_plugin_program_delete_ignored"), selectedIgnoredEntry);

                if (MessageBox.Show(msg, string.Empty, MessageBoxButton.YesNo) == MessageBoxResult.Yes)
                {
                    _settings.IgnoredSequence.Remove(selectedIgnoredEntry);
                    programIgnoreView.Items.Refresh();
                }
            }
            else
            {
                string msg = context.API.GetTranslation("wox_plugin_program_pls_select_ignored");
                MessageBox.Show(msg);
            }
        }

        private void btnEditIgnored_OnClick(object sender, RoutedEventArgs e)
        {
            IgnoredEntry selectedIgnoredEntry = programIgnoreView.SelectedItem as IgnoredEntry;
            if (selectedIgnoredEntry != null)
            {
                new AddIgnored(selectedIgnoredEntry, _settings).ShowDialog();
                programIgnoreView.Items.Refresh();
            }
            else
            {
                string msg = context.API.GetTranslation("wox_plugin_program_pls_select_ignored");
                MessageBox.Show(msg);
            }
        }
        private void btnAddIgnored_OnClick(object sender, RoutedEventArgs e)
        {
            new AddIgnored(_settings).ShowDialog();
            programIgnoreView.Items.Refresh();
        }
    }
}