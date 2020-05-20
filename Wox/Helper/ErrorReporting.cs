using System;
using System.Net;
using System.Net.Sockets;
using System.Runtime.CompilerServices;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Threading;
using NLog;
using Sentry;
using Sentry.Protocol;
using Wox.Infrastructure;
using Wox.Infrastructure.Exception;

namespace Wox.Helper
{
    public static class ErrorReporting
    {
        private static void Report(Exception e, string id, [CallerMemberName] string method = "")
        {
            var logger = LogManager.GetLogger(method);
            logger.Fatal(ExceptionFormatter.ExceptionWithRuntimeInfo(e, id));

            var reportWindow = new ReportWindow(e, id);
            reportWindow.Show();
        }

        public static void UnhandledExceptionHandleTask(Task t)
        {
            string id = SendException(t.Exception);
            Application.Current.Dispatcher.Invoke(() =>
            {
                Report(t.Exception, id.ToString());
            });
        }

        public static void UnhandledExceptionHandleMain(object sender, UnhandledExceptionEventArgs e)
        {
            string id = SendException(e.ExceptionObject as Exception);
            //handle non-ui main thread exceptions
            Application.Current.Dispatcher.Invoke(() =>
            {
                Report((Exception)e.ExceptionObject, id.ToString());
            });
        }

        public static void DispatcherUnhandledException(object sender, DispatcherUnhandledExceptionEventArgs e)
        {
            string id = SendException(e.Exception);
            Report(e.Exception, id);
            //prevent application exist, so the user can copy prompted error info
            e.Handled = true;
        }

        public static IDisposable InitializedSentry(string systemLanguage)
        {
            var s = SentrySdk.Init(o =>
            {
                o.Dsn = new Dsn("https://b87bf28a6fab49bf9cb1b53e9648152f@o385966.ingest.sentry.io/5219588");
                o.Debug = true; // todo
                o.Release = Constant.Version;
                o.SendDefaultPii = true;
                o.DisableAppDomainUnhandledExceptionCapture();
            });
            SentrySdk.ConfigureScope(scope =>
            {
                scope.SetTag("systemLanguage", systemLanguage);
                scope.SetTag("timezone", TimeZoneInfo.Local.DisplayName);
            });
            return s;
        }

        public static string SendException(Exception exception)
        {
            SentryId id = SentryId.Empty;
            SentrySdk.WithScope(scope =>
            {
                scope.Level = SentryLevel.Fatal;
                id = SentrySdk.CaptureException(exception);
            });
            return id.ToString();
        }
    }
}
