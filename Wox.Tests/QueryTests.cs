using System;
using NUnit.Framework;
using Wox.Plugin;

namespace Wox.Tests
{
    [TestFixture]
    public class QueryTests
    {
        [Test]
        public void Test1()
        {
            Assert.Throws<ArgumentNullException>(() => new Query(null));
        }

        [Test]
        public void Test2()
        {
            var query = new Query(string.Empty);

            Assert.AreEqual(query.Raw, string.Empty);
            Assert.AreEqual(query.Command, string.Empty);

            Assert.AreEqual(query.Arguments, new[] {""});

            Assert.AreEqual(query.Tail, null);
            Assert.AreEqual(query.Modificator, null);
            Assert.AreEqual(query.Options, null);

            Assert.AreEqual(query.IsEmpty(), true);
        }

        [Test]
        public void Test3()
        {
            var query = new Query("foo");

            Assert.AreEqual(query.Raw, "foo");
            Assert.AreEqual(query.Command, "foo");

            Assert.AreEqual(query.Arguments, new[] {"foo"});

            Assert.AreEqual(query.Tail, null);
            Assert.AreEqual(query.Modificator, null);
            Assert.AreEqual(query.Options, null);

            Assert.AreEqual(query.IsEmpty(), false);
        }

        [Test]
        public void Test4()
        {
            var query = new Query("foo bar");

            Assert.AreEqual(query.Raw, "foo bar");
            Assert.AreEqual(query.Command, "foo");

            Assert.AreEqual(query.Arguments, new[] {"foo", "bar"});

            Assert.AreEqual(query.Tail, "bar");
            Assert.AreEqual(query.Modificator, "bar");
            Assert.AreEqual(query.Options, null);

            Assert.AreEqual(query.IsEmpty(), false);
        }

        [Test]
        public void Test5()
        {
            var query = new Query("foo bar baz");

            Assert.AreEqual(query.Raw, "foo bar baz");
            Assert.AreEqual(query.Command, "foo");

            Assert.AreEqual(query.Arguments, new[] {"foo", "bar", "baz"});

            Assert.AreEqual(query.Tail, "bar baz");
            Assert.AreEqual(query.Modificator, "bar");
            Assert.AreEqual(query.Options, new[] {"baz"});

            Assert.AreEqual(query.IsEmpty(), false);
        }

        [Test]
        public void Test6()
        {
            var query = new Query("foo bar baz meow");

            Assert.AreEqual(query.Raw, "foo bar baz meow");
            Assert.AreEqual(query.Command, "foo");

            Assert.AreEqual(query.Arguments, new[] {"foo", "bar", "baz", "meow"});

            Assert.AreEqual(query.Tail, "bar baz meow");
            Assert.AreEqual(query.Modificator, "bar");
            Assert.AreEqual(query.Options, new[] {"baz", "meow"});

            Assert.AreEqual(query.IsEmpty(), false);
        }
    }
}