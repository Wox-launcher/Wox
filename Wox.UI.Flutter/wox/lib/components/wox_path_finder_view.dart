import 'package:fluent_ui/fluent_ui.dart';
import 'package:uuid/v4.dart';
import 'package:wox/utils/picker.dart';

class WoxPathFinder extends StatefulWidget {
  final String path;
  final bool showOpenButton;
  final bool showChangeButton;
  final ValueChanged<String> onChanged;

  const WoxPathFinder({
    super.key,
    required this.path,
    this.showOpenButton = false,
    this.showChangeButton = true,
    required this.onChanged,
  });

  @override
  State<WoxPathFinder> createState() => _WoxPathFinderState();
}

class _WoxPathFinderState extends State<WoxPathFinder> {
  late TextEditingController controller;
  bool disableBrowse = false;

  @override
  void initState() {
    super.initState();
    controller = TextEditingController(text: widget.path);
  }

  @override
  void didUpdateWidget(WoxPathFinder oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.path != controller.text) {
      controller.text = widget.path;
    }
  }

  @override
  void dispose() {
    controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return TextBox(
      controller: controller,
      onChanged: (value) {
        widget.onChanged(value);
      },
      suffixMode: OverlayVisibilityMode.always,
      suffix: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (widget.showChangeButton)
            Button(
              onPressed: disableBrowse
                  ? null
                  : () async {
                      disableBrowse = true;
                      final selectedDirectory = await FileSelector.pick(
                        const UuidV4().generate(),
                        FileSelectorParams(isDirectory: true),
                      );
                      if (selectedDirectory.isNotEmpty) {
                        controller.text = selectedDirectory[0];
                        widget.onChanged(selectedDirectory[0]);
                      }
                      disableBrowse = false;
                    },
              child: const Text('Browse'),
            ),
        ],
      ),
    );
  }
}
