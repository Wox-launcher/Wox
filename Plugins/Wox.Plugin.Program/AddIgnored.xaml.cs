using System.Collections.Generic;
using System.Windows;
using System.Windows.Forms;

namespace Wox.Plugin.Program
{
    public partial class AddIgnored
    {
        private IgnoredEntry _editing;
        private Settings _settings;

        public AddIgnored(Settings settings)
        {
            InitializeComponent();
            _settings = settings;
            IgnoredStringTextbox.Focus();
        }

        public AddIgnored(IgnoredEntry edit, Settings settings)
        {
            _editing = edit;
            _settings = settings;

            InitializeComponent();
            IgnoredStringTextbox.Text = _editing.EntryString;
            RegexCheckbox.IsChecked = _editing.IsRegex;
        }

        private void ButtonAdd_OnClick(object sender, RoutedEventArgs e)
        {
            if(_editing == null)
            {
                _settings.IgnoredSequence.Add(new IgnoredEntry()
                {
                    EntryString = IgnoredStringTextbox.Text,
                    IsRegex = RegexCheckbox.IsChecked.Value
                });
            }
            else
            {
                _settings.IgnoredSequence.Remove(_editing);
                _settings.IgnoredSequence.Add(new IgnoredEntry() {
                    EntryString = IgnoredStringTextbox.Text,
                    IsRegex = RegexCheckbox.IsChecked.Value
                });
            }
            Close();
        }
    }
}
