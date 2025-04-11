import 'package:get/get.dart';
import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/controllers/wox_setting_controller.dart';

abstract class WoxSettingBaseView extends GetView<WoxSettingController> {
  const WoxSettingBaseView({super.key});

  Widget form({double width = 960, required List<Widget> children}) {
    return SingleChildScrollView(
      child: Padding(
        padding: const EdgeInsets.only(left: 20, right: 40, bottom: 20, top: 20),
        child: Column(
          children: [
            ...children.map((e) => SizedBox(
                  width: width,
                  child: e,
                )),
          ],
        ),
      ),
    );
  }

  Widget formField({required String label, required Widget child, String? tips, double labelWidth = 160}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 20),
      child: Column(
        children: [
          Row(
            children: [
              Padding(
                padding: const EdgeInsets.only(right: 20),
                child: SizedBox(width: labelWidth, child: Text(label, textAlign: TextAlign.right)),
              ),
              Flexible(
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: child,
                ),
              ),
            ],
          ),
          if (tips != null)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Padding(
                    padding: const EdgeInsets.only(right: 20),
                    child: SizedBox(width: labelWidth, child: const Text("")),
                  ),
                  Flexible(
                    child: Text(
                      tips,
                      style: TextStyle(color: Colors.grey[90], fontSize: 13),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
