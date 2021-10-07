using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Collections.Specialized;
using System.ComponentModel;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using NLog;
using Wox.Infrastructure.Logger;
using Wox.Infrastructure.UserSettings;
using Wox.Plugin;

namespace Wox.ViewModel
{
    public class ResultsViewModel : BaseModel
    {
        #region Private Fields

        public ResultCollection Results { get; }

        private readonly Settings _settings;
        private int MaxResults => _settings?.MaxResultsToShow ?? 6;
        private readonly object _collectionLock = new object();
        public volatile bool CollectionJustChanged = false;
        public volatile bool UserChangedIndex = false;
        public object _selectedIndexLock = new object();

        public ResultsViewModel()
        {
            Results = new ResultCollection();
            BindingOperations.EnableCollectionSynchronization(Results, _collectionLock);
            Results.CollectionChangedPrioritized += (token) =>
            {
                // Mark collection as changed, so that the selected item should be updated to the first item in ResultListBox.
                // When ResultListBox finished its update, CollectionJustChanged is checked. If it is true, set it to false, then update SelectedIndex to -1 (and then to 0).
                CollectionJustChanged = true;

                // Try Changing SelectedIndex, so that "SelectedIndex's TargetUpdated" aka SelectedIndex.set will always be invoked.
                Task.Delay(0).ContinueWith(___ =>
                {
                    SelectedIndex = -1;
                });                
            };
        }

        public ResultsViewModel(Settings settings) : this()
        {
            _settings = settings;
            _settings.PropertyChanged += (s, e) =>
            {
                if (e.PropertyName == nameof(_settings.MaxResultsToShow))
                {
                    OnPropertyChanged(nameof(MaxHeight));
                }
            };
        }

        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

        #endregion

        #region Properties

        public int MaxHeight => MaxResults * 50;

        private int _selectedIndex;
        public int SelectedIndex {
            get { return _selectedIndex; }
            set
            {
                lock (_selectedIndexLock)
                {
                    _selectedIndex = value;

                    if (CollectionJustChanged)
                    {
                        if (!UserChangedIndex)
                        {
                            Task.Delay(1).ContinueWith(___ =>
                            {
                                CollectionJustChanged = false;
                                SelectedIndex = 0;
                            });
                            Task.Delay(50).ContinueWith(___ =>
                            {
                                CollectionJustChanged = false; // Delay setting CollectionJustChanged = false, because SelectedIndex could change multiple times (but to the same value) after collection changed
                                SelectedIndex = 0;
                            });
                        }
                    }
                    else
                    {
                        if (_selectedIndex < 0)
                        {
                            SelectedIndex = 0;
                        }

                        if (value != -1 && value != 0)
                        {
                            UserChangedIndex = true;
                        }
                    }
                }
            }
        }

        public ResultViewModel SelectedItem { get; set; }
        public Thickness Margin { get; set; }
        public Visibility Visbility { get; set; } = Visibility.Collapsed;

        #endregion

        #region Private Methods

        private int NewIndex(int i)
        {
            var n = Results.Count;
            if (n > 0)
            {
                i = (n + i) % n;
                return i;
            }
            else
            {
                // SelectedIndex returns -1 if selection is empty.
                return -1;
            }
        }


        #endregion

        #region Public Methods

        public void SelectNextResult()
        {
            SelectedIndex = NewIndex(SelectedIndex + 1);
        }

        public void SelectPrevResult()
        {
            SelectedIndex = NewIndex(SelectedIndex - 1);
        }

        public void SelectNextPage()
        {
            SelectedIndex = NewIndex(SelectedIndex + MaxResults);
        }

        public void SelectPrevPage()
        {
            SelectedIndex = NewIndex(SelectedIndex - MaxResults);
        }

        public void SelectFirstResult()
        {
            SelectedIndex = NewIndex(0);
        }

        public void Clear()
        {
            lock (_collectionLock)
            {
                Results.RemoveAll();
            }
        }

        public int Count => Results.Count;

        public void AddResults(List<Result> newRawResults, string resultId)
        {
            CancellationToken token = new CancellationTokenSource().Token;
            List<ResultsForUpdate> updates = new List<ResultsForUpdate>()
            {
                new ResultsForUpdate(newRawResults, resultId, token)
            };
            AddResults(updates);
        }

