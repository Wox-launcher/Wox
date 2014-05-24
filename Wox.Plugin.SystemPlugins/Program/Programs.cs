using System;
using System.Collections.Generic;
using System.Linq;
using Wox.Infrastructure;
using Wox.Infrastructure.Storage.UserSettings;
using Wox.Plugin.SystemPlugins.Program.ProgramSources;

namespace Wox.Plugin.SystemPlugins.Program
{
    public class Program
    {
        private static readonly global::System.Text.RegularExpressions.Regex AbbrRegexp = new global::System.Text.RegularExpressions.Regex("[^A-Z0-9]", global::System.Text.RegularExpressions.RegexOptions.Compiled);
        private string m_Title;
        public string Title
        {
            get
            {
                return m_Title;
            }
            set
            {
                m_Title = value;
                string pinyin = m_Title.Unidecode();
                PinyinTitle = pinyin;
                AbbrTitle = AbbrRegexp.Replace(global::System.Threading.Thread.CurrentThread.CurrentCulture.TextInfo.ToTitleCase(pinyin), "");
                if (AbbrTitle.Length < 2) AbbrTitle = null;
            }
        }
        public string PinyinTitle { get; private set; }
        public string AbbrTitle { get; private set; }
        public string IcoPath { get; set; }
        public string ExecutePath { get; set; }
        public string ExecuteName { get; set; }
        public int Score { get; set; }
        public IProgramSource Source { get; set; }
    }

    public class Programs : BaseSystemPlugin, ISettingProvider
    {
        List<Program> installedList = new List<Program>();
        List<IProgramSource> sources = new List<IProgramSource>();
        public static Dictionary<string, Type> SourceTypes = new Dictionary<string, Type>() { 
            {"FileSystemProgramSource", typeof(FileSystemProgramSource)},
            {"PortableAppsProgramSource", typeof(PortableAppsProgramSource)},
            {"CommonStartMenuProgramSource", typeof(CommonStartMenuProgramSource)},
            {"UserStartMenuProgramSource", typeof(UserStartMenuProgramSource)},
            {"AppPathsProgramSource", typeof(AppPathsProgramSource)},
			//{"FileSystemFolderSourceShallow", typeof(FileSystemFolderSourceShallow)},
        };
        private PluginInitContext context;

        public override List<Result> Query(Query query)
        {
            if (string.IsNullOrEmpty(query.RawQuery) || query.RawQuery.EndsWith(" ") || query.RawQuery.Length <= 1) return new List<Result>();

            var fuzzyMather = FuzzyMatcher.Create(query.RawQuery);
            List<Program> returnList = installedList.Where(o => MatchProgram(o, fuzzyMather)).ToList();
            returnList.ForEach(ScoreFilter);
            //return ordered list instead of return the score, because programs scores will affect other
            //plugins, the weight of program should be less than the plugins when they showed at the same time.
            returnList = returnList.OrderByDescending(o => o.Score).ToList();

            return returnList.Select(c => new Result()
            {
                Title = c.Title,
                SubTitle = c.ExecutePath,
                IcoPath = c.IcoPath,
                Score = 0,
                Action = (e) =>
                {
                    context.HideApp();
                    context.ShellRun(c.ExecutePath);
                    return true;
                }
            }).ToList();
        }

        private bool MatchProgram(Program program, FuzzyMatcher matcher)
        {
            if (program.AbbrTitle != null && (program.Score = matcher.Evaluate(program.AbbrTitle).Score) > 0) return true;
            if ((program.Score = matcher.Evaluate(program.Title).Score) > 0) return true;
            if ((program.Score = matcher.Evaluate(program.PinyinTitle).Score) > 0) return true;
            if (program.ExecuteName != null && (program.Score = matcher.Evaluate(program.ExecuteName).Score) > 0) return true;

            return false;
        }

        public override void Init(PluginInitContext context)
        {
            this.context = context;

            if (UserSettingStorage.Instance.ProgramSources == null)
                UserSettingStorage.Instance.ProgramSources = UserSettingStorage.Instance.LoadDefaultProgramSources();

            UserSettingStorage.Instance.ProgramSources.ForEach(source =>
            {
                if (source.Enabled)
                {
                    Type sourceClass;
                    if (SourceTypes.TryGetValue(source.Type, out sourceClass))
                    {
                        sources.Add(sourceClass.GetConstructor(
                            new Type[] { typeof(ProgramSource) }
                            ).Invoke(new object[] { source }) as IProgramSource);
                    }
                    else
                    {
                        // TODO: invalid class
                    }
                }
            });

            foreach (var source in sources)
            {
                var list = source.LoadPrograms();
                list.ForEach(o =>
                {
                    o.Source = source;
                });
                installedList.AddRange(list);
            }

            // filter duplicate program
            installedList = installedList.GroupBy(x => new { x.ExecutePath, x.ExecuteName })
                                         .Select(g => g.First()).ToList();
        }

        private void ScoreFilter(Program p)
        {
            p.Score += p.Source.BonusPoints;

            if (p.Title.Contains("启动") || p.Title.ToLower().Contains("start"))
                p.Score += 10;

            if (p.Title.Contains("帮助") || p.Title.ToLower().Contains("help") || p.Title.Contains("文档") || p.Title.ToLower().Contains("documentation"))
                p.Score -= 10;

            if (p.Title.Contains("卸载") || p.Title.ToLower().Contains("uninstall"))
                p.Score -= 20;
        }


        public override string Name
        {
            get { return "Programs"; }
        }

        public override string IcoPath
        {
            get { return @"Images\app.png"; }
        }

        public override string Description
        {
            get { return base.Description; }
        }

        #region ISettingProvider Members

        public System.Windows.Controls.Control CreateSettingPanel()
        {
            return new ProgramSetting();
        }

        #endregion
    }
}
