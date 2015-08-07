﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using NUnit.Framework;
using Wox.Core.Plugin;
using Wox.Plugin;

namespace Wox.Test
{
    public class QueryTest
    {
        [Test]
        public void ExclusivePluginQueryTest()
        {
            Query q = new Query("f file.txt file2 file3");
            q.Search = "file.txt file2 file3";

            Assert.AreEqual(q.FirstSearch, "file.txt");
            Assert.AreEqual(q.SecondSearch, "file2");
            Assert.AreEqual(q.ThirdSearch, "file3");
            Assert.AreEqual(q.SecondToEndSearch, "file2 file3");
        }

        [Test]
        public void GenericPluginQueryTest()
        {
            Query q = new Query("file.txt file2 file3");
            q.Search = q.RawQuery;

            Assert.AreEqual(q.FirstSearch, "file.txt");
            Assert.AreEqual(q.SecondSearch, "file2");
            Assert.AreEqual(q.ThirdSearch, "file3");
            Assert.AreEqual(q.SecondToEndSearch, "file2 file3");
        }
    }
}
