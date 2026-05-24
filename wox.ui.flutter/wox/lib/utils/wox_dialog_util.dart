import 'package:flutter/material.dart';

/// Shows a dialog as an in-window overlay, bypassing Flutter's native windowed-dialog
/// feature introduced in Flutter 3.45 / windowing API.
///
/// When [WindowManager] is present and the windowing feature flag is enabled,
/// [showDialog] internally calls [showRawDialog] which redirects to a native OS
/// dialog window (via [DialogWindowController]).  That makes every modal in the
/// settings window appear as a tiny floating OS window titled "Dialog" instead of
/// a normal in-app overlay.
///
/// This function replicates [showDialog]'s behaviour — including theme capture,
/// barrier colour, safe-area handling and dismiss behaviour — by pushing
/// [DialogRoute] directly to the [Navigator], completely bypassing
/// [showRawDialog] and its windowing check.
Future<T?> showWoxDialog<T>({
  required BuildContext context,
  required WidgetBuilder builder,
  bool barrierDismissible = true,
  Color? barrierColor,
  String? barrierLabel,
  bool useSafeArea = true,
  bool useRootNavigator = true,
  RouteSettings? routeSettings,
}) {
  final CapturedThemes themes = InheritedTheme.capture(from: context, to: Navigator.of(context, rootNavigator: useRootNavigator).context);
  return Navigator.of(context, rootNavigator: useRootNavigator).push<T>(
    DialogRoute<T>(
      context: context,
      builder: builder,
      barrierColor: barrierColor ?? DialogTheme.of(context).barrierColor ?? Colors.black54,
      barrierDismissible: barrierDismissible,
      barrierLabel: barrierLabel,
      useSafeArea: useSafeArea,
      themes: themes,
      settings: routeSettings,
    ),
  );
}
