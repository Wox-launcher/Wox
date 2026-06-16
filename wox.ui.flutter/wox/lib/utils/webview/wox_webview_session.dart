import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';

enum WoxWebViewSessionAction { toggleActionPanel, fallbackEscape }

class WoxWebViewNavigationState {
  final bool canGoBack;
  final bool canGoForward;

  const WoxWebViewNavigationState({this.canGoBack = false, this.canGoForward = false});
}

abstract class WoxWebViewSession {
  bool get isCached;

  String? get cacheKey;

  Stream<WoxWebViewSessionAction> get actions;

  ValueListenable<WoxWebViewNavigationState> get navigationState;

  Widget buildWidget();

  Future<void> dispose();
}
