using System;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using Wox.Annotations;

namespace Wox.Controls
{
    /// <summary>
    /// Interaction logic for Switch.xaml
    /// </summary>
    public partial class Switch : UserControl, INotifyPropertyChanged
    {
        public Switch()
        {
            InitializeComponent();
            if (Content is FrameworkElement first)
                first.DataContext = this;
        }

        public Brush BubbleBackground
        {
            get { return (Brush)GetValue(BubbleBackgroundProperty); }
            set { SetValue(BubbleBackgroundProperty, value); }
        }

        public bool IsChecked
        {
            get { return (bool)GetValue(IsCheckedProperty); }
            set
            {
                SetValue(IsCheckedProperty, value);
                OnPropertyChanged(nameof(IsChecked));
                if (value)
                    Checked?.Invoke(this, EventArgs.Empty);
                else
                    Unchecked?.Invoke(this, EventArgs.Empty);
            }
        }

        public double BorderRadius => ActualHeight / 2;

        public double BubbleDiameter => ActualHeight - BorderThickness.Top - BorderThickness.Bottom;

        public double BubbleLeft => ActualWidth / 2 - BubbleDiameter / 2;

        public double BubbleTop => ActualHeight / 2 - BubbleDiameter / 2;

        public double TranslateX => (ActualWidth - BubbleDiameter - BorderThickness.Left - BorderThickness.Right) / 2;

        public static readonly DependencyProperty BubbleBackgroundProperty =
            DependencyProperty.Register(nameof(BubbleBackground), typeof(Brush), typeof(Switch),
                new PropertyMetadata(new SolidColorBrush(Color.FromRgb(255, 255, 255))));

        public static readonly DependencyProperty IsCheckedProperty =
            DependencyProperty.Register(nameof(IsChecked), typeof(bool), typeof(Switch),
                new PropertyMetadata(false));

        public event EventHandler Checked;

        public event EventHandler Unchecked;

        public event PropertyChangedEventHandler PropertyChanged;

        [NotifyPropertyChangedInvocator]
        protected virtual void OnPropertyChanged([CallerMemberName] string propertyName = null)
        {
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
        }

        private void Switch_OnMouseUp(object sender, MouseButtonEventArgs e)
        {
            IsChecked = !IsChecked;
        }

        private void Switch_OnSizeChanged(object sender, SizeChangedEventArgs e)
        {
            OnPropertyChanged(nameof(BubbleDiameter));
            OnPropertyChanged(nameof(BorderRadius));
            OnPropertyChanged(nameof(TranslateX));
            OnPropertyChanged(nameof(BubbleTop));
            OnPropertyChanged(nameof(BubbleLeft));
        }
    }
}
