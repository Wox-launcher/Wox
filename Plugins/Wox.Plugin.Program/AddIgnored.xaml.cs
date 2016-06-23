using System.Collections.Generic;
using System.Windows;
using System.Windows.Forms;

namespace Wox.Plugin.Program
{
    public partial class AddIgnored
    {
        private string _editing;
        private Settings _settings;

        public AddIgnored(Settings settings)
        {
            InitializeComponent();
            _settings = settings;
            Ignored.Focus();
        }

        public AddIgnored(string edit, Settings settings)
        {
            _editing = edit;
            _settings = settings;

            InitializeComponent();
            Ignored.Text = _editing;
        }

        private void ButtonAdd_OnClick(object sender, RoutedEventArgs e)
        {
            if(_editing == null)
            {
                _settings.IgnoredSequence.Add(Ignored.Text);
            }
            else
            {
                _settings.IgnoredSequence.Remove(_editing);
                _settings.IgnoredSequence.Add(Ignored.Text);
            }
            Close();
        }
    }
}
