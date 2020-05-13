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

        public CacheEntry(string key, ImageSource image)
        {
            Key = key;
            Image = image;
            ExpiredDate = DateTime.Now;
        }
    }

    class ImageCache
    {
        private readonly TimeSpan _expiredTime = new TimeSpan(24, 0, 0);
        private readonly TimeSpan _checkInterval = new TimeSpan(0, 1, 0);
        private const int _cacheLimit = 100;
        private readonly object _addLock = new object();

        private readonly ConcurrentDictionary<String, CacheEntry> _cache;
        private readonly SortedSet<CacheEntry> _cacheSorted;
        private readonly Func<string, ImageSource> _imageFactory;

        private readonly System.Threading.Timer timer;
        private static readonly NLog.Logger Logger = LogManager.GetCurrentClassLogger();

        public ImageCache(Func<string, ImageSource> imageFactory)
        {
            _imageFactory = imageFactory;
            _cache = new ConcurrentDictionary<string, CacheEntry>();
            _cacheSorted = new SortedSet<CacheEntry>(new CacheEntryComparer());
            timer = new System.Threading.Timer(ExpirationCheck, null, _checkInterval, _checkInterval);
        }

        private void ExpirationCheck(object state)
        {
            DateTime now = DateTime.Now;
            Logger.WoxDebug($"ExpirationCheck start {now}");
            List<KeyValuePair<string, CacheEntry>> pairs = _cache.Where(pair => now > pair.Value.ExpiredDate).ToList();

            foreach (KeyValuePair<string, CacheEntry> pair in pairs)
            {
                bool success = _cache.TryRemove(pair.Key, out CacheEntry entry);
                Logger.WoxDebug($"remove expired: <{success}> entry: <{pair.Key}>");
                throw new System.Exception("test timer exception caught");
            }
        }


        /// <summary>
        /// Not thread safe, should be only called from ui thread
        /// </summary>
        /// <param name="key"></param>
        /// <returns></returns>
        public ImageSource GetOrAdd([NotNull] string key)
        {
            key.RequireNonNull();
            CacheEntry entry;
            bool getResult = _cache.TryGetValue(key, out entry);
            if (!getResult)
            {
                entry = Add(key);
            }
            entry.ExpiredDate = DateTime.Now + _expiredTime;
            return entry.Image;
        }

        public ImageSource GetOrAdd([NotNull] string key, ImageSource defaultImage, Action<ImageSource> updateImageCallback)
        {
            key.RequireNonNull();
            CacheEntry getEntry;
            bool getResult = _cache.TryGetValue(key, out getEntry);
            if (!getResult)
            {
                var t = Task.Run(() =>
                {
                    CacheEntry addEntry = Add(key);
                    addEntry.ExpiredDate = DateTime.Now + _expiredTime;
                    updateImageCallback(addEntry.Image);
                }).ContinueWith(ErrorReporting.UnhandledExceptionHandleTask, TaskContinuationOptions.OnlyOnFaulted);
                return defaultImage;
            }
            else
            {
                getEntry.ExpiredDate = DateTime.Now + _expiredTime;
                return getEntry.Image;
            }
        }

        private CacheEntry Add(string key)
        {
            lock (_addLock)
            {
                CacheEntry entry;
                ImageSource image = _imageFactory(key);
                entry = new CacheEntry(key, image);
                _cache[key] = entry;
                _cacheSorted.Add(entry);

                int currentCount = _cache.Count;
                if (currentCount > _cacheLimit)
                {
                    CacheEntry min = _cacheSorted.Min;
                    _cacheSorted.Remove(min);
                    bool removeResult = _cache.TryRemove(min.Key, out _);
                    Logger.WoxDebug($"remove exceed: <{removeResult}> entry: <{min.Key}>");
                }

                return entry;
            }
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
