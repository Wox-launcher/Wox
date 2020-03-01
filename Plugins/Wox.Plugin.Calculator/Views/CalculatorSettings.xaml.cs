using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
using System.Windows.Shapes;
using Wox.Plugin.Caculator.ViewModels;

namespace Wox.Plugin.Caculator.Views
{
    /// <summary>
    /// Interaction logic for CalculatorSettings.xaml
    /// </summary>
    public partial class CalculatorSettings : UserControl
    {
        private readonly SettingsViewModel _viewModel;
        private readonly Settings _settings;

        public CalculatorSettings(SettingsViewModel viewModel)
        {
            _viewModel = viewModel;
            _settings = viewModel.Settings;
            DataContext = viewModel;
            InitializeComponent();
        }

        private void CalculatorSettings_Loaded(object sender, RoutedEventArgs e)
        {
            DecimalSeparatorComboBox.SelectedItem = _settings.DecimalSeparator;
            MaxDecimalPlaces.SelectedItem = _settings.MaxDecimalPlaces;
        }
    }

    
}
