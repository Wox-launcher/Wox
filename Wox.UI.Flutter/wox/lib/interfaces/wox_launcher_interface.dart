import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';

/// This is the interface that will be used to communicate with the Wox Launcher.
abstract class WoxLauncherInterface {
  /// Hide the Wox Launcher.
  Future<void> hideApp() async {}

  /// Show the Wox Launcher.
  /// [params] is the parameters of the show action.
  Future<void> showApp(ShowAppParams params) async {}

  /// Toggle the Wox Launcher.
  /// [params] is the parameters of the toggle action.
  Future<void> toggleApp(ShowAppParams params) async {}

  /// Toggle the action panel.
  /// [params] is the parameters of the toggle action.
  Future<void> toggleActionPanel() async {}

  /// When key enter is pressed, call this method to execute the result action.
  Future<void> executeResultAction() async {}

  /// When the query is changed, this method will be called.
  /// [query] is the changed query.
  void onQueryChanged(WoxChangeQuery query) {}

  /// When the query box on action panel is changed, this method will be called.
  /// [queryAction] is the changed query action.
  void onQueryActionChanged(String queryAction) {}

  /// When arrow down/up is pressed, or mouse wheel down/up is changed, this method will be called.
  /// And this method will change the active result index and scroll the result list.
  /// [direction] is the direction of the change.
  /// [deviceType] is the device type of the change.
  void changeResultScrollPosition(WoxEventDeviceType deviceType, WoxDirection direction) {}

  void changeResultActionScrollPosition(WoxEventDeviceType deviceType, WoxDirection direction) {}
}
