import 'package:flutter/material.dart';

/// A simple error screen that shows a branded image and the error message.
class MagpieErrorScreen extends StatelessWidget {
  final String title;
  final String message;
  final StackTrace? stack;

  const MagpieErrorScreen({
    super.key,
    this.title = 'Something went wrong',
    required this.message,
    this.stack,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Material(
      color: theme.colorScheme.surface,
      child: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24.0),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 520),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Image from package assets
                  Image.asset(
                    'assets/magpie_error.png',
                    package: 'bao',
                    fit: BoxFit.contain,
                    height: 180,
                  ),
                  const SizedBox(height: 16),
                  Text(
                    title,
                    style: theme.textTheme.headlineSmall?.copyWith(
                      color: theme.colorScheme.onSurface,
                      fontWeight: FontWeight.w600,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    message,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  if (stack != null) ...[
                    const SizedBox(height: 16),
                    ExpansionTile(
                      title: const Text('Details'),
                      children: [
                        SingleChildScrollView(
                          scrollDirection: Axis.horizontal,
                          child: Padding(
                            padding: const EdgeInsets.all(12.0),
                            child: Text(
                              stack.toString(),
                              style: theme.textTheme.bodySmall,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Installs a global error widget that shows [MagpieErrorScreen].
/// Call this early (e.g., in main()) for Flutter apps.
void installMagpieErrorScreen({String title = 'Something went wrong'}) {
  ErrorWidget.builder = (FlutterErrorDetails details) {
    final msg = details.exceptionAsString();
    return MagpieErrorScreen(title: title, message: msg, stack: details.stack);
  };
}

/// Shows a dialog with [MagpieErrorScreen]. Useful when you catch a known error.
Future<void> showMagpieErrorDialog(BuildContext context, Object error,
    {String title = 'Something went wrong', StackTrace? stack}) {
  return showDialog<void>(
    context: context,
    builder: (_) => Dialog(
      insetPadding: const EdgeInsets.all(24),
      child: MagpieErrorScreen(
        title: title,
        message: error.toString(),
        stack: stack,
      ),
    ),
  );
}
