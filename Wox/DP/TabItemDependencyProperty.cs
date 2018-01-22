using System;
using System.Windows;
using System.Windows.Controls;

namespace Wox.DP
{
    public class TabItemDependencyProperty
    {
        public static DependencyProperty TabItemIconProperty = DependencyProperty.RegisterAttached("TabItemIcon", typeof(Uri),
            typeof(TabItem));

        public static Uri GetTabItemIcon(DependencyObject obj)
        {
            return (Uri)obj.GetValue(TabItemIconProperty);
        }

        public static void SetTabItemIcon(DependencyObject obj, Uri value)
        {
            obj.SetValue(TabItemIconProperty, value);
        }
    }
}