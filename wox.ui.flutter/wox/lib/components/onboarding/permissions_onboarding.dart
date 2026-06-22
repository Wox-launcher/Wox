import 'dart:io';

import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/utils/colors.dart';

class WoxPermissionsOnboarding extends StatelessWidget {
  const WoxPermissionsOnboarding({super.key, required this.accent, required this.tr, required this.isPermissionLoading, required this.accessibilityPassed});

  static const _permissionAttentionColor = Color(0xFFF59E0B);
  static const _permissionReadyColor = Color(0xFF22C55E);

  final Color accent;
  final String Function(String key) tr;
  final bool isPermissionLoading;
  final bool? accessibilityPassed;

  @override
  Widget build(BuildContext context) {
    final accessibilityReady = accessibilityPassed == true;
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
            tail: tr(accessibilityReady ? 'onboarding_permission_ready' : 'onboarding_permission_needs_action'),
            tailColor: accessibilityReady ? _permissionReadyColor : _permissionAttentionColor,
          ),
          WoxDemoResult(
            title: tr('onboarding_permission_disk_title'),
            subtitle: tr('onboarding_permission_disk_body'),
            icon: Icon(Icons.folder_open_outlined, color: accent, size: 22),
            tail: tr('onboarding_permission_optional'),
            tailColor: _permissionAttentionColor,
          ),
          WoxDemoResult(
            title: tr('onboarding_permission_privacy_card'),
            subtitle: Platform.isMacOS ? tr('onboarding_permission_open_privacy') : tr('onboarding_permissions_lite_body'),
            icon: Icon(Icons.security_outlined, color: accent, size: 22),
            tail: tr('onboarding_permission_ready'),
            tailColor: _permissionReadyColor,
          ),
          WoxDemoResult(
            title: tr('onboarding_permissions_lite_title'),
            subtitle: tr('onboarding_permissions_lite_body'),
            icon: Icon(Icons.verified_user_outlined, color: accent, size: 22),
            tail: Platform.operatingSystem,
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

    final statusText = isPermissionLoading ? tr('onboarding_permission_checking') : tr('onboarding_permission_needs_action');
    return _PermissionSetupPanel(
      key: const ValueKey('onboarding-permission-macos'),
      rows: [
        _PermissionSetupRow(
          icon: Icons.accessibility_new_rounded,
          iconColor: accessibilityPassed == true ? _permissionReadyColor : _permissionAttentionColor,
          title: tr('onboarding_permission_accessibility_title'),
          body: tr('onboarding_permission_accessibility_body'),
          trailing: [
            accessibilityPassed == true ? _buildPermissionReadyIndicator() : _buildPermissionStatusPill(statusText, _permissionAttentionColor),
            if (accessibilityPassed != true)
              _buildPermissionActionButton(
                text: tr('onboarding_permission_open_accessibility'),
                onPressed: () => WoxApi.instance.openAccessibilityPermission(const UuidV4().generate()),
              ),
          ],
        ),
        _PermissionSetupRow(
          icon: Icons.folder_open_rounded,
          iconColor: _permissionAttentionColor,
          title: tr('onboarding_permission_disk_title'),
          body: tr('onboarding_permission_disk_body'),
          trailing: [
            _buildPermissionStatusPill(tr('onboarding_permission_optional'), _permissionAttentionColor),
            _buildPermissionActionButton(text: tr('onboarding_permission_open_privacy'), onPressed: () => WoxApi.instance.openPrivacyPermission(const UuidV4().generate())),
          ],
        ),
      ],
    );
  }

  Widget _buildPermissionStatusPill(String text, Color color) {
    // Status refinement: pending and optional states are small supporting
    // labels, not primary controls. Keeping them as quiet tinted pills makes
    // the action chip the only clickable element in the row.
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(color: color.withValues(alpha: 0.15), borderRadius: BorderRadius.circular(999), border: Border.all(color: color.withValues(alpha: 0.22))),
      child: Text(text, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: color, fontSize: 12, fontWeight: FontWeight.w700, height: 1.0)),
    );
  }

  Widget _buildPermissionActionButton({required String text, required VoidCallback onPressed}) {
    // Visual refinement: permission shortcuts open macOS System Settings, so a
    // compact filled chip is clearer than the previous heavy outlined button
    // and reads as a secondary row action.
    return TextButton.icon(
      onPressed: onPressed,
      icon: const Icon(Icons.open_in_new_rounded, size: 14),
      label: Text(text, maxLines: 1, overflow: TextOverflow.ellipsis),
      style: ButtonStyle(
        foregroundColor: WidgetStatePropertyAll(getThemeTextColor()),
        backgroundColor: WidgetStateProperty.resolveWith<Color>((states) {
          return getThemeTextColor().withValues(alpha: states.contains(WidgetState.hovered) ? 0.13 : 0.075);
        }),
        overlayColor: WidgetStatePropertyAll(getThemeTextColor().withValues(alpha: 0.08)),
        padding: const WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: 12, vertical: 7)),
        textStyle: const WidgetStatePropertyAll(TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
        shape: WidgetStatePropertyAll(RoundedRectangleBorder(borderRadius: BorderRadius.circular(999))),
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
