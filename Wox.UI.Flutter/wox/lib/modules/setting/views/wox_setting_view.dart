import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingView extends GetView<WoxSettingController> {
  const WoxSettingView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return RawKeyboardListener(
          focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
            if (event is RawKeyDownEvent) {
              switch (event.logicalKey) {
                case LogicalKeyboardKey.escape:
                  controller.hideWindow();
                  return KeyEventResult.handled;
              }
            }

            return KeyEventResult.ignored;
          }),
          child: FluentApp(
            debugShowCheckedModeBanner: false,
            home: NavigationView(
              pane: NavigationPane(
                selected: controller.activePaneIndex.value,
                onChanged: (index) => controller.activePaneIndex.value = index,
                displayMode: PaneDisplayMode.auto,
                items: [
                  PaneItem(
                    icon: const Icon(FluentIcons.home),
                    title: const Text('Home'),
                    body: const Text('Home bdoy'),
                  ),
                  PaneItem(
                    icon: const Icon(FluentIcons.issue_tracking),
                    title: const Text('Track orders'),
                    infoBadge: const InfoBadge(source: Text('8')),
                    body: const Text('Track orders'),
                  ),
                  PaneItem(
                    icon: const Icon(FluentIcons.disable_updates),
                    title: const Text('Disabled Item'),
                    body: const Text('Track orders'),
                    enabled: false,
                  ),
                  PaneItemExpander(
                    icon: const Icon(FluentIcons.account_management),
                    title: const Text('Account'),
                    body: const Text('Track orders'),
                    items: [
                      PaneItem(
                        icon: const Icon(FluentIcons.mail),
                        title: const Text('Mail'),
                        body: const Text('Track orders'),
                      ),
                      PaneItem(
                        icon: const Icon(FluentIcons.calendar),
                        title: const Text('Calendar'),
                        body: const Text('Track orders'),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ));
    });
  }
}
