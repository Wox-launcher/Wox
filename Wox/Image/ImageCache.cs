using JetBrains.Annotations;
using NLog;
using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using System.Windows.Media;

using Wox.Helper;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;

namespace Wox.Image
{
    class CacheEntry
    {
        internal string Key;
        internal ImageSource Image;
        internal DateTime ExpiredDate;

        public CacheEntry(string key, ImageSource image, DateTime expiredTime)
        {
            Key = key;
            Image = image;
            ExpiredDate = expiredTime;
        }
    }

    class UpdateCallbackEntry
    {
        internal string Key;
        internal Func<string, ImageSource> ImageFactory;
        internal Action<ImageSource> UpdateImageCallback;

        public UpdateCallbackEntry(string key, Func<string, ImageSource> imageFactory, Action<ImageSource> updateImageCallback)
        {
            Key = key;
            ImageFactory = imageFactory;
            UpdateImageCallback = updateImageCallback;
        }
    }

    class ImageCache
    {
        private readonly TimeSpan _expiredTime = new TimeSpan(24, 0, 0);
        private readonly TimeSpan _checkInterval = new TimeSpan(1, 0, 0);
        private const int _cacheLimit = 500;

        private readonly ConcurrentDictionary<string, CacheEntry> _cache;
        BlockingCollection<CacheEntry> _cacheQueue;
        private readonly SortedSet<CacheEntry> _cacheSorted;
        BlockingCollection<UpdateCallbackEntry> _updateQueue;

        private readonly System.Threading.Timer timer;
        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public ImageCache()
        {
            _cache = new ConcurrentDictionary<string, CacheEntry>();
            _cacheSorted = new SortedSet<CacheEntry>(new CacheEntryComparer());
            _cacheQueue = new BlockingCollection<CacheEntry>();
            _updateQueue = new BlockingCollection<UpdateCallbackEntry>();

            timer = new System.Threading.Timer(ExpirationCheck, null, _checkInterval, _checkInterval);
            Task.Run(() =>
            {
                while (true)
                {
                    CacheEntry entry = _cacheQueue.Take();
                    int currentCount = _cache.Count;
                    if (currentCount > _cacheLimit)
                    {
                        CacheEntry min = _cacheSorted.Min;
                        _cacheSorted.Remove(min);
                        bool removeResult = _cache.TryRemove(min.Key, out _);
                        Logger.WoxDebug($"remove exceed: <{removeResult}> entry: <{min.Key}>");
                    }
                    else
                    {
                        _cacheSorted.Remove(entry);
                    }
                    _cacheSorted.Add(entry);
                }
            }).ContinueWith(ErrorReporting.UnhandledExceptionHandleTask, TaskContinuationOptions.OnlyOnFaulted);
            Task.Run(() =>
            {
                while (true)
                {
                    UpdateCallbackEntry entry = _updateQueue.Take();
                    CacheEntry addEntry = Add(entry.Key, entry.ImageFactory);
                    entry.UpdateImageCallback(addEntry.Image);
                }
            }).ContinueWith(ErrorReporting.UnhandledExceptionHandleTask, TaskContinuationOptions.OnlyOnFaulted);
        }

        private void ExpirationCheck(object state)
        {
            try
            {
                DateTime now = DateTime.Now;
                Logger.WoxDebug($"ExpirationCheck start {now}");
                List<KeyValuePair<string, CacheEntry>> pairs = _cache.Where(pair => now > pair.Value.ExpiredDate).ToList();

                foreach (KeyValuePair<string, CacheEntry> pair in pairs)
                {
                    bool success = _cache.TryRemove(pair.Key, out CacheEntry entry);
                    Logger.WoxDebug($"remove expired: <{success}> entry: <{pair.Key}>");
                }
            }
            catch (Exception e)
            {
                e.Data.Add(nameof(state), state);
                Logger.WoxError($"error check image cache with state: {state}", e);
            }
        }


        /// <summary>
        /// Not thread safe, should be only called from ui thread
        /// </summary>
        /// <param name="key"></param>
        /// <returns></returns>
        public ImageSource GetOrAdd([NotNull] string key, Func<string, ImageSource> imageFactory)
        {
            key.RequireNonNull();
            CacheEntry entry;
            bool getResult = _cache.TryGetValue(key, out entry);
            if (!getResult)
            {
                entry = Add(key, imageFactory);
                return entry.Image;
            }
            else
            {
                UpdateDate(entry);
                return entry.Image;
            }
        }

        public ImageSource GetOrAdd([NotNull] string key, ImageSource defaultImage, Func<string, ImageSource> imageFactory, Action<ImageSource> updateImageCallback)
        {
            key.RequireNonNull();
            CacheEntry getEntry;
            bool getResult = _cache.TryGetValue(key, out getEntry);
            if (!getResult)
            {
                _updateQueue.Add(new UpdateCallbackEntry(key, imageFactory, updateImageCallback));
                return defaultImage;
            }
            else
            {
                UpdateDate(getEntry);
                return getEntry.Image;
            }
        }

        private CacheEntry Add(string key, Func<string, ImageSource> imageFactory)
        {
            CacheEntry entry;
            ImageSource image = imageFactory(key);
            entry = new CacheEntry(key, image, DateTime.Now + _expiredTime);
            _cache[key] = entry;
            _cacheQueue.Add(entry);
            return entry;
        }

        private void UpdateDate(CacheEntry entry)
        {
            entry.ExpiredDate = DateTime.Now + _expiredTime;
            _cacheQueue.Add(entry);
        }
    }

    internal class CacheEntryComparer : IComparer<CacheEntry>
    {
        public int Compare(CacheEntry x, CacheEntry y)
        {
            return x.ExpiredDate.CompareTo(y.ExpiredDate);
        }
    }
}
