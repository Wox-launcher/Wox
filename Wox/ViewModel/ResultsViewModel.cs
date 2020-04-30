using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading;
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

        private ResultCollection Results { get; }

        private readonly object _collectionLock = new object();
        private readonly Settings _settings;
        private int MaxResults => _settings?.MaxResultsToShow ?? 6;

        public ResultsViewModel()
        {
            Results = new ResultCollection();
            BindingOperations.EnableCollectionSynchronization(Results, _collectionLock);
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

        public int SelectedIndex { get; set; }

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
            Results.RemoveAll();
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
                token = updates.Select(u => u.Token).Distinct().First();
            }
            catch (InvalidOperationException e)
            {
                Logger.WoxError("more than one not canceled query result in same batch processing", e);
                return;
            }

            List<ResultViewModel> newResults = NewResults(updates, token);
            Logger.WoxTrace($"newResults {newResults.Count}");

            Results.Update(newResults, token);

            if (Results.Count > 0)
            {
                Margin = new Thickness { Top = 8 };
                SelectedIndex = 0;
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

        public class ResultCollection : ObservableCollection<ResultViewModel>
        {

            public void RemoveAll()
            {
                this.Clear();
            }

            /// <summary>
            /// Update the results collection with new results, try to keep identical results
            /// </summary>
            /// <param name="newItems"></param>
            public void Update(List<ResultViewModel> newItems, System.Threading.CancellationToken token)
            {
                CheckReentrancy();
                if (token.IsCancellationRequested) { return; }

                int newCount = newItems.Count;
                int oldCount = Items.Count;
                int location = newCount > oldCount ? oldCount : newCount;

                for (int i = 0; i < location; i++)
                {
                    if (token.IsCancellationRequested) { return; }

                    ResultViewModel oldResult = this[i];
                    ResultViewModel newResult = newItems[i];
                    Logger.WoxTrace(
                        $"index {i} " +
                              $"old<{oldResult.Result.Title} {oldResult.Result.Score}> " +
                              $"new<{newResult.Result.Title} {newResult.Result.Score}>"
                        );
                    if (oldResult.Equals(newResult))
                    {
                        Logger.WoxTrace($"index <{i}> equal");
                        // update following info no matter they are equal or not
                        // because check equality will cause more computation
                        this[i].Result.Score = newResult.Result.Score;
                        this[i].Result.TitleHighlightData = newResult.Result.TitleHighlightData;
                        this[i].Result.SubTitleHighlightData = newResult.Result.SubTitleHighlightData;
                    }
                    else
                    {
                        // result is not the same update it in the current index
                        this[i] = newResult;
                        Logger.WoxTrace($"index <{i}> not equal old<{oldResult.GetHashCode()}> new<{newResult.GetHashCode()}>");
                    }
                }


                if (newCount >= oldCount)
                {
                    for (int i = oldCount; i < newCount; i++)
                    {
                        if (token.IsCancellationRequested) { return; }

                        Logger.WoxTrace($"add {i} new<{newItems[i].Result.Title}");
                        Add(newItems[i]);
                    }
                }
                else
                {
                    for (int i = oldCount - 1; i >= newCount; i--)
                    {
                        if (token.IsCancellationRequested) { return; }

                        RemoveAt(i);
                    }
                }
            }
        }
    }
}