using System;
using System.Threading.Tasks;
using System.Windows.Threading;
using NLog;
using Wox.Infrastructure;
using Wox.Infrastructure.Exception;

namespace Wox.Helper
{
    public static class ErrorReporting
    {
        private static void Report(Exception e)
        {
            var logger = LogManager.GetLogger("UnHandledException");
            logger.Fatal(ExceptionFormatter.ExceptionWithRuntimeInfo(e));
            var reportWindow = new ReportWindow(e);
            reportWindow.Show();
        }

        public static void UnhandledExceptionHandleTask(Task t)
        {
            //handle non-ui sub task exceptions
            Report(t.Exception);
        }

        public static void UnhandledExceptionHandleMain(object sender, UnhandledExceptionEventArgs e)
        {
            //handle non-ui main thread exceptions
            Report((Exception)e.ExceptionObject);
        }

        public static void DispatcherUnhandledException(object sender, DispatcherUnhandledExceptionEventArgs e)
        {
            //handle ui thread exceptions
            Report(e.Exception);
            //prevent application exist, so the user can copy prompted error info
            e.Handled = true;
        }

    }
}
