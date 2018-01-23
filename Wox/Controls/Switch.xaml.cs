using System;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Animation;
using Wox.Annotations;

namespace Wox.Controls
{
    /// <summary>
    /// Interaction logic for Switch.xaml
    /// </summary>
    public partial class Switch : UserControl, INotifyPropertyChanged
    {
        private DoubleAnimation SwitchOnAnimation { get; }
        private DoubleAnimation SwitchOffAnimation { get; }

        public Switch()
        {
            DataContext = this;

            //SwitchOnAnimation = new DoubleAnimation(
            //    (ActualWidth - BorderThickness.Left - BorderThickness.Right) / 2,
            //    new Duration(TimeSpan.FromMilliseconds(200)));
            //SwitchOffAnimation = new DoubleAnimation(
            //    -(ActualWidth - BorderThickness.Left - BorderThickness.Right) / 2,
            //    new Duration(TimeSpan.FromMilliseconds(200)));

            //var switchOn = new Storyboard();
            //switchOn.Children.Add(SwitchOnAnimation);
            //Resources.Add("SwitchOn", switchOn);

            //var switchOff = new Storyboard();
            //switchOff.Children.Add(SwitchOffAnimation);
            //Resources.Add("SwitchOff", switchOff);
            InitializeComponent();
        }

        public Brush BubbleBorderBrush
        {
            get { return (Brush)GetValue(BubbleBorderBrushProperty); }
            set { SetValue(BubbleBorderBrushProperty, value); }
        }

        public Brush BubbleFillBrush
        {
            get { return (Brush)GetValue(BubbleFillBrushProperty); }
            set { SetValue(BubbleFillBrushProperty, value); }
        }

        public bool Checked
        {
            get { return (bool)GetValue(CheckedProperty); }
            set { SetValue(CheckedProperty, value); }
        }

        public double BorderRadius => ActualHeight / 2;

        public double BubbleDiameter => ActualHeight - BorderThickness.Top - BorderThickness.Bottom;

        public static DependencyProperty BubbleBorderBrushProperty =
            DependencyProperty.Register(nameof(BubbleBorderBrush),
                typeof(Brush), typeof(Switch), new PropertyMetadata(new SolidColorBrush(Color.FromRgb(255, 255, 255))));

        public static DependencyProperty BubbleFillBrushProperty = DependencyProperty.Register(nameof(BubbleFillBrush),
            typeof(Brush),
            typeof(Switch), new PropertyMetadata(new SolidColorBrush(Color.FromRgb(100, 100, 100))));

        public static DependencyProperty CheckedProperty = DependencyProperty.Register(nameof(Checked), typeof(bool),
            typeof(Switch), new PropertyMetadata(false));

        public event PropertyChangedEventHandler PropertyChanged;

        [NotifyPropertyChangedInvocator]
        protected virtual void OnPropertyChanged([CallerMemberName] string propertyName = null)
        {
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
        }

        private void Switch_OnMouseUp(object sender, MouseButtonEventArgs e)
        {
            Checked = !Checked;
        }

        private void Switch_OnLoaded(object sender, RoutedEventArgs e)
        {
            //var switchOn = new Storyboard();
            //switchOn.Children.Add(new DoubleAnimation(
            //    (ActualWidth - BorderThickness.Left - BorderThickness.Right) / 2,
            //    new Duration(TimeSpan.FromMilliseconds(200))));
            //Resources.Add("SwitchOn", switchOn);

            //var switchOff = new Storyboard();
            //switchOff.Children.Add(new DoubleAnimation(
            //    -(ActualWidth - BorderThickness.Left - BorderThickness.Right) / 2,
            //    new Duration(TimeSpan.FromMilliseconds(200))));
            //Resources.Add("SwitchOff", switchOff);
        }

        private void Switch_OnSizeChanged(object sender, SizeChangedEventArgs e)
        {
            OnPropertyChanged(nameof(BubbleDiameter));
            OnPropertyChanged(nameof(BorderRadius));
        }
    }
}
