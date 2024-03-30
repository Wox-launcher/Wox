import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingStorePluginView extends GetView<WoxSettingController> {
  const WoxSettingStorePluginView({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.only(bottom: 20),
            child: RawKeyboardListener(
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
              child: TextBox(
                autofocus: true,
                placeholder: 'Search plugins',
                padding: const EdgeInsets.all(10),
                suffix: const Padding(
                  padding: EdgeInsets.only(right: 8.0),
                  child: Icon(FluentIcons.search),
                ),
                onChanged: (value) => {controller.onFilterStorePlugins(value)},
              ),
            ),
          ),
          Expanded(
            child: Scrollbar(
              child: Obx(() {
                return ListView.builder(
                  itemCount: controller.storePlugins.length,
                  itemBuilder: (context, index) {
                    final plugin = controller.storePlugins[index];
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 8.0),
                      child: ListTile(
                        leading: WoxImageView(woxImage: plugin.icon, width: 32),
                        title: Text(plugin.name),
                        subtitle: Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(
                            plugin.description,
                            maxLines: 2, // Limiting the description to two lines
                            overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                            style: TextStyle(
                              color: Colors.grey[100],
                            ),
                          ),
                        ),
                        trailing: plugin.isInstalled
                            ? const Text("Installed")
                            : Button(
                                style: ButtonStyle(
                                  backgroundColor: ButtonState.resolveWith((states) {
                                    // blue color for installed plugins
                                    return Colors.blue;
                                  }),
                                  foregroundColor: ButtonState.resolveWith((states) {
                                    // white color for installed plugins
                                    return Colors.white;
                                  }),
                                ),
                                onPressed: () async {
                                  plugin.isInstalling = true;
                                  await controller.install(plugin);
                                  plugin.isInstalling = false;
                                }, // Add onPressed feature
                                child: Row(
                                  children: [
                                    plugin.isInstalling ? const Icon(FluentIcons.processing) : const Icon(FluentIcons.download),
                                    const SizedBox(width: 4),
                                    const Text('Install'),
                                  ],
                                ), // Change the text to 'Install'
                              ),
                      ),
                    );
                  },
                );
              }),
            ),
          ),
        ],
      ),
    );
  }
}
