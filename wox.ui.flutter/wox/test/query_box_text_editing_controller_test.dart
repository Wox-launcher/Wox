import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:wox/controllers/query_box_text_editing_controller.dart';

void main() {
  testWidgets('renders selected text with selected style only when enabled', (tester) async {
    final controller = QueryBoxTextEditingController(selectedTextStyle: const TextStyle(color: Colors.white), enableSelectedTextStyle: true)
      ..value = const TextEditingValue(text: 'gh issues', selection: TextSelection(baseOffset: 0, extentOffset: 2));

    late BuildContext context;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (buildContext) {
            context = buildContext;
            return const SizedBox.shrink();
          },
        ),
      ),
    );

    final focusedSpan = controller.buildTextSpan(context: context, style: const TextStyle(color: Colors.black));
    final focusedChildren = focusedSpan.children!.cast<TextSpan>();
    expect(focusedChildren.first.style?.color, Colors.white);

    controller.updateSelectedTextStyle(style: const TextStyle(color: Colors.white), enabled: false);

    final unfocusedSpan = controller.buildTextSpan(context: context, style: const TextStyle(color: Colors.black));
    expect(unfocusedSpan.toPlainText(), 'gh issues');
    expect(unfocusedSpan.children, isNull);
  });
}
