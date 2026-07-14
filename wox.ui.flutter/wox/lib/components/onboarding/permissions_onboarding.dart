import 'dart:io';

import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/models/macos_permission_status.dart';
import 'package:wox/utils/colors.dart';

class WoxPermissionsOnboarding extends StatelessWidget {
  const WoxPermissionsOnboarding({super.key, required this.accent, required this.tr, required this.isPermissionLoading, required this.status, required this.onOpenPermission});

  static const _permissionAttentionColor = Color(0xFFF59E0B);
  static const _permissionReadyColor = Color(0xFF22C55E);

  final Color accent;
  final String Function(String key) tr;
  final bool isPermissionLoading;
  final MacOSPermissionStatus status;
  final ValueChanged<String> onOpenPermission;

  @override
  Widget build(BuildContext context) {
    final accessibilityReady = status.accessibility == MacOSPermissionState.granted;
    final fullDiskAccessReady = status.fullDiskAccess == MacOSPermissionState.granted;
    return WoxOnboardingStepLayout(
      previewKey: const ValueKey('onboarding-media-permissions'),
      content: _buildContent(),
      demo: WoxDemoWindow(
        accent: accent,
        query: 'permissions',
        results: [
          WoxDemoResult(
            title: tr('onboarding_permission_accessibility_title'),
            subtitle: tr('onboarding_permission_accessibility_body'),
            icon: const Icon(Icons.accessibility_new_outlined, color: Colors.white, size: 22),
            selected: true,
            tail: tr(accessibilityReady ? 'onboarding_permission_ready' : 'onboarding_permission_authorize'),
            tailColor: accessibilityReady ? _permissionReadyColor : _permissionAttentionColor,
          ),
          WoxDemoResult(
            title: tr('onboarding_permission_disk_title'),
            subtitle: tr('onboarding_permission_disk_body'),
            icon: Icon(Icons.folder_open_outlined, color: accent, size: 22),
            tail: tr(fullDiskAccessReady ? 'onboarding_permission_ready' : 'onboarding_permission_authorize'),
            tailColor: fullDiskAccessReady ? _permissionReadyColor : _permissionAttentionColor,
          ),
        ],
      ),
    );
  }

  Widget _buildContent() {
    if (!Platform.isMacOS) {
      return WoxOnboardingInfoPanel(
        key: const ValueKey('onboarding-permission-lite'),
        title: tr('onboarding_permissions_lite_title'),
        body: tr('onboarding_permissions_lite_body'),
        badge: Platform.operatingSystem,
      );
    }

    final statusText = isPermissionLoading ? tr('onboarding_permission_checking') : tr('onboarding_permission_authorize');
    return _PermissionSetupPanel(
      key: const ValueKey('onboarding-permission-macos'),
      rows: [
        _buildPermissionRow(
          icon: Icons.accessibility_new_rounded,
          state: status.accessibility,
          title: tr('onboarding_permission_accessibility_title'),
          body: tr('onboarding_permission_accessibility_body'),
          onPressed: () => onOpenPermission('accessibility'),
          pendingText: statusText,
        ),
        _buildPermissionRow(
          icon: Icons.folder_open_rounded,
          state: status.fullDiskAccess,
          title: tr('onboarding_permission_disk_title'),
          body: tr('onboarding_permission_disk_body'),
          onPressed: () => onOpenPermission('fullDiskAccess'),
          pendingText: statusText,
        ),
      ],
    );
  }

  Widget _buildPermissionRow({
    required IconData icon,
    required MacOSPermissionState state,
    required String title,
    required String body,
    required VoidCallback onPressed,
    required String pendingText,
  }) {
    final ready = state == MacOSPermissionState.granted;
    return _PermissionSetupRow(
      icon: icon,
      iconColor: ready ? _permissionReadyColor : _permissionAttentionColor,
      title: title,
      body: body,
      trailing: [if (ready) _buildPermissionReadyIndicator() else _buildPermissionActionButton(text: pendingText, onPressed: isPermissionLoading ? null : onPressed)],
    );
  }

