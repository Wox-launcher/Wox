import 'package:flutter/material.dart' as base;
import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingPluginView extends GetView<WoxSettingController> {
  const WoxSettingPluginView({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 300,
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
                      onChanged: (value) => {controller.onFilterPlugins(value)},
                    ),
                  ),
                ),
                Expanded(
                  child: Scrollbar(
                    child: Obx(() {
                      return ListView.builder(
                        itemCount: controller.filteredPluginDetails.length,
                        itemBuilder: (context, index) {
                          final plugin = controller.filteredPluginDetails[index];
                          return Padding(
                            padding: const EdgeInsets.only(bottom: 8.0),
                            child: Obx(() {
                              return Container(
                                decoration: BoxDecoration(
                                  color: controller.activePluginDetail.value.id == plugin.id ? Colors.blue : Colors.transparent,
                                  borderRadius: BorderRadius.circular(4),
                                ),
                                child: GestureDetector(
                                  behavior: HitTestBehavior.translucent,
                                  onTap: () {
                                    controller.activePluginDetail.value = plugin;
                                  },
                                  child: base.ListTile(
                                    leading: WoxImageView(woxImage: plugin.icon, width: 32),
                                    //ellipsis: true,
                                    title: Text(plugin.name,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: const TextStyle(
                                          fontSize: 15,
                                        )),
                                    subtitle: Padding(
                                      padding: const EdgeInsets.only(top: 4),
                                      child: Text(
                                        "${plugin.version} - ${plugin.author}",
                                        maxLines: 1, // Limiting the description to two lines
                                        overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                                        style: TextStyle(
                                          color: Colors.grey[100],
                                        ),
                                      ),
                                    ),
                                  ),
                                ),
                              );
                            }),
                          );
                        },
                      );
                    }),
                  ),
                ),
              ],
            ),
          ),
          Obx(() {
            return Text(controller.activePluginDetail.value.name);
          }),
        ],
      ),
    );
  }
}
