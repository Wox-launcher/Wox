using System;
using System.Collections.Generic;
using System.IO;
using System.Windows.Media;

namespace Wox.Plugin
{

    public class Result : BaseModel
    {
        private string _icoPath;

        public string Title { get; set; }

        public string SubTitle { get; set; }

        /// <summary>
        /// This holds the action keyword that triggered the result. 
        /// If result is triggered by global keyword: *, this should be empty.
        /// </summary>
        public string ActionKeywordAssigned { get; set; }

        public string IcoPath { get; set; }

        public delegate ImageSource IconDelegate();

        public IconDelegate Icon;


        /// <summary>
        /// return true to hide wox after select result
        /// </summary>
        public Func<ActionContext, bool> Action { get; set; }

        public int Score { get; set; }

        /// <summary>
        /// A list of indexes for the characters to be highlighted in Title
        /// </summary>
        public IList<int> TitleHighlightData { get; set; }

        /// <summary>
        /// A list of indexes for the characters to be highlighted in SubTitle
        /// </summary>
        public IList<int> SubTitleHighlightData { get; set; }

        /// <summary>
        /// Only results that originQuery match with current query will be displayed in the panel
        /// </summary>
        internal Query OriginQuery { get; set; }

        /// <summary>
        /// Plugin directory
        /// </summary>
        public string PluginDirectory { get; set; }

        public override bool Equals(object obj)
        {
            var r = obj as Result;
            var equality = r?.PluginID == PluginID && r?.Title == Title && r?.SubTitle == SubTitle;
            return equality;
        }

        public override int GetHashCode()
        {
            int hash1 = PluginID?.GetHashCode() ?? 0;
            int hash2 = Title?.GetHashCode() ?? 0;
            int hash3 = SubTitle?.GetHashCode() ?? 0;
            int hashcode = hash1 ^ hash2 ^ hash3;
            return hashcode;
        }

        public override string ToString()
        {
            return Title + SubTitle;
        }


        /// <summary>
        /// Context menus associate with this result
        /// </summary>
        [Obsolete("Use IContextMenu instead")]
        public List<Result> ContextMenu { get; set; }

        [Obsolete("Use Object initializer instead")]
        public Result(string Title, string IcoPath, string SubTitle = null)
        {
            this.Title = Title;
            this.IcoPath = IcoPath;
            this.SubTitle = SubTitle;
        }

        public Result() { }

        /// <summary>
        /// Additional data associate with this result
        /// </summary>
        public object ContextData { get; set; }

        /// <summary>
        /// Plugin ID that generated this result
        /// </summary>
        public string PluginID { get; internal set; }
    }
}