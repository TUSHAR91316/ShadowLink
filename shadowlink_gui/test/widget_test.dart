import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:shadowlink_gui/main.dart';
import 'package:shadowlink_gui/screens/dashboard_screen.dart';
import 'package:shadowlink_gui/screens/eula_screen.dart';
import 'package:shadowlink_gui/theme/app_theme.dart';

void main() {
  testWidgets('App loads and displays EULA screen with security terms',
      (WidgetTester tester) async {
    await tester.pumpWidget(const ShadowLinkApp());

    expect(find.byType(MaterialApp), findsOneWidget);
    expect(find.text('SHADOWLINK PROTOCOL'), findsOneWidget);
    expect(find.text('INITIALIZE SHADOWLINK CLIENT'), findsOneWidget);
  });

  testWidgets('EULA acceptance enables initialization button',
      (WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.darkTheme,
        home: const EulaScreen(),
      ),
    );

    // Initial state: button is disabled
    final buttonFinder = find.widgetWithText(
      ElevatedButton,
      'INITIALIZE SHADOWLINK CLIENT',
    );
    expect(buttonFinder, findsOneWidget);
    ElevatedButton btn = tester.widget(buttonFinder);
    expect(btn.onPressed, isNull);

    // Tap the acceptance checkbox
    await tester.tap(find.byType(Checkbox));
    await tester.pump();

    // After checkbox checked: button should be enabled
    btn = tester.widget(buttonFinder);
    expect(btn.onPressed, isNotNull);
  });

  testWidgets('Dashboard renders mobile cyber-defense layout on small screens',
      (WidgetTester tester) async {
    // Set viewport to mobile dimensions (portrait)
    tester.view.physicalSize = const Size(400, 850);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.darkTheme,
        home: const DashboardScreen(),
      ),
    );
    await tester.pump();

    // Verify Mobile Header Branding
    expect(find.text('ShadowLink dVPN'), findsOneWidget);
    expect(find.text('Decentralized Mesh'), findsOneWidget);

    // Verify Mobile Virtual Location Card
    expect(find.text('Zurich, Switzerland'), findsOneWidget);
    expect(find.text('Switch ⇄'), findsOneWidget);

    // Verify Circuit & Defense Badges
    expect(find.text('Multi-Hop Onion Route'), findsOneWidget);
    expect(find.text('ChaCha20-Poly1305'), findsWidgets);

    // Verify Telemetry Grid
    expect(find.text('Download'), findsOneWidget);
    expect(find.text('Upload'), findsOneWidget);
    expect(find.text('Session'), findsOneWidget);

    // Verify Mobile Bottom Navigation Items
    expect(find.text('Shield'), findsOneWidget);
    expect(find.text('Nodes'), findsOneWidget);
    expect(find.text('Traffic'), findsOneWidget);
    expect(find.text('Settings'), findsOneWidget);
  });

  testWidgets('Dashboard renders desktop workstation layout on large screens',
      (WidgetTester tester) async {
    // Set viewport to desktop dimensions
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.darkTheme,
        home: const DashboardScreen(),
      ),
    );
    await tester.pump();

    // Verify Desktop Left Navigation Workstation branding
    expect(find.text('SHADOWLINK'), findsWidgets);
    expect(find.text('dVPN v2.0'), findsOneWidget);
    expect(find.text('Sovereign Identity'), findsOneWidget);
    expect(find.text('0x8f4c...4e19'), findsOneWidget);

    // Verify Central Cyber Map Stage
    expect(find.text('Change Server Node'), findsOneWidget);
    expect(find.text('Select a route and press connect'), findsOneWidget);

    // Verify Right Inspector Panel
    expect(find.text('Multi-Hop Onion Route'), findsOneWidget);
    expect(find.text('Download'), findsOneWidget);
    expect(find.text('Node Operator Controls'), findsOneWidget);
  });

  testWidgets('Server selector bottom sheet opens and displays available nodes',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(400, 850);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.darkTheme,
        home: const DashboardScreen(),
      ),
    );
    await tester.pump();

    // Tap on the virtual location card to trigger exit node selection
    await tester.tap(find.text('Zurich, Switzerland'));
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump(const Duration(milliseconds: 300));

    // Verify bottom sheet modal opened
    expect(find.text('Select Exit Egress Node'), findsOneWidget);
    expect(find.text('Frankfurt, Germany'), findsOneWidget);
    expect(find.text('Reykjavik, Iceland'), findsOneWidget);

    // Select Frankfurt node
    await tester.tap(find.text('Frankfurt, Germany'));
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump(const Duration(milliseconds: 300));

    // Verify bottom sheet closed and Frankfurt is now selected
    expect(find.text('Frankfurt, Germany'), findsOneWidget);
  });
}
