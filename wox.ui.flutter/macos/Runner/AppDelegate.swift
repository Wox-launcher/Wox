import Cocoa

@main
class AppDelegate: NSObject, NSApplicationDelegate {

    func applicationDidFinishLaunching(_ aNotification: Notification) {
        // Insert code here to initialize your application
    }

    func applicationWillTerminate(_ aNotification: Notification) {
        // Insert code here to tear down your application
    }

    // MARK: - Core Data stack

    lazy var persistentContainer: NSPersistentContainer = {
        /*
         The persistent container for the application. This implementation
         creates and returns a container, having loaded the store for the
         application to it. This property is optional since there are legitimate
         error conditions that could cause the creation of the store to fail.
        */
        let container = NSPersistentContainer(name: "wox")
        container.loadPersistentStores(completionHandler: { (storeDescription, error) in
            if let error = error as NSError? {
                // Replace this implementation with code to handle the error appropriately.
                // fatalError() causes the application to generate a crash log and terminate. You should not use this function in a shipping application, although it may be useful during development.
                 
                /*
                 Typical reasons for an error here include:
                 * The parent directory does not exist, cannot be created, or disallows writing.
                 * The persistent store is not accessible, due to permissions or data protection when the device is locked.
                 * The device is out of space.
                 * The store could not be migrated to the current model version.
                 Check the error message to determine what the actual problem was.
                 */
                fatalError("Unresolved error \(error), \(error.userInfo)")
            }
        })
        return container
    }()

    // MARK: - Core Data Saving and Undo support

    func saveContext () {
        let context = persistentContainer.viewContext
        if context.hasChanges {
            do {
                try context.save()
            } catch {
                // Replace this implementation with code to handle the error appropriately.
                // fatalError() causes the application to generate a crash log and terminate. You should not use this function in a shipping application, although it may be useful during development.
                let nserror = error as NSError
                fatalError("Unresolved error \(nserror), \(nserror.userInfo)")
            }
        }
    }

    // MARK: - Window management

    var window: NSWindow!

    func applicationDidBecomeActive(_ notification: Notification) {
        // This method is called when the application is being activated.
        // The system calls this method when the application is being launched for a user that is not using a different application.
        // If your application is not using a document-based UI, this method is called instead of applicationDidFinishLaunching when the user opens a document.
        // If the application supports background execution, this method is called instead of applicationWillTerminate when the user quits.
        // If your application supports multiple windows, this method is called when the active window changes.
        // Add code here to handle the active window change.
    }

    func applicationWillResignActive(_ notification: Notification) {
        // This method is called when the application is being deactivated.
        // The system calls this method when the application is being closed or put to the background.
        // If the application supports background execution, this method is called instead of applicationWillTerminate when the user quits.
        // If your application supports multiple windows, this method is called when the active window changes.
        // Add code here to handle the active window change.
    }

    func applicationWillTerminate(_ notification: Notification) {
        // This method is called when the application is being terminated.
        // If your application supports background execution, this method is called instead of applicationWillTerminate when the user quits.
        // Add code here to save data if appropriate.
        // If your application supports multiple windows, this method is called when the last window is being closed.
        // Add code here to tear down your application.
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        // This method is called when the last window is being closed.
        // If your application supports multiple windows, this method should return false if the last window is being closed and true if the application should terminate.
        return true
    }

    func application(_ application: NSApplication, didFinishLaunchingWithOptions launchOptions: [NSApplication.LaunchOptionsKey: Any]?) -> Bool {
        // Override point for customization after application launch.
        return true
    }

    func application(_ application: NSApplication, didFailToRegisterForRemoteNotificationsWithError error: Error) {
        // This method is called when the application fails to register for remote notifications.
        // Add code here to notify the user about the error and the reason for the failure.
    }

    func application(_ application: NSApplication, didReceiveRemoteNotification userInfo: [AnyHashable: Any]) {
        // This method is called when the application receives a remote notification.
        // Add code here to handle the notification.
    }

    func application(_ application: NSApplication, didReceiveRemoteNotification userInfo: [AnyHashable: Any], fetchCompletionHandler completionHandler: @escaping (NSApplication.RemoteNotificationResult) -> Void) {
        // This method is called when the application receives a remote notification while it is running in the foreground.
        // Add code here to handle the notification.
        completionHandler(.newData)
    }

    func application(_ application: NSApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        // This method is called when the application successfully registers for remote notifications.
        // Add code here to send the token to your server.
    }

    func application(_ application: NSApplication, didRegisterForRemoteNotificationTypes types: NSApplication.RemoteNotificationType) {
        // This method is called when the application is being asked to register for remote notification types.
        // Add code here to register for remote notification types.
    }

    func application(_ application: NSApplication, didReceive notification: NSUserNotification) {
        // This method is called when the application receives a user notification.
        // Add code here to handle the notification.
    }

    func application(_ application: NSApplication, didActivate application: NSApplication) {
        // This method is called when the application is being activated.
        // Add code here to handle the activation.
    }

    func application(_ application: NSApplication, didDeactivate application: NSApplication) {
        // This method is called when the application is being deactivated.
        // Add code here to handle the deactivation.
    }

    func application(_ application: NSApplication, didFailToRegisterForProfileNotificationWithError error: Error) {
        // This method is called when the application fails to register for profile notifications.
        // Add code here to notify the user about the error and the reason for the failure.
    }

    func application(_ application: NSApplication, didReceiveProfileNotification notification: NSUserNotification) {
        // This method is called when the application receives a profile notification.
        // Add code here to handle the notification.
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
    }

    func application(_ application: NSApplication, didFinishLaunchingWithOptions launchOptions: [NSApplication.LaunchOptionsKey: Any]?, withCompletionHandler completionHandler: @escaping (NSApplication.LaunchResult) -> Void) {
        // This method is called when the application is being launched with options.
        // Add code here to handle the launch options.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didFailToRegisterForCustomProtocolNotificationWithError error: Error) {
        // This method is called when the application fails to register for a custom protocol notification.
        // Add code here to notify the user about the error and the reason for the failure.
    }

    func application(_ application: NSApplication, didReceiveCustomProtocolNotification notification: NSUserNotification) {
        // This method is called when the application receives a custom protocol notification.
        // Add code here to handle the notification.
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

    func application(_ application: NSApplication, didReceive screenSaverEvent event: NSEvent, withCompletionHandler completionHandler: @escaping (NSApplication.ScreenSaverResult) -> Void) {
        // This method is called when the application receives a screen saver event.
        // Add code here to handle the event.
        completionHandler(.success)
    }

 