using System;
using System.IO;
using System.Windows;
using System.Windows.Forms;
using System.Windows.Input;
using System.Windows.Media.Animation;
using Wox.Helper;
using Wox.Infrastructure;
using Wox.Infrastructure.Image;

namespace Wox
{
    public partial class Msg : Window
    {
        Storyboard fadeOutStoryboard = new Storyboard();
        private bool closing;

        public Msg()
        {
            InitializeComponent();
            var screen = Screen.FromPoint(System.Windows.Forms.Cursor.Position);
            var dipWorkingArea = WindowIntelopHelper.TransformPixelsToDIP(this,
                screen.WorkingArea.Width,
                screen.WorkingArea.Height);
            Left = dipWorkingArea.X - Width;
            Top = dipWorkingArea.Y;
            showAnimation.From = dipWorkingArea.Y;
            showAnimation.To = dipWorkingArea.Y - Height;

            // Create the fade out storyboard
            fadeOutStoryboard.Completed += fadeOutStoryboard_Completed;
            DoubleAnimation fadeOutAnimation = new DoubleAnimation(dipWorkingArea.Y - Height, dipWorkingArea.Y, new Duration(TimeSpan.FromSeconds(1)))
            {
                AccelerationRatio = 0.2
            };
            Storyboard.SetTarget(fadeOutAnimation, this);
            Storyboard.SetTargetProperty(fadeOutAnimation, new PropertyPath(TopProperty));
            fadeOutStoryboard.Children.Add(fadeOutAnimation);

            imgClose.Source = ImageLoader.Load(Path.Combine(Constant.ProgramDirectory, "Images\\close.png"));
            imgClose.MouseUp += imgClose_MouseUp;
        }

        void imgClose_MouseUp(object sender, MouseButtonEventArgs e)
        {
            if (!closing)
            {
                closing = true;
                fadeOutStoryboard.Begin();
            }
        }

        private void fadeOutStoryboard_Completed(object sender, EventArgs e)
        {
            Close();
        }

        public void Show(string title, string subTitle, string iconPath)
        {
            tbTitle.Text = title;
            tbSubTitle.Text = subTitle;
            if (string.IsNullOrEmpty(subTitle))
            {
                tbSubTitle.Visibility = Visibility.Collapsed;
            }
            imgIco.Source = ImageLoader.Load(iconPath);

            Show();

            if (!closing)
            {
                closing = true;
                fadeOutStoryboard.Begin();
            }
        }
    }
}
