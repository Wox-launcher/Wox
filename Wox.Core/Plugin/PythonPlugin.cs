using System;
using System.Diagnostics;
using System.IO;
using System.Windows.Forms;
using Newtonsoft.Json;
using Newtonsoft.Json.Serialization;
using Wox.Infrastructure;
using Wox.Infrastructure.Logger;
using Wox.Plugin;
using System.Collections.Concurrent;
using System.Text;

namespace Wox.Core.Plugin
{
    internal class PythonPlugin : JsonRPCPlugin
    {
        //private readonly ProcessStartInfo _startInfo;
        public override string SupportedLanguage { get; set; } = AllowedLanguage.Python;
        
        private readonly Process _pythonProcess;
        private readonly JsonSerializerSettings _jsonSerializerSettings;
        private readonly BlockingCollection<string> outputBuffer, errorBuffer;

        public PythonPlugin(string filename)
        {
            _pythonProcess = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = filename,
                    UseShellExecute = false,
                    CreateNoWindow = true,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    RedirectStandardInput = true,
                }
            };

            // temp fix for issue #667
            var path = Path.Combine(Constant.ProgramDirectory, JsonRPC);
            _pythonProcess.StartInfo.EnvironmentVariables["PYTHONPATH"] = path;

            _pythonProcess.OutputDataReceived += new DataReceivedEventHandler((sender, e) => outputBuffer.Add(e.Data));
            _pythonProcess.ErrorDataReceived += new DataReceivedEventHandler((sender, e) => errorBuffer.Add(e.Data));

            _jsonSerializerSettings = new JsonSerializerSettings
            {
                ContractResolver = new DefaultContractResolver
                {
                    NamingStrategy = new CamelCaseNamingStrategy
                    {
                        OverrideSpecifiedNames = false
                    }
                }
            };
            outputBuffer = new BlockingCollection<string>(new ConcurrentQueue<string>());
            errorBuffer = new BlockingCollection<string>(new ConcurrentQueue<string>());
        }

        private bool IsRunning()
        {
            try
            {
                return (!_pythonProcess.HasExited && _pythonProcess.Id != 0);
            }
            catch
            {
                return false;
            }
        }

        private string SendToProcess(string request)
        {
            if (!IsRunning())
            {
                try
                {
                    _pythonProcess.CancelOutputRead();
                    _pythonProcess.CancelErrorRead();
                }
                catch (InvalidOperationException) { }
                _pythonProcess.StartInfo.Arguments = $"-B \"{context.CurrentPluginMetadata.ExecuteFilePath}\"";
                _pythonProcess.StartInfo.WorkingDirectory = context.CurrentPluginMetadata.PluginDirectory;
                _pythonProcess.Start();
                _pythonProcess.BeginOutputReadLine();
                _pythonProcess.BeginErrorReadLine();
            }
            
            StreamWriter standardInput = _pythonProcess.StandardInput;

            standardInput.AutoFlush = true;

            while (outputBuffer.TryTake(out string line)) ; //Empty stdout before we start
            standardInput.WriteLine(request);
            if (!outputBuffer.TryTake(out string result, -1))
            {
                result = string.Empty;
            }

            if (string.IsNullOrEmpty(result))
            {
                StringBuilder errorBuild = new StringBuilder("");
                while (errorBuffer.TryTake(out string line, 1000) && line != null)
                {
                    errorBuild.AppendLine(line);
                }
                string error = errorBuild.ToString();
                if (!string.IsNullOrEmpty(error))
                {
                    Log.Error($"|JsonRPCPlugin.Execute|{error}");
                    return string.Empty;
                }
                else
                {
                    Log.Error("|JsonRPCPlugin.Execute|Empty standard output and standard error.");
                    return string.Empty;
                }
            }
            else if (result.StartsWith("DEBUG:"))
            {
                MessageBox.Show(new Form { TopMost = true }, result.Substring(6));
                return string.Empty;
            }
            else
            {
                return result;
            }
        }

        protected override string ExecuteQuery(Query query)
        {
            JsonRPCServerRequestModel request = new JsonRPCServerRequestModel
            {
                Method = "query",
                Parameters = new object[] { query.Search },
            };
            return SendToProcess(JsonConvert.SerializeObject(request, _jsonSerializerSettings));
        }

        protected override string ExecuteCallback(JsonRPCRequestModel rpcRequest)
        {
            return SendToProcess(JsonConvert.SerializeObject(rpcRequest, _jsonSerializerSettings));
        }

        protected override string ExecuteContextMenu(Result selectedResult) {
            JsonRPCServerRequestModel request = new JsonRPCServerRequestModel {
                Method = "context_menu",
                Parameters = new object[] { selectedResult.ContextData },
            };
            return SendToProcess(JsonConvert.SerializeObject(request, _jsonSerializerSettings));
        }
    }
}