﻿using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using NUnit.Framework;
using Wox.Infrastructure;

namespace Wox.Test
{
    [TestFixture]
    public class FuzzyMatcherTest
    {
        [Test]
        public void MatchTest()
        {
            var sources = new List<string>()
            {
                "file open in browser-test",
                "Install Package",
                "add new bsd",
                "Inste",
                "aac",
            };


            var results = new List<Wox.Plugin.Result>();
            foreach (var str in sources)
            {
                results.Add(new Plugin.Result()
                {
                    Title = str,
                    Score = FuzzyMatcher.Create("inst").Score(str)
                });
            }

            results = results.Where(x => x.Score > 0).OrderByDescending(x => x.Score).ToList();

            Assert.IsTrue(results.Count == 3);
            Assert.IsTrue(results[0].Title == "Inste");
            Assert.IsTrue(results[1].Title == "Install Package");
            Assert.IsTrue(results[2].Title == "file open in browser-test");
        }
    }
}
