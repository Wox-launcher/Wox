import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingInstalledPluginView extends GetView<WoxSettingController> {
  const WoxSettingInstalledPluginView({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.only(bottom: 20),
            child: TextBox(
              autofocus: true,
              placeholder: 'Search plugins',
              padding: const EdgeInsets.all(10),
              suffix: const Padding(
                padding: EdgeInsets.only(right: 8.0),
                child: Icon(FluentIcons.search),
              ),
              onChanged: (value) => {},
            ),
          ),
          Expanded(
            child: Scrollbar(
              child: Obx(() {
                return ListView.builder(
                  itemCount: controller.installedPlugins.length,
                  itemBuilder: (context, index) {
                    final plugin = controller.installedPlugins[index];
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
                          style: ButtonStyle(
                            backgroundColor: ButtonState.resolveWith((states) {
                              // blue color for installed plugins
                              return Colors.red["dark"];
                            }),
                            foregroundColor: ButtonState.resolveWith((states) {
                              // white color for installed plugins
                              return Colors.white;
                            }),
                          ),
                          onPressed: () {}, // Add onPressed feature
                          child: Row(
                            children: [
                              Icon(FluentIcons.download),
                              SizedBox(width: 4),
                              Text(plugin.isSystem ? "Disable" : 'Uninstall'),
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
