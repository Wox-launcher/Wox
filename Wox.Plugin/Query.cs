using System;
using JetBrains.Annotations;
using Wox.Core.Extensions;

namespace Wox.Plugin
{
    /// <summary>
    /// Query structure:
    ///   * Command
    ///   * Modificator
    ///   * Options
    /// </summary>
    /// <example>
    /// npm install express
    /// </example>
    public sealed class Query
    {
        public Query([CanBeNull] string raw)
        {
            if (raw == null)
                throw new ArgumentNullException("raw");
            
            Raw = raw.Trim();
            Parse();
        }

        /// <summary>
        /// Raw query representation
        /// </summary>
        [NotNull]
        public string Raw { get; private set; }

        /// <summary>
        /// Query arguments
        /// </summary>
        public string[] Arguments { get; private set; }

        /// <summary>
        /// Query tail (everything except first argument)
        /// </summary>
        [CanBeNull]
        public string Tail { get; private set; }

        /// <summary>
        /// Query keyword (first argument)
        /// </summary>
        [NotNull]
        public string Command { get; private set; }

        /// <summary>
        /// Query decorator (second argument)
        /// </summary>
        [CanBeNull]
        public string Modificator { get; private set; }

        /// <summary>
        /// Query options (everything except first and second arguments)
        /// </summary>
        [CanBeNull]
        public string[] Options { get; private set; }

        /// <summary>
        /// Query options length
        /// </summary>
        private int Length { get; set; }

        public bool IsEmpty()
        {
            return Raw == string.Empty;
        }

        private void Parse()
        {
            Arguments = Raw.Split(' ');
            Length = Arguments.Length;

            if (Length > 0)
            {
                Command = Arguments[0]; // car

                if(Length > 1)
                {
                    Tail = string.Join(" ", Arguments.SubArray(1, Arguments.Length - 1)); // cdr
                    Modificator = Arguments[1];

                    if (Length > 2)
                    {
                        Options = Arguments.SubArray(2, Arguments.Length - 2);
                    }
                }
            }
        }
    }
}