using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;

namespace Wox.Infrastructure.UI
{
    public static class ShowUpWoxHelper
    {
        public static void ShowUpWox()
        {
            Window mainWindow = Application.Current.MainWindow;

            try
            {
                dynamic woxMainWindow = mainWindow;
                woxMainWindow.ShowUpWox();
            }
            catch (System.Exception) {
                mainWindow.Visibility = Visibility.Visible;
            }
            
        }

        public static void HideWox()
        {
            Window mainWindow = Application.Current.MainWindow;
            mainWindow.Visibility = Visibility.Hidden;
        }
    }
}
