using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
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

        private readonly object _addResultsLock = new object();
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

        private int InsertIndexOf(int newScore, IList<ResultViewModel> list)
        {
            int index = 0;
            for (; index < list.Count; index++)
            {
                var result = list[index];
                if (newScore > result.Result.Score)
                {
                    break;
                }
            }
            return index;
        }

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
            Results.Clear();
        }

        public void RemoveResultsExcept(PluginMetadata metadata)
        {
            Results.RemoveAll(r => r.Result.PluginID != metadata.ID);
        }

        public void RemoveResultsFor(PluginMetadata metadata)
        {
            Results.RemoveAll(r => r.Result.PluginID == metadata.ID);
        }

        /// <summary>
        /// To avoid deadlock, this method should not called from main thread
        /// </summary>
        public void AddResults(List<Result> newRawResults, string resultId)
        {

            lock (_addResultsLock)
            {
                var newResults = NewResults(newRawResults, resultId);
                // update UI in one run, so it can avoid UI flickering
                Results.Update(newResults);

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
        }

        private List<ResultViewModel> NewResults(List<Result> newRawResults, string resultId)
        {
            var newResults = newRawResults.Select(r => new ResultViewModel(r)).ToList();
            var results = Results.ToList();
            var oldResults = results.Where(r => r.Result.PluginID == resultId).ToList();

            // intersection of A (old results) and B (new newResults)
            var intersection = oldResults.Intersect(newResults).ToList();

            // remove result of relative complement of B in A
            foreach (var result in oldResults.Except(intersection))
            {
                results.Remove(result);
            }

            // update index for result in intersection of A and B
            foreach (var commonResult in intersection)
            {
                int oldIndex = results.IndexOf(commonResult);
                int oldScore = results[oldIndex].Result.Score;
                var newResult = newResults[newResults.IndexOf(commonResult)];
                int newScore = newResult.Result.Score;
                if (newScore != oldScore)
                {
                    var oldResult = results[oldIndex];

                    oldResult.Result.Score = newScore;
                    oldResult.Result.OriginQuery = newResult.Result.OriginQuery;
                    oldResult.Result.TitleHighlightData = newResult.Result.TitleHighlightData;
                    oldResult.Result.SubTitleHighlightData = newResult.Result.SubTitleHighlightData;

                    results.RemoveAt(oldIndex);
                    int newIndex = InsertIndexOf(newScore, results);
                    results.Insert(newIndex, oldResult);
                }
            }

            int maxResults = _settings.MaxResultsToShow * 5;
            // insert result in relative complement of A in B
            foreach (var result in newResults.Except(intersection))
            {
                if (results.Count <= maxResults)
                {
                    int newIndex = InsertIndexOf(result.Result.Score, results);
                    results.Insert(newIndex, result);
                }
                else
                {
                    break;
                }
            }

            if (results.Count > maxResults)
            {
                var resultsCopy = results.GetRange(0, maxResults);
                return resultsCopy;
            }
            else
            {
                return results;
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

            public void RemoveAll(Predicate<ResultViewModel> predicate)
            {
                CheckReentrancy();

                for (int i = Count - 1; i >= 0; i--)
                {
                    if (predicate(this[i]))
                    {
                        RemoveAt(i);
                    }
                }
            }

            /// <summary>
            /// Update the results collection with new results, try to keep identical results
            /// </summary>
            /// <param name="newItems"></param>
            public void Update(List<ResultViewModel> newItems)
            {
                CheckReentrancy();

                int newCount = newItems.Count;
                int oldCount = Items.Count;
                int location = newCount > oldCount ? oldCount : newCount;

                for (int i = 0; i < location; i++)
                {
                    ResultViewModel oldResult = this[i];
                    ResultViewModel newResult = newItems[i];
                    Logger.WoxDebug(
                        $"index {i} " +
                              $"old<{oldResult.Result.Title} {oldResult.Result.Score}> " +
                              $"new<{newResult.Result.Title} {newResult.Result.Score}>"
                        );
                    if (oldResult.Equals(newResult))
                    {
                        Logger.WoxDebug($"index <{i}> equal");
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
                        Logger.WoxDebug($"index <{i}> not equal old<{oldResult.GetHashCode()}> new<{newResult.GetHashCode()}>");
                    }
                }


                if (newCount >= oldCount)
                {
                    for (int i = oldCount; i < newCount; i++)
                    {
                        Logger.WoxDebug($"Logger.WoxErroradd index {i} new<{newItems[i].Result.Title}");
                        Add(newItems[i]);
                    }
                }
                else
                {
                    for (int i = oldCount - 1; i >= newCount; i--)
                    {
                        RemoveAt(i);
                    }
                }
            }
        }
    }
}