import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:shadowlink_gui/main.dart';

void main() {
  testWidgets('App loads smoke test', (WidgetTester tester) async {
    // Build our app and trigger a frame.
    await tester.pumpWidget(const ShadowLinkApp());

    // Verify that the Eula screen is shown (assuming it has some recognizable text or widget)
    // For now, just make sure the app can build and pump without crashing.
    expect(find.byType(MaterialApp), findsOneWidget);
  });
}
