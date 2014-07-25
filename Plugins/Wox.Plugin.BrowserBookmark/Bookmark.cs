﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using Wox.Infrastructure;

namespace Wox.Plugin.BrowserBookmark {
	public class Bookmark : IEquatable<Bookmark>, IEqualityComparer<Bookmark> {
		public string Name { get; set; }
		public string PinyinName { get { return Name.Unidecode(); } }
		public string Url { get; set; }
		public string Source { get; set; }
		public int Score { get; set; }

		/* TODO: since Source maybe unimportant, we just need to compare Name and Url */
		public bool Equals(Bookmark other) {
			return Equals(this, other);
		}

		public bool Equals(Bookmark x, Bookmark y) {
			if (Object.ReferenceEquals(x, y)) return true;
			if (Object.ReferenceEquals(x, null) || Object.ReferenceEquals(y, null))
				return false;

			return x.Name == y.Name && x.Url == y.Url;
		}

		public int GetHashCode(Bookmark bookmark) {
			if (Object.ReferenceEquals(bookmark, null)) return 0;
			int hashName = bookmark.Name == null ? 0 : bookmark.Name.GetHashCode();
			int hashUrl = bookmark.Url == null ? 0 : bookmark.Url.GetHashCode();
			return hashName ^ hashUrl;
		}

		public override int GetHashCode() {
			return GetHashCode(this);
		}
	}
}
