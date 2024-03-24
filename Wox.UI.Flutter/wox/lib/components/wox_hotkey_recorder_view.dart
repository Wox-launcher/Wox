import 'package:fluent_ui/fluent_ui.dart';
import 'package:hotkey_manager/hotkey_manager.dart';

class WoxHotkeyRecorder extends StatefulWidget {
  final ValueChanged<HotKey> onHotKeyRecorded;
  final HotKey? hotkey;

  const WoxHotkeyRecorder({super.key, required this.onHotKeyRecorded, required this.hotkey});

  @override
  State<WoxHotkeyRecorder> createState() => _WoxHotkeyRecorderState();
}

class _WoxHotkeyRecorderState extends State<WoxHotkeyRecorder> {
  final controller = FlyoutController();

  @override
  Widget build(BuildContext context) {
    return FlyoutTarget(
        controller: controller,
        child: Button(
          child: widget.hotkey == null ? Text("empty") : HotKeyVirtualView(hotKey: widget.hotkey!),
          onPressed: () {
            controller.showFlyout(
              autoModeConfiguration: FlyoutAutoConfiguration(
                preferredMode: FlyoutPlacementMode.right,
              ),
              barrierDismissible: false,
              dismissOnPointerMoveAway: true,
              dismissWithEsc: true,
              builder: (context) {
                return FlyoutContent(
                  child: SizedBox(
                    width: 120,
                    height: 50,
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text("Recording..."),
                        HotKeyRecorder(
                          onHotKeyRecorded: widget.onHotKeyRecorded,
                        ),
                      ],
                    ),
                  ),
                );
              },
            );
          },
        ));
  }
}
