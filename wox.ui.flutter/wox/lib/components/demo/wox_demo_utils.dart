part of 'wox_demo.dart';

String _formatDemoHotkey(String hotkey, {required String fallback}) {
  final configuredHotkey = hotkey.trim();
  final rawHotkey = configuredHotkey.isEmpty ? fallback : configuredHotkey;
  // Feature fix: onboarding demos render the persisted hotkey values with the
  // same platform-specific modifier labels as the real toolbar/recorder. That
  // avoids teaching Windows/Linux users macOS-only shortcut symbols.
  return WoxHotkeyDisplayUtil.labelFromHotkeyString(rawHotkey);
}

String _demoActionPanelHotkey() {
  // Feature fix: settings popovers reuse the same query demos without access to
  // onboarding's configured Action Panel hotkey. Using the platform default
  // keeps the launcher toolbar visible and truthful enough for feature previews
  // while avoiding a dependency on the onboarding controller.
  return _formatDemoHotkey('', fallback: WoxPlatformHotkeyUtil.primaryHotkey('j'));
}

double _demoDesktopHintTopInset() {
  // UX fix: hint cards sit inside the simulated desktop, so macOS needs extra
  // top clearance for the 28 px menu bar. The old shared 18 px inset let the
  // hint card overlap Finder/File and made the teaching prompt look attached to
  // the system chrome instead of the Wox demo content.
  return Platform.isMacOS ? 42 : 18;
}

EdgeInsets _demoDesktopHintContentPadding() {
  return EdgeInsets.fromLTRB(48, _demoDesktopHintTopInset(), 52, 36);
}
