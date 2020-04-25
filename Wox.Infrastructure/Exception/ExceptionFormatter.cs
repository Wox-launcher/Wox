using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Xml;
using Microsoft.Win32;

namespace Wox.Infrastructure.Exception
{
    public class ExceptionFormatter
    {
        public static string FormattedException(System.Exception ex)
        {
            return FormattedAllExceptionInternal(ex).ToString();
        }


        private static StringBuilder FormattedAllExceptionInternal(System.Exception ex)
        {
            var sb = new StringBuilder();
            sb.AppendLine("Exception begin --------------------");
            int count = 1;
            while (ex != null)
            {
                sb.Append(ex.GetType().FullName);
                sb.Append(": ");
                sb.AppendLine(ex.Message);
                sb.Append("   HResult: ");
                sb.AppendLine(ex.HResult.ToString());

                if (ex.Source != null)
                {
                    sb.Append("   Source: ");
                    sb.AppendLine(ex.Source);
                }
                if (ex.TargetSite != null)
                {
                    sb.Append("   TargetAssembly: ");
                    sb.AppendLine(ex.TargetSite.Module.Assembly.ToString());
                    sb.Append("   TargetModule: ");
                    sb.AppendLine(ex.TargetSite.Module.ToString());
                    sb.Append("   TargetSite: ");
                    sb.AppendLine(ex.TargetSite.ToString());
                }
                sb.AppendLine("   StackTrace: --------------------");
                sb.AppendLine(ex.StackTrace);

                sb.Append("   InnerException ");
                sb.Append(count);
                sb.AppendLine(": --------------------");
                ex = ex.InnerException;
            }

            sb.AppendLine("Exception end --------------------");
            sb.AppendLine();
            return sb;
        }

        private static StringBuilder RuntimeInfo()
        {
            StringBuilder sb = new StringBuilder();

            sb.AppendLine("## Environment");
            sb.AppendLine($"* Command Line: {Environment.CommandLine}");
            sb.AppendLine($"* Timestamp: {DateTime.Now.ToString(CultureInfo.InvariantCulture)}");
            sb.AppendLine($"* Wox version: {Constant.Version}");
            sb.AppendLine($"* OS Version: {Environment.OSVersion.VersionString}");
            sb.AppendLine($"* x64: {Environment.Is64BitOperatingSystem}");
            sb.AppendLine($"* Python Path: {Constant.PythonPath}");
            sb.AppendLine($"* Everything SDK Path: {Constant.EverythingSDKPath}");
            sb.AppendLine($"* CLR Version: {Environment.Version}");
            sb.AppendLine($"* Installed .NET Framework: ");
            foreach (var result in GetFrameworkVersionFromRegistry())
            {
                sb.Append("   * ");
                sb.AppendLine(result);
            }

            sb.AppendLine();
            sb.AppendLine("## Assemblies - " + AppDomain.CurrentDomain.FriendlyName);
            sb.AppendLine();
            foreach (var ass in AppDomain.CurrentDomain.GetAssemblies().OrderBy(o => o.GlobalAssemblyCache ? 50 : 0))
            {
                sb.Append("* ");
                sb.Append(ass.FullName);
                sb.Append(" (");

                if (ass.IsDynamic)
                {
                    sb.Append("dynamic assembly doesn't has location");
                }
                else if (string.IsNullOrEmpty(ass.Location))
                {
                    sb.Append("location is null or empty");

                }
                else
                {
                    sb.Append(ass.Location);

                }
                sb.AppendLine(")");

            }

            return sb;
        }

        public static string ExceptionWithRuntimeInfo(System.Exception ex)
        {
            StringBuilder sb = new StringBuilder();
            var formatted = FormattedAllExceptionInternal(ex);
            sb.Append(formatted);
            var info = RuntimeInfo();
            sb.Append(info);

            return sb.ToString();
        }



        // http://msdn.microsoft.com/en-us/library/hh925568%28v=vs.110%29.aspx
        private static List<string> GetFrameworkVersionFromRegistry()
        {
            try
            {
                var result = new List<string>();
                using (RegistryKey ndpKey = Registry.LocalMachine.OpenSubKey(@"SOFTWARE\Microsoft\NET Framework Setup\NDP\"))
                {
                    foreach (string versionKeyName in ndpKey.GetSubKeyNames())
                    {
                        if (versionKeyName.StartsWith("v"))
                        {
                            RegistryKey versionKey = ndpKey.OpenSubKey(versionKeyName);
                            string name = (string)versionKey.GetValue("Version", "");
                            string sp = versionKey.GetValue("SP", "").ToString();
                            string install = versionKey.GetValue("Install", "").ToString();
                            if (install != "")
                                if (sp != "" && install == "1")
                                    result.Add(string.Format("{0} {1} SP{2}", versionKeyName, name, sp));
                                else
                                    result.Add(string.Format("{0} {1}", versionKeyName, name));

                            if (name != "")
                            {
                                continue;
                            }
                            foreach (string subKeyName in versionKey.GetSubKeyNames())
                            {
                                RegistryKey subKey = versionKey.OpenSubKey(subKeyName);
                                name = (string)subKey.GetValue("Version", "");
                                if (name != "")
                                    sp = subKey.GetValue("SP", "").ToString();
                                install = subKey.GetValue("Install", "").ToString();
                                if (install != "")
                                {
                                    if (sp != "" && install == "1")
                                        result.Add(string.Format("{0} {1} {2} SP{3}", versionKeyName, subKeyName, name, sp));
                                    else if (install == "1")
                                        result.Add(string.Format("{0} {1} {2}", versionKeyName, subKeyName, name));
                                }

                            }

                        }
                    }
                }
                using (RegistryKey ndpKey = Registry.LocalMachine.OpenSubKey(@"SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full\"))
                {
                    int releaseKey = (int)ndpKey.GetValue("Release");
                    {
                        if (releaseKey == 394806 || releaseKey == 394806)
                        {
                            result.Add("v4.6.2");
                        }
                        if (releaseKey == 460798 || releaseKey == 460805)
                        {
                            result.Add("v4.7");
                        }
                        if (releaseKey == 461308 || releaseKey == 461310)
                        {
                            result.Add("v4.7.1");
                        }
                        if (releaseKey == 461808 || releaseKey == 461814)
                        {
                            result.Add("v4.7.2");
                        }
                        if (releaseKey == 528040 || releaseKey == 528209 || releaseKey == 528049)
                        {
                            result.Add("v4.8");
                        }

                    }
                }
                return result;
            }
            catch (System.Exception)
            {
                return new List<string>();
            }

        }
    }
}