        /// <summary>
        /// To avoid deadlock, this method should not called from main thread
        /// </summary>
        public void AddResults(List<ResultsForUpdate> updates)
        {
            var updatesNotCanceled = updates.Where(u => !u.Token.IsCancellationRequested);

            CancellationToken token;
            try
            {
                token = updatesNotCanceled.Select(u => u.Token).Distinct().First();
            }
            catch (InvalidOperationException)
            {
                // This is common, so WoxError -> WoxDebug
                Logger.WoxDebug("more than one not canceled query result in same batch processing"); // ?
                return;
            }


            // https://stackoverflow.com/questions/14336750
            lock (_collectionLock)
            {
                List<ResultViewModel> newResults = NewResults(updatesNotCanceled.ToList(), token);
                Logger.WoxTrace($"newResults {newResults.Count}");
                Results.Update(newResults, token);
            }

            if (Results.Count > 0)
            {
                Margin = new Thickness { Top = 8 };
            }
            else
            {
                Margin = new Thickness { Top = 0 };
            }
        }

        private List<ResultViewModel> NewResults(List<ResultsForUpdate> updates, CancellationToken token)
        {
            if (token.IsCancellationRequested) { return Results.ToList(); }
            var newResults = Results.ToList();
            if (updates.Count > 0)
            {
                if (token.IsCancellationRequested) { return Results.ToList(); }
                List<Result> resultsFromUpdates = updates.SelectMany(u => u.Results).ToList();

                if (token.IsCancellationRequested) { return Results.ToList(); }
                newResults.RemoveAll(r => updates.Any(u => u.ID == r.Result.PluginID));

                if (token.IsCancellationRequested) { return Results.ToList(); }
                IEnumerable<ResultViewModel> vm = resultsFromUpdates.Select(r => new ResultViewModel(r));
                newResults.AddRange(vm);

                if (token.IsCancellationRequested) { return Results.ToList(); }
                List<ResultViewModel> sorted = newResults.OrderByDescending(r => r.Result.Score).Take(MaxResults * 4).ToList();

                return sorted;
            }
            else
            {
                return Results.ToList();
            }
        }

        #endregion

        #region FormattedText Dependency Property
        public static readonly DependencyProperty FormattedTextProperty = DependencyProperty.RegisterAttached(
            "FormattedText",
            typeof(Inline),
            typeof(ResultsViewModel),
            new PropertyMetadata(null, FormattedTextPropertyChanged));

        public static void SetFormattedText(DependencyObject textBlock, IList<int> value)
        {
            textBlock.SetValue(FormattedTextProperty, value);
        }

        public static Inline GetFormattedText(DependencyObject textBlock)
        {
            return (Inline)textBlock.GetValue(FormattedTextProperty);
        }

        private static void FormattedTextPropertyChanged(DependencyObject d, DependencyPropertyChangedEventArgs e)
        {
            var textBlock = d as TextBlock;
            if (textBlock == null) return;

            var inline = (Inline)e.NewValue;

            textBlock.Inlines.Clear();
            if (inline == null) return;

            textBlock.Inlines.Add(inline);
        }
        #endregion

        public delegate void NotifyCollectionChangedPrioritizedEventHandler(CancellationToken token);
        public class ResultCollection : Collection<ResultViewModel>, INotifyCollectionChanged
        {
            public event NotifyCollectionChangedPrioritizedEventHandler CollectionChangedPrioritized;
            public event NotifyCollectionChangedEventHandler CollectionChanged;

            public void RemoveAll()
            {
                this.Clear();
                if (CollectionChanged != null)
                {
                    CollectionChanged.Invoke(this, new NotifyCollectionChangedEventArgs(NotifyCollectionChangedAction.Reset));
                }
            }

            public void Update(List<ResultViewModel> newItems, CancellationToken token)
            {
                if (token.IsCancellationRequested) { return; }

                this.Clear();
                foreach (var i in newItems)
                {
                    if (token.IsCancellationRequested) { break; }
                    this.Add(i);
                }
                if (CollectionChangedPrioritized != null)
                {
                    CollectionChangedPrioritized.Invoke(token);
                }
                if (CollectionChanged != null)
                {
                    // wpf use directx / double buffered already, so just reset all won't cause ui flickering
                    CollectionChanged.Invoke(this, new NotifyCollectionChangedEventArgs(NotifyCollectionChangedAction.Reset));
                }
            }
        }
    }
}
