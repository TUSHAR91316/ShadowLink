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

    // The daemon selects exit peers from the DHT; the UI does not invent locations.
    expect(find.text('Automatic exit selection'), findsOneWidget);
    expect(find.text('Details'), findsOneWidget);

    // Verify Circuit & Defense Badges
    expect(find.text('Circuit Routing'), findsWidgets);
    expect(find.text('ChaCha20-Poly1305'), findsWidgets);

    // Verify Telemetry Grid
    expect(find.text('Download'), findsOneWidget);
    expect(find.text('Upload'), findsOneWidget);
    expect(find.text('Node uptime'), findsOneWidget);

    // Verify Mobile Bottom Navigation Items
    expect(find.text('Shield'), findsOneWidget);
    expect(find.text('Nodes'), findsOneWidget);
    expect(find.text('Traffic'), findsNWidgets(2));
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
    expect(find.text('Local peer identity'), findsOneWidget);
    expect(find.text('Managed by libp2p'), findsOneWidget);

    // Verify Central Cyber Map Stage
    expect(find.text('Automatic Route Details'), findsOneWidget);
    expect(find.text('Exit peers are selected automatically'), findsOneWidget);

    // Verify Right Inspector Panel
    expect(find.text('Circuit Routing'), findsWidgets);
    expect(find.text('Download'), findsOneWidget);
    expect(find.text('Node Operator Controls'), findsOneWidget);
  });

  testWidgets('Route details explain automatic DHT peer selection',
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

    // Open the route details from the automatic route card.
    await tester.tap(find.text('Automatic exit selection'));
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Automatic Exit Selection'), findsOneWidget);
    expect(
      find.textContaining('selects one when each connection is built'),
      findsOneWidget,
    );
  });
}
