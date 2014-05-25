using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Wox.Core;

namespace Wox.Plugins.Internal.CMD
{
    public class CMD : BaseSystemPlugin
    {
        private PluginInitContext context;

        public override bool IsAvailable(Query query)
        {
            return query.Raw.StartsWith(">");
        }

        public override List<Result> Query(Query query)
        {
            List<Result> results = new List<Result>();
            List<Result> pushedResults = new List<Result>();
            if (query.Raw == ">")
            {
                IEnumerable<Result> history = CMDStorage.Instance.CMDHistory.OrderByDescending(o => o.Value)
                 .Select(m => new Result
                 {
                     Title = m.Key,
                     SubTitle = "this command has been executed " + m.Value + " times",
                     IcoPath = "Images/cmd.png",
                     Action = (c) =>
                     {
                         ExecuteCmd(m.Key);
                         return true;
                     }
                 }).Take(5);

                results.AddRange(history);
            }
            
            if (query.Raw.Length > 1)
            {
                string cmd = query.Raw.Substring(1);
                Result result = new Result
                {
                    Title = cmd,
                    Score = 5000,
                    SubTitle = "execute command through command shell",
                    IcoPath = "Images/cmd.png",
                    Action = (c) =>
                    {
                        ExecuteCmd(cmd);
                        return true;
                    }
                };

                try
                {
                    if (File.Exists(cmd) || Directory.Exists(cmd))
                    {
                        result.IcoPath = cmd;
                    }
                }
                catch (Exception) { }

                context.PushResults(query, new List<Result>() { result });
                pushedResults.Add(result);

                IEnumerable<Result> history = CMDStorage.Instance.CMDHistory.Where(o => o.Key.Contains(cmd))
                    .OrderByDescending(o => o.Value)
                    .Select(m =>
                    {
                        if (m.Key == cmd)
                        {
                            result.SubTitle = "this command has been executed " + m.Value + " times";
                            return null;
                        }

                        var ret = new Result
                        {
                            Title = m.Key,
                            SubTitle = "this command has been executed " + m.Value + " times",
                            IcoPath = "Images/cmd.png",
                            Action = (c) =>
                            {
                                ExecuteCmd(m.Key);
                                return true;
                            }
                        };
                        try
                        {
                            if (File.Exists(m.Key) || Directory.Exists(m.Key))
                            {
                                ret.IcoPath = m.Key;
                            }
                        }
                        catch (Exception) { }

                        return ret;
                    }).Where(o => o != null).Take(4);

                context.PushResults(query, history.ToList());
                pushedResults.AddRange(history);

                try
                {
                    string basedir = null;
                    string dir = null;
                    string excmd = Environment.ExpandEnvironmentVariables(cmd);
                    if (Directory.Exists(excmd) && (cmd.EndsWith("/") || cmd.EndsWith(@"\")))
                    {
                        basedir = excmd;
                        dir = cmd;
                    }
                    else if (Directory.Exists(Path.GetDirectoryName(excmd)))
                    {
                        basedir = Path.GetDirectoryName(excmd);
                        var dirn = Path.GetDirectoryName(cmd);
                        dir = (dirn.EndsWith("/") || dirn.EndsWith(@"\")) ? dirn : cmd.Substring(0, dirn.Length + 1);
                    }

                    if (basedir != null)
                    {
                        List<string> autocomplete = Directory.GetFileSystemEntries(basedir).Select(o => dir + Path.GetFileName(o)).Where(o => o.StartsWith(cmd, StringComparison.OrdinalIgnoreCase) && !results.Any(p => o.Equals(p.Title, StringComparison.OrdinalIgnoreCase)) && !pushedResults.Any(p => o.Equals(p.Title, StringComparison.OrdinalIgnoreCase))).ToList();
                        autocomplete.Sort();
                        results.AddRange(autocomplete.ConvertAll(m => new Result()
                        {
                            Title = m,
                            SubTitle = "",
                            IcoPath = m,
                            Action = (c) =>
                            {
                                ExecuteCmd(m);
                                return true;
                            }
                        }));
                    }
                }
                catch (Exception) { }
            }
            return results;
        }


        private void ExecuteCmd(string cmd)
        {
            if (context.ShellRun(cmd))
                CMDStorage.Instance.AddCmdHistory(cmd);
        }

        public override void Init(PluginInitContext context)
        {
            this.context = context;
        }

        public override string Name
        {
            get { return "Shell"; }
        }

        public override string IcoPath
        {
            get { return @"Images\cmd.png"; }
        }

        public override string Description
        {
            get { return base.Description; }
        }
    }
}
