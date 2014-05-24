﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using NUnit.Framework;
using Wox.Plugin;

namespace Wox.Test
{
    public class QueryTest
    {
        [Test]
        public void QueryActionTest()
        {
            Query q = new Query("this");

            q = new Query("ev file.txt");
            Assert.AreEqual(q.Command,"ev");
            Assert.AreEqual(q.Arguments.Length, 1);
            Assert.AreEqual(q.Arguments[0], "file.txt");

            q = new Query("ev file.txt file2.txt");
            Assert.AreEqual(q.Command, "ev");
            Assert.AreEqual(q.Arguments.Length, 2);
            Assert.AreEqual(q.Arguments[1], "file2.txt");

            q = new Query("ev file.txt file2.tx st");
//            Assert.AreEqual(q.GetAllRemainingParameter(), "file.txt file2.tx st");
        }
    }
}