  Widget _buildPermissionActionButton({required String text, required VoidCallback? onPressed}) {
    // The permission state and action share one control so every pending row has a single clear next step.
    return TextButton.icon(
      onPressed: onPressed,
      icon: const Icon(Icons.open_in_new_rounded, size: 14),
      label: Text(text, maxLines: 1, overflow: TextOverflow.ellipsis),
      style: ButtonStyle(
        foregroundColor: const WidgetStatePropertyAll(_permissionAttentionColor),
        backgroundColor: WidgetStateProperty.resolveWith<Color>((states) {
          return _permissionAttentionColor.withValues(alpha: states.contains(WidgetState.hovered) ? 0.20 : 0.12);
        }),
        overlayColor: WidgetStatePropertyAll(_permissionAttentionColor.withValues(alpha: 0.10)),
        padding: const WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: 12, vertical: 7)),
        textStyle: const WidgetStatePropertyAll(TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
        shape: WidgetStatePropertyAll(RoundedRectangleBorder(borderRadius: BorderRadius.circular(999), side: BorderSide(color: _permissionAttentionColor, width: 1))),
        minimumSize: const WidgetStatePropertyAll(Size.zero),
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
      ),
    );
  }

  Widget _buildPermissionReadyIndicator() {
    // Completed permissions do not need another action. A green check keeps the
    // row aligned with actionable states while avoiding a redundant "Ready"
    // badge that competes with the permission title.
    // Tooltip styling is centralized in WoxTooltip so onboarding status affordances
    // do not drift from the launcher and settings hover overlays.
    return WoxTooltip(
      message: tr('onboarding_permission_ready'),
      child: Container(
        width: 32,
        height: 32,
        decoration: BoxDecoration(
          color: _permissionReadyColor.withValues(alpha: 0.14),
          borderRadius: BorderRadius.circular(999),
          border: Border.all(color: _permissionReadyColor.withValues(alpha: 0.32)),
        ),
        child: const Icon(Icons.check_rounded, color: _permissionReadyColor, size: 19),
      ),
    );
  }
}

class _PermissionSetupPanel extends StatelessWidget {
  const _PermissionSetupPanel({super.key, required this.rows});

  final List<Widget> rows;

  @override
  Widget build(BuildContext context) {
    // Permission redesign: the macOS permission step is a compact checklist,
    // not two independent cards. One shared panel keeps scanning predictable
    // and gives the right-side status/action area a stable edge.
    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        color: getThemeTextColor().withValues(alpha: 0.038),
        border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.18)),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (var index = 0; index < rows.length; index++) ...[
            rows[index],
            if (index != rows.length - 1) Divider(height: 1, thickness: 1, color: getThemeSubTextColor().withValues(alpha: 0.12)),
          ],
        ],
      ),
    );
  }
}

class _PermissionSetupRow extends StatelessWidget {
  const _PermissionSetupRow({required this.icon, required this.iconColor, required this.title, required this.body, required this.trailing});

  final IconData icon;
  final Color iconColor;
  final String title;
  final String body;
  final List<Widget> trailing;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          // Row identity: an icon tile separates permission type from status,
          // so the right side can focus only on state and the next action.
          Container(
            width: 38,
            height: 38,
            decoration: BoxDecoration(
              color: iconColor.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: iconColor.withValues(alpha: 0.24)),
            ),
            child: Icon(icon, color: iconColor, size: 21),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 16, fontWeight: FontWeight.w700)),
                const SizedBox(height: 6),
                Text(body, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(color: subTextColor, fontSize: 13, height: 1.35)),
              ],
            ),
          ),
          const SizedBox(width: 18),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 260),
            child: Wrap(alignment: WrapAlignment.end, crossAxisAlignment: WrapCrossAlignment.center, spacing: 8, runSpacing: 8, children: trailing),
          ),
        ],
      ),
    );
  }
}
