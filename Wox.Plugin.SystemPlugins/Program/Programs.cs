using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.Serialization.Formatters.Binary;
using System.Windows.Forms;
using Wox.Infrastructure;
using Wox.Infrastructure.Storage.UserSettings;
using Wox.Plugin.SystemPlugins.Program.ProgramSources;

namespace Wox.Plugin.SystemPlugins.Program
{
    public class Programs : BaseSystemPlugin, ISettingProvider
    {
        private static object lockObject = new object();
        private static List<Program> programs = new List<Program>();
        private static List<IProgramSource> sources = new List<IProgramSource>();
        private static Dictionary<string, Type> SourceTypes = new Dictionary<string, Type>() { 
            {"FileSystemProgramSource", typeof(FileSystemProgramSource)},
            {"CommonStartMenuProgramSource", typeof(CommonStartMenuProgramSource)},
            {"UserStartMenuProgramSource", typeof(UserStartMenuProgramSource)},
            {"AppPathsProgramSource", typeof(AppPathsProgramSource)},
        };
        private PluginInitContext context;

        protected override List<Result> QueryInternal(Query query)
        {
            if (query.RawQuery.Trim().Length <= 1) return new List<Result>();

            var fuzzyMather = FuzzyMatcher.Create(query.RawQuery);
            List<Program> returnList = programs.Where(o => MatchProgram(o, fuzzyMather)).ToList();
            returnList.ForEach(ScoreFilter);
            returnList = returnList.OrderByDescending(o => o.Score).ToList();

            return returnList.Select(c => new Result()
            {
                Title = c.Title,
                SubTitle = c.ExecutePath,
                IcoPath = c.IcoPath,
                Score = c.Score,
                Action = (e) =>
                {
                    context.API.HideApp();
                    context.API.ShellRun(c.ExecutePath);
                    return true;
                },
                ContextMenu = new List<Result>()
                {
                    new Result()
                    {
                        Title = "Run As Administrator",
                        Action = _ =>
                        {
                            context.API.HideApp();
                            context.API.ShellRun(c.ExecutePath,true);
                            return true;
                        },
                        IcoPath = "Images/cmd.png"
                    }
                }
            }).ToList();
        }

        private bool MatchProgram(Program program, FuzzyMatcher matcher)
        {
            if ((program.Score = matcher.Evaluate(program.Title).Score) > 0) return true;
            if ((program.Score = matcher.Evaluate(program.PinyinTitle).Score) > 0) return true;
            if (program.AbbrTitle != null && (program.Score = matcher.Evaluate(program.AbbrTitle).Score) > 0) return true;
            if (program.ExecuteName != null && (program.Score = matcher.Evaluate(program.ExecuteName).Score) > 0) return true;

            return false;
        }

        protected override void InitInternal(PluginInitContext context)
        {
            this.context = context;
            programs = ProgramCacheStorage.Instance.Programs;
            using (new Timeit("Program Index"))
            {
                IndexPrograms();
            }
        }

        public static void IndexPrograms()
        {
            lock (lockObject)
            {
                List<ProgramSource> programSources = new List<ProgramSource>();
                programSources.AddRange(LoadDeaultProgramSources());
                if (UserSettingStorage.Instance.ProgramSources != null &&
                    UserSettingStorage.Instance.ProgramSources.Count(o => o.Enabled) > 0)
                {
                    programSources.AddRange(UserSettingStorage.Instance.ProgramSources.Where(o => o.Enabled));
                }

                sources.Clear();
                programSources.ForEach(source =>
                {
                    Type sourceClass;
                    if (SourceTypes.TryGetValue(source.Type, out sourceClass))
                    {
                        ConstructorInfo constructorInfo = sourceClass.GetConstructor(new[] { typeof(ProgramSource) });
                        if (constructorInfo != null)
                        {
                            IProgramSource programSource =
                                constructorInfo.Invoke(new object[] { source }) as IProgramSource;
                            sources.Add(programSource);
                        }
                    }
                });

                var tempPrograms = new List<Program>();
                foreach (var source in sources)
                {
                    var list = source.LoadPrograms();
                    list.ForEach(o =>
                    {
                        o.Source = source;
                    });
                    tempPrograms.AddRange(list);
                }

                // filter duplicate program
                programs = tempPrograms.GroupBy(x => new { x.ExecutePath, x.ExecuteName })
                    .Select(g => g.First()).ToList();

                ProgramCacheStorage.Instance.Programs = programs;
                ProgramCacheStorage.Instance.Save();
            }
        }

        /// <summary>
        /// Load program sources that wox always provide
        /// </summary>
        private static List<ProgramSource> LoadDeaultProgramSources()
        {
            var list = new List<ProgramSource>();
            list.Add(new ProgramSource()
            {
                BonusPoints = 0,
                Enabled = true,
                Type = "CommonStartMenuProgramSource"
            });
            list.Add(new ProgramSource()
            {
                BonusPoints = 0,
                Enabled = true,
                Type = "UserStartMenuProgramSource"
            });
            list.Add(new ProgramSource()
            {
                BonusPoints = -10,
                Enabled = true,
                Type = "AppPathsProgramSource"
            });
            return list;
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

        public override string ID
        {
            get { return "791FC278BA414111B8D1886DFE447410"; }
        }

        public override string Name
        {
            get { return "Programs"; }
        }

        public override string IcoPath
        {
            get { return @"Images\program.png"; }
        }

        public override string Description
        {
            get { return "Provide searching programs in your computer."; }
        }

        #region ISettingProvider Members

        public System.Windows.Controls.Control CreateSettingPanel()
        {
            return new ProgramSetting();
        }

        #endregion
    }
}
