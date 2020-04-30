using System;
using System.Runtime.CompilerServices;
using System.Threading.Tasks;
using System.Windows.Threading;
using Mindscape.Raygun4Net;
using NLog;
using Wox.Infrastructure.Exception;

namespace Wox.Helper
{
    public static class ErrorReporting
    {
        private static RaygunClient _raygunClient = new RaygunClient("LG5MX0YYMCpCN2AtD0fdZw");

        private static void Report(Exception e, [CallerMemberName] string method = "")
        {
            var logger = LogManager.GetLogger(method);
            logger.Fatal(ExceptionFormatter.ExceptionWithRuntimeInfo(e));
            var reportWindow = new ReportWindow(e);
            reportWindow.Show();
        }

        public static void UnhandledExceptionHandleTask(Task t)
        {
            _raygunClient.Send(t.Exception);
            Report(t.Exception);
        }

        public static void UnhandledExceptionHandleMain(object sender, UnhandledExceptionEventArgs e)
        {
            _raygunClient.Send(e.ExceptionObject as Exception);
            //handle non-ui main thread exceptions
            Report((Exception)e.ExceptionObject);
        }

        public static void DispatcherUnhandledException(object sender, DispatcherUnhandledExceptionEventArgs e)
        {
            _raygunClient.Send(e.Exception);
            Report(e.Exception);
            //prevent application exist, so the user can copy prompted error info
            e.Handled = true;
        }

    }
}
