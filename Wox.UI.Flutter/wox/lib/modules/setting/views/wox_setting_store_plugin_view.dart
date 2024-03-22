import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingStorePluginView extends GetView<WoxSettingController> {
  const WoxSettingStorePluginView({super.key});

  @override
  Widget build(BuildContext context) {
    return Scrollbar(
      child: Padding(
        padding: const EdgeInsets.only(left: 20, top: 24),
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
                  trailing: Button(
                    onPressed: () {}, // Add onPressed feature
                    child: const Row(
                      children: [
                        Icon(FluentIcons.download),
                        Text('Install'),
                      ],
                    ), // Change the text to 'Install'
                  ),
                ),
              );
            },
          );
        }),
      ),
    );
  }
}
