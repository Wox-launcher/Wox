using System.Collections.Generic;
using System.Linq;
using System.Runtime.Remoting.Contexts;
using System.Windows;
using System.Windows.Controls;
using Wox.Plugin;
using Wox.ViewModel;

namespace Wox
{
    [Synchronization]
    public partial class ResultListBox
    {
        public void AddResults(List<Result> newRawResults)
        {
            var vm = DataContext as ResultsViewModel;
            var newResults = newRawResults.Select(r => new ResultViewModel(r)).ToList();
            vm.Results.Update(newResults);
            vm.SelectedIndex = 0;
        }
        

        public ResultListBox()
        {
            InitializeComponent();
        }

        private void OnSelectionChanged(object sender, SelectionChangedEventArgs e)
        {
            if (e.AddedItems.Count > 0 && e.AddedItems[0] != null)
            {
                ScrollIntoView(e.AddedItems[0]);
            }
        }
    }
}