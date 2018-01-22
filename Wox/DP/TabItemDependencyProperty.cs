using System;
using System.Windows;

namespace Wox.DP
{
    public class TabItemDependencyProperty
    {
        public static DependencyProperty TabItemIconProperty = DependencyProperty.RegisterAttached("Icon", typeof(Uri),
            typeof(TabItemDependencyProperty));

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