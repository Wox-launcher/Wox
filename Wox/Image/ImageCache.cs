@@ -1,98 +0,0 @@
﻿using NLog;
using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Windows.Media;

using Wox.Infrastructure.Logger;

namespace Wox.Infrastructure.Image
{
    class CacheEntry
    {
        internal string key;
        internal ImageSource Image;
        internal DateTime ExpiredDate;

        public CacheEntry(string key, ImageSource image)
        {
            Image = image;
            ExpiredDate = DateTime.Now;
        }
    }

    class ImageCache
    {
        private readonly TimeSpan _expiredTime = new TimeSpan(24, 0, 0);
        private readonly TimeSpan _checkInterval = new TimeSpan(0, 1, 0);
        private const int _cacheLimit = 100;
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
        public ImageSource GetOrAdd(string key)
        {
            CacheEntry entry;
            bool getResult = _cache.TryGetValue(key, out entry);
            if (!getResult)
            {
                ImageSource image = _imageFactory(key);
                entry = new CacheEntry(key, image);
                _cache[key] = entry;
                _cacheSorted.Add(entry);

                int currentCount = _cache.Count;
                if (currentCount > _cacheLimit)
                {
                    CacheEntry min = _cacheSorted.Min;
                    _cacheSorted.Remove(min);
                    bool removeResult = _cache.TryRemove(min.key, out _);
                    Logger.WoxDebug($"remove exceed: <{removeResult}> entry: <{min.key}>");
                }
            }
            entry.ExpiredDate = DateTime.Now + _expiredTime;
            return entry.Image;
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