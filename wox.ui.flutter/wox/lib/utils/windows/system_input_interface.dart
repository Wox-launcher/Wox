import 'dart:ui';

enum SystemMouseButton { left, right, middle }

abstract final class SystemInputKeys {
  static const String alt = 'alt';
  static const String control = 'control';
  static const String shift = 'shift';
  static const String meta = 'meta';
  static const String escape = 'escape';
  static const String enter = 'enter';
  static const String tab = 'tab';
  static const String space = 'space';
  static const String arrowUp = 'arrowUp';
  static const String arrowDown = 'arrowDown';
  static const String arrowLeft = 'arrowLeft';
  static const String arrowRight = 'arrowRight';
}

abstract class SystemInputInterface {
  Future<void> keyDown(String key);

  Future<void> keyUp(String key);

  Future<void> moveMouse(Offset position);

  Future<void> mouseDown(SystemMouseButton button);

  Future<void> mouseUp(SystemMouseButton button);

  Future<void> keyPress(String key) async {
    await keyDown(key);
    await keyUp(key);
  }

  Future<void> mouseClick({SystemMouseButton button = SystemMouseButton.left, Offset? position}) async {
    if (position != null) {
      await moveMouse(position);
    }

    await mouseDown(button);
    await mouseUp(button);
  }
}
