import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_panel.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_cloud_sync.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';

enum _CloudSyncAccountAction { changePassword, logout }

enum _CloudSyncSubscriptionAction { refreshStatus, manageSubscription }

class WoxSettingCloudSyncView extends WoxSettingBaseView {
  const WoxSettingCloudSyncView({super.key});

  static const String _pluginExclusionTableKey = "CloudSyncDisabledPluginsTable";
  static const String _pluginExclusionPluginIdKey = "PluginId";
  static const double _cloudSyncLabelWidth = 420.0;
  static const double _cloudSyncValueWidth = GENERAL_SETTING_WIDE_FORM_WIDTH - _cloudSyncLabelWidth - 32.0;

  Widget buildCloudSyncInfoValue(String value, {Color? color, int? maxLines, TextOverflow? overflow}) {
    return Text(value, style: TextStyle(color: color ?? getThemeTextColor(), fontSize: 13, height: 1.35), maxLines: maxLines, overflow: overflow, softWrap: maxLines == null);
  }

  String formatCloudSyncTime(int timestamp) {
    if (timestamp <= 0) {
      return controller.tr("ui_cloud_sync_never");
    }
    final date = DateTime.fromMillisecondsSinceEpoch(timestamp);
    return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')} '
        '${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}:${date.second.toString().padLeft(2, '0')}';
  }

  String normalizeCloudSyncError(String error) {
    if (error.contains('cloud sync is not configured') || error.contains('account is not logged in')) {
      return controller.tr("ui_cloud_sync_not_configured");
    }
    if (error.contains('subscription_required')) {
      return controller.tr("ui_cloud_sync_subscription_required");
    }
    if (error.contains('failed to decrypt payload') || error.contains('message authentication failed')) {
      return controller.tr("ui_cloud_sync_recovery_code_invalid");
    }
    return error;
  }

  int lastCloudSyncTimestamp(WoxCloudSyncState? state) {
    if (state == null) {
      return 0;
    }
    return state.lastPullTs > state.lastPushTs ? state.lastPullTs : state.lastPushTs;
  }

  String normalizeAccountActionError(String error) {
    if (error.contains('invalid_current_password')) {
      return controller.tr("ui_cloud_sync_account_current_password_invalid");
    }
    if (error.contains('password_too_short') || error.contains('invalid_password')) {
      return controller.tr("ui_cloud_sync_account_password_min_length");
    }
    if (error.contains('invalid_email')) {
      return controller.tr("ui_cloud_sync_account_error_invalid_email");
    }
    if (error.contains('invalid_verification_code')) {
      return controller.tr("ui_cloud_sync_account_verify_code_failed");
    }
    if (error.contains('subscription_required')) {
      return controller.tr("ui_cloud_sync_subscription_required");
    }
    if (error.contains('unauthorized')) {
      return controller.tr("ui_cloud_sync_account_session_expired");
    }
    if (error.contains('already_registered') || error.contains('already exists') || error.contains('account_exists')) {
      return controller.tr("ui_cloud_sync_account_error_exists");
    }
    return error.replaceFirst('Exception: ', '');
  }

  Future<WoxAccountActionResult?> showAccountLoginDialog(BuildContext context) async {
    return await showDialog<WoxAccountActionResult>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _AccountLoginDialog(controller: controller, normalizeAccountActionError: normalizeAccountActionError),
    );
  }

  Future<WoxAccountActionResult?> showAccountRegisterDialog(BuildContext context) async {
    return await showDialog<WoxAccountActionResult>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _AccountRegisterDialog(controller: controller, normalizeAccountActionError: normalizeAccountActionError),
    );
  }

  Future<bool?> showAccountVerifyEmailDialog(BuildContext context, String email) async {
    return await showDialog<bool>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _AccountVerifyEmailDialog(controller: controller, email: email, normalizeAccountActionError: normalizeAccountActionError),
    );
  }

  // Continues account actions into email verification without treating cancellation as login success.
  Future<void> showEmailVerificationIfNeeded(BuildContext context, WoxAccountActionResult? result) async {
    if (result == null || !result.needsEmailVerification) {
      return;
    }

    final email = result.email.isNotEmpty ? result.email : controller.accountStatus.value.email;
    if (email.isEmpty || !context.mounted) {
      return;
    }
    await showAccountVerifyEmailDialog(context, email);
  }

  Future<bool?> showAccountChangePasswordDialog(BuildContext context) async {
    return await showDialog<bool>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _AccountChangePasswordDialog(controller: controller, normalizeAccountActionError: normalizeAccountActionError),
    );
  }

  // Opens a pre-addressed support email for billing issues.
  Future<void> openBillingSupportEmail() async {
    final uri = Uri(scheme: 'mailto', path: 'billing@woxlauncher.com', queryParameters: {'subject': controller.tr("ui_cloud_sync_billing_help_email_subject")});
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  Future<String?> showTokenDialog(BuildContext context, String title, String hint) async {
    final tokenController = TextEditingController();
    try {
      return await showDialog<String>(
        context: context,
        barrierColor: getThemePopupBarrierColor(),
        builder: (context) {
          return WoxDialog(
            title: Text(title),
            content: WoxTextField(controller: tokenController, hintText: hint, width: 360),
            actions: [
              WoxButton.secondary(text: controller.tr("ui_cloud_sync_cancel"), onPressed: () => Navigator.pop(context)),
              WoxButton.primary(text: controller.tr("ui_cloud_sync_confirm"), onPressed: () => Navigator.pop(context, tokenController.text.trim())),
            ],
          );
        },
      );
    } finally {
      tokenController.dispose();
    }
  }

  Future<Map<String, String>?> showResetPasswordDialog(BuildContext context) async {
    final tokenController = TextEditingController();
    final passwordController = TextEditingController();
    try {
      return await showDialog<Map<String, String>>(
        context: context,
        barrierColor: getThemePopupBarrierColor(),
        builder: (context) {
          return WoxDialog(
            title: Text(controller.tr("ui_cloud_sync_account_reset_confirm")),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                WoxTextField(controller: tokenController, hintText: controller.tr("ui_cloud_sync_account_reset_token"), width: 360),
                const SizedBox(height: 10),
                WoxTextField(controller: passwordController, hintText: controller.tr("ui_cloud_sync_account_new_password"), width: 360, obscureText: true),
              ],
            ),
            actions: [
              WoxButton.secondary(text: controller.tr("ui_cloud_sync_cancel"), onPressed: () => Navigator.pop(context)),
              WoxButton.primary(
                text: controller.tr("ui_cloud_sync_confirm"),
                onPressed: () => Navigator.pop(context, {"token": tokenController.text.trim(), "password": passwordController.text}),
              ),
            ],
          );
        },
      );
    } finally {
      tokenController.dispose();
      passwordController.dispose();
    }
  }

  Future<void> showRecoveryCodeDialog(BuildContext context, String code) async {
    await showDialog(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) {
        return WoxDialog(
          title: Text(controller.tr("ui_cloud_sync_recovery_code_title")),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SelectableText(code, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
              const SizedBox(height: 10),
              Text(controller.tr("ui_cloud_sync_recovery_code_tips"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
            ],
          ),
          actions: [
            WoxButton.secondary(
              text: controller.tr("ui_cloud_sync_recovery_code_copy"),
              onPressed: () {
                Clipboard.setData(ClipboardData(text: code));
              },
            ),
            WoxButton.primary(
              text: controller.tr("ui_cloud_sync_recovery_code_close"),
              onPressed: () {
                Navigator.pop(context);
              },
            ),
          ],
        );
      },
    );
  }

  Future<Map<String, String>?> showCloudSyncInitKeyDialog(BuildContext context) async {
    final recoveryController = TextEditingController();
    final deviceController = TextEditingController();
    try {
      return await showDialog<Map<String, String>>(
        context: context,
        barrierColor: getThemePopupBarrierColor(),
        builder: (context) {
          return WoxDialog(
            title: Text(controller.tr("ui_cloud_sync_key_init_title")),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(controller.tr("ui_cloud_sync_recovery_code_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 6),
                WoxTextField(controller: recoveryController, hintText: controller.tr("ui_cloud_sync_recovery_code_hint"), width: 360),
                const SizedBox(height: 12),
                Text(controller.tr("ui_cloud_sync_device_name"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 6),
                WoxTextField(controller: deviceController, hintText: controller.tr("ui_cloud_sync_device_name_hint"), width: 360),
              ],
            ),
            actions: [
              WoxButton.secondary(
                text: controller.tr("ui_cloud_sync_cancel"),
                onPressed: () {
                  Navigator.pop(context);
                },
              ),
              WoxButton.primary(
                text: controller.tr("ui_cloud_sync_confirm"),
                onPressed: () {
                  Navigator.pop(context, {"recoveryCode": recoveryController.text.trim(), "deviceName": deviceController.text.trim()});
                },
              ),
            ],
          );
        },
      );
    } finally {
      recoveryController.dispose();
      deviceController.dispose();
    }
  }

  Future<String?> showCloudSyncFetchKeyDialog(BuildContext context) async {
    final recoveryController = TextEditingController();
    try {
      return await showDialog<String>(
        context: context,
        barrierColor: getThemePopupBarrierColor(),
        builder: (context) {
          return WoxDialog(
            title: Text(controller.tr("ui_cloud_sync_key_fetch_title")),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(controller.tr("ui_cloud_sync_recovery_code_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 6),
                WoxTextField(controller: recoveryController, hintText: controller.tr("ui_cloud_sync_recovery_code_hint"), width: 360),
              ],
            ),
            actions: [
              WoxButton.secondary(
                text: controller.tr("ui_cloud_sync_cancel"),
                onPressed: () {
                  Navigator.pop(context);
                },
              ),
              WoxButton.primary(
                text: controller.tr("ui_cloud_sync_confirm"),
                onPressed: () {
                  Navigator.pop(context, recoveryController.text.trim());
                },
              ),
            ],
          );
        },
      );
    } finally {
      recoveryController.dispose();
    }
  }

  Future<bool?> showCloudSyncResetDialog(BuildContext context) async {
    return await showDialog<bool>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) {
        return WoxDialog(
          title: Text(controller.tr("ui_cloud_sync_reset_title")),
          content: Text(controller.tr("ui_cloud_sync_reset_warning"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          actions: [
            WoxButton.secondary(
              text: controller.tr("ui_cloud_sync_cancel"),
              onPressed: () {
                Navigator.pop(context, false);
              },
            ),
            WoxButton.primary(
              text: controller.tr("ui_cloud_sync_reset_confirm"),
              onPressed: () {
                Navigator.pop(context, true);
              },
            ),
          ],
        );
      },
    );
  }

  Future<void> showCloudSyncBootstrapDialog(BuildContext context, WoxCloudSyncBootstrapStatus status) async {
    const dialogContentWidth = 360.0;
    final recoveryController = TextEditingController();
    final confirmRecoveryController = TextEditingController();
    try {
      await showDialog<void>(
        context: context,
        barrierColor: getThemePopupBarrierColor(),
        builder: (context) {
          String? validationError;
          bool isSubmitting = false;

          Future<void> submit(StateSetter setDialogState) async {
            if (isSubmitting) {
              return;
            }
            final recoveryCode = recoveryController.text.trim();
            final confirmRecoveryCode = confirmRecoveryController.text.trim();
            String? nextError;
            if (recoveryCode.isEmpty) {
              nextError = controller.tr("ui_cloud_sync_recovery_code_required");
            } else if (!status.hasRemoteData && confirmRecoveryCode.isEmpty) {
              nextError = controller.tr("ui_cloud_sync_recovery_code_confirm_required");
            } else if (!status.hasRemoteData && recoveryCode != confirmRecoveryCode) {
              nextError = controller.tr("ui_cloud_sync_recovery_code_mismatch");
            }
            if (nextError != null) {
              setDialogState(() {
                validationError = nextError;
              });
              return;
            }

            setDialogState(() {
              validationError = null;
              isSubmitting = true;
            });
            final started = await controller.cloudSyncBootstrapStart(recoveryCode);
            if (!context.mounted) {
              return;
            }
            if (started) {
              Navigator.pop(context);
              return;
            }
            setDialogState(() {
              validationError = normalizeCloudSyncError(controller.cloudSyncActionError.value);
              isSubmitting = false;
            });
          }

          return StatefulBuilder(
            builder: (context, setDialogState) {
              return WoxDialog(
                title: Text(status.hasRemoteData ? controller.tr("ui_cloud_sync_bootstrap_restore_title") : controller.tr("ui_cloud_sync_bootstrap_start_title")),
                content: SizedBox(
                  width: dialogContentWidth,
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        status.hasRemoteData ? controller.tr("ui_cloud_sync_bootstrap_restore_description") : controller.tr("ui_cloud_sync_bootstrap_start_description"),
                        style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.45),
                      ),
                      const SizedBox(height: 12),
                      Text(controller.tr("ui_cloud_sync_recovery_code"), style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600)),
                      const SizedBox(height: 6),
                      WoxTextField(
                        controller: recoveryController,
                        hintText: controller.tr("ui_cloud_sync_recovery_code_hint"),
                        width: dialogContentWidth,
                        obscureText: true,
                        enabled: !isSubmitting,
                        onSubmitted: (_) => submit(setDialogState),
                      ),
                      if (!status.hasRemoteData) ...[
                        const SizedBox(height: 12),
                        Text(controller.tr("ui_cloud_sync_recovery_code_confirm"), style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600)),
                        const SizedBox(height: 6),
                        WoxTextField(
                          controller: confirmRecoveryController,
                          hintText: controller.tr("ui_cloud_sync_recovery_code_confirm_hint"),
                          width: dialogContentWidth,
                          obscureText: true,
                          enabled: !isSubmitting,
                          onSubmitted: (_) => submit(setDialogState),
                        ),
                      ],
                      if (validationError != null) ...[const SizedBox(height: 10), Text(validationError!, style: const TextStyle(color: Colors.red, fontSize: 12))],
                      if (validationError == null && isSubmitting) ...[
                        const SizedBox(height: 10),
                        Text(controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                      ],
                    ],
                  ),
                ),
                actions: [
                  WoxButton.secondary(text: controller.tr("ui_cloud_sync_cancel"), onPressed: isSubmitting ? null : () => Navigator.pop(context)),
                  WoxButton.primary(
                    text: isSubmitting ? controller.tr("ui_cloud_sync_loading") : controller.tr("ui_cloud_sync_confirm"),
                    onPressed: isSubmitting ? null : () => submit(setDialogState),
                  ),
                ],
              );
            },
          );
        },
      );
    } finally {
      recoveryController.dispose();
      confirmRecoveryController.dispose();
    }
  }

  Widget buildCloudSyncStatusSection() {
    return Obx(() {
      final status = controller.cloudSyncStatus.value;
      final account = controller.accountStatus.value;
      final state = status.state;
      final isLoading = controller.isCloudSyncStatusLoading.value;
      final statusError = controller.cloudSyncStatusError.value;
      final actionError = controller.cloudSyncActionError.value;
      final stateError = state?.lastError ?? '';
      final lastSyncTime = formatCloudSyncTime(lastCloudSyncTimestamp(state));
      final String statusText;
      final Color? statusColor;
      final String? detailText;
      if (isLoading) {
        statusText = controller.tr("ui_cloud_sync_loading");
        statusColor = getThemeSubTextColor();
        detailText = null;
      } else if (statusError.isNotEmpty) {
        statusText = "${controller.tr("ui_cloud_sync_sync_error")}: ${normalizeCloudSyncError(statusError)}";
        statusColor = Colors.red;
        detailText = null;
      } else if (actionError.isNotEmpty) {
        statusText = "${controller.tr("ui_cloud_sync_sync_error")}: ${normalizeCloudSyncError(actionError)}";
        statusColor = Colors.red;
        detailText = null;
      } else if (stateError.isNotEmpty) {
        statusText = "${controller.tr("ui_cloud_sync_sync_error")}: ${normalizeCloudSyncError(stateError)}";
        statusColor = Colors.red;
        detailText = null;
      } else if (account.loggedIn && account.syncEligible && (!account.syncEnabled || !status.keyStatus.available || state == null || !state.bootstrapped)) {
        statusText = controller.tr("ui_cloud_sync_unsynced");
        statusColor = getThemeSubTextColor();
        detailText = null;
      } else if (!account.syncEnabled || !status.enabled) {
        statusText = controller.tr("ui_cloud_sync_disabled");
        statusColor = getThemeSubTextColor();
        detailText = null;
      } else if (!status.keyStatus.available) {
        statusText = "${controller.tr("ui_cloud_sync_sync_error")}: ${controller.tr("ui_cloud_sync_key_missing")}";
        statusColor = Colors.red;
        detailText = null;
      } else if (state != null && !state.bootstrapped) {
        statusText = "${controller.tr("ui_cloud_sync_sync_error")}: ${controller.tr("ui_cloud_sync_not_initialized")}";
        statusColor = Colors.red;
        detailText = null;
      } else {
        statusText = controller.tr("ui_cloud_sync_synced");
        statusColor = null;
        detailText = "${controller.tr("ui_cloud_sync_last_sync_time")}: $lastSyncTime";
      }

      return formSection(
        title: controller.tr("ui_cloud_sync_status_label"),
        children: [
          formField(
            label: controller.tr("ui_cloud_sync_sync_status"),
            labelWidth: _cloudSyncLabelWidth,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                buildCloudSyncInfoValue(statusText, color: statusColor),
                if (detailText != null) ...[const SizedBox(height: 4), buildCloudSyncInfoValue(detailText, color: getThemeSubTextColor())],
              ],
            ),
          ),
        ],
      );
    });
  }

  Widget buildAccountSection(BuildContext context) {
    return Obx(() {
      final account = controller.accountStatus.value;
      final isBusy = controller.isCloudSyncActionLoading.value;
      final isBillingWaiting = controller.isAccountBillingWaiting.value;
      final billingWaitingMessageKey = controller.accountBillingWaitingMessageKey.value;
      final subscriptionError = controller.accountSubscriptionError.value;
      final subscriptionActive = account.syncEligible;
      final billingWaitingText = billingWaitingMessageKey.isNotEmpty ? controller.tr(billingWaitingMessageKey) : controller.tr("ui_cloud_sync_subscription_waiting_payment");
      final subscriptionStatusText =
          isBillingWaiting
              ? billingWaitingText
              : subscriptionError.isNotEmpty
              ? normalizeAccountActionError(subscriptionError)
              : subscriptionActive
              ? controller.tr("ui_cloud_sync_subscription_active")
              : controller.tr("ui_cloud_sync_subscription_required");
      final subscriptionStatusColor =
          isBillingWaiting
              ? getThemeSubTextColor()
              : subscriptionError.isNotEmpty || !subscriptionActive
              ? Colors.red
              : null;
      return formSection(
        title: controller.tr("ui_cloud_sync_account"),
        children: [
          if (account.loggedIn) ...[
            formField(
              label: controller.tr("ui_cloud_sync_account_email"),
              labelWidth: _cloudSyncLabelWidth,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [Flexible(child: buildCloudSyncInfoValue(account.email)), const SizedBox(width: 6), buildAccountActionMenu(context: context, isBusy: isBusy)],
              ),
            ),
            formField(
              label: controller.tr("ui_cloud_sync_subscription_status"),
              labelWidth: _cloudSyncLabelWidth,
              child: SizedBox(
                width: _cloudSyncValueWidth,
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    Expanded(
                      child: Align(
                        alignment: Alignment.centerRight,
                        child: buildCloudSyncInfoValue(subscriptionStatusText, color: subscriptionStatusColor, maxLines: 1, overflow: TextOverflow.ellipsis),
                      ),
                    ),
                    const SizedBox(width: 10),
                    if (!subscriptionActive)
                      WoxButton.primary(text: controller.tr("ui_cloud_sync_subscribe"), onPressed: isBusy || isBillingWaiting ? null : () => controller.accountOpenCheckout()),
                    if (subscriptionActive) buildSubscriptionActionMenu(isBusy: isBusy, isBillingWaiting: isBillingWaiting),
                  ],
                ),
              ),
            ),
            formField(
              label: controller.tr("ui_cloud_sync_billing_help"),
              labelWidth: _cloudSyncLabelWidth,
              tips: controller.tr("ui_cloud_sync_billing_help_tips"),
              child: SizedBox(
                width: _cloudSyncValueWidth,
                child: Align(
                  alignment: Alignment.centerRight,
                  child: WoxButton.secondary(
                    text: controller.tr("ui_cloud_sync_contact_support"),
                    icon: const Icon(Icons.email_outlined, size: 16),
                    onPressed: () async => openBillingSupportEmail(),
                  ),
                ),
              ),
            ),
          ] else
            formField(
              label: controller.tr("ui_cloud_sync_account"),
              labelWidth: _cloudSyncLabelWidth,
              child: Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  WoxButton.primary(
                    text: controller.tr("ui_cloud_sync_account_login"),
                    onPressed: () async {
                      final result = await showAccountLoginDialog(context);
                      if (context.mounted) {
                        await showEmailVerificationIfNeeded(context, result);
                      }
                    },
                  ),
                  WoxButton.secondary(
                    text: controller.tr("ui_cloud_sync_account_register"),
                    onPressed: () async {
                      final result = await showAccountRegisterDialog(context);
                      if (context.mounted) {
                        await showEmailVerificationIfNeeded(context, result);
                      }
                    },
                  ),
                ],
              ),
            ),
        ],
      );
    });
  }

  // Keeps account-level commands reachable without making the logged-in account summary look action-heavy.
  Widget buildAccountActionMenu({required BuildContext context, required bool isBusy}) {
    final textStyle = TextStyle(color: getThemeTextColor(), fontSize: 13);
    return PopupMenuButton<_CloudSyncAccountAction>(
      enabled: !isBusy,
      tooltip: '',
      color: getThemePopupSurfaceColor(),
      elevation: 8,
      offset: const Offset(0, 8),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6), side: BorderSide(color: getThemePopupOutlineColor())),
      onSelected: (action) async {
        switch (action) {
          case _CloudSyncAccountAction.changePassword:
            if (!context.mounted) {
              return;
            }
            await showAccountChangePasswordDialog(context);
            break;
          case _CloudSyncAccountAction.logout:
            await controller.accountLogout();
            break;
        }
      },
      itemBuilder:
          (context) => [
            PopupMenuItem(value: _CloudSyncAccountAction.changePassword, child: Text(controller.tr("ui_cloud_sync_account_change_password"), style: textStyle)),
            PopupMenuItem(value: _CloudSyncAccountAction.logout, child: Text(controller.tr("ui_cloud_sync_account_logout"), style: textStyle)),
          ],
      child: Container(
        width: 28,
        height: 28,
        alignment: Alignment.center,
        child: Icon(Icons.arrow_drop_down, size: 20, color: getThemeTextColor().withValues(alpha: isBusy ? 0.36 : 0.76)),
      ),
    );
  }

  // Keeps subscription-only actions next to the subscription status instead of mixing them into account commands.
  Widget buildSubscriptionActionMenu({required bool isBusy, required bool isBillingWaiting}) {
    final textStyle = TextStyle(color: getThemeTextColor(), fontSize: 13);
    return PopupMenuButton<_CloudSyncSubscriptionAction>(
      enabled: !isBusy,
      tooltip: '',
      color: getThemePopupSurfaceColor(),
      elevation: 8,
      offset: const Offset(0, 8),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6), side: BorderSide(color: getThemePopupOutlineColor())),
      onSelected: (action) async {
        switch (action) {
          case _CloudSyncSubscriptionAction.refreshStatus:
            await controller.accountRefreshSubscriptionStatus();
            break;
          case _CloudSyncSubscriptionAction.manageSubscription:
            await controller.accountOpenBillingPortal();
            break;
        }
      },
      itemBuilder:
          (context) => [
            PopupMenuItem(value: _CloudSyncSubscriptionAction.refreshStatus, child: Text(controller.tr("ui_cloud_sync_refresh_status"), style: textStyle)),
            PopupMenuItem(
              value: _CloudSyncSubscriptionAction.manageSubscription,
              enabled: !isBillingWaiting,
              child: Text(controller.tr("ui_cloud_sync_manage_subscription"), style: textStyle),
            ),
          ],
      child: Container(
        width: 28,
        height: 28,
        alignment: Alignment.center,
        child: Icon(Icons.arrow_drop_down, size: 20, color: getThemeTextColor().withValues(alpha: isBusy ? 0.36 : 0.76)),
      ),
    );
  }

  Widget buildCloudSyncIntroSection() {
    return formSection(
      title: controller.tr("ui_cloud_sync_intro_title"),
      children: [
        WoxPanel(
          padding: const EdgeInsets.all(22),
          showShadow: false,
          child: LayoutBuilder(
            builder: (context, constraints) {
              final useStackedLayout = constraints.maxWidth < 760;
              final featureItems = [
                _CloudSyncIntroFeature(
                  icon: Icons.settings_outlined,
                  title: controller.tr("ui_cloud_sync_intro_settings_title"),
                  description: controller.tr("ui_cloud_sync_intro_settings_description"),
                ),
                _CloudSyncIntroFeature(
                  icon: Icons.extension_outlined,
                  title: controller.tr("ui_cloud_sync_intro_plugins_title"),
                  description: controller.tr("ui_cloud_sync_intro_plugins_description"),
                ),
                _CloudSyncIntroFeature(
                  icon: Icons.key_outlined,
                  title: controller.tr("ui_cloud_sync_intro_keys_title"),
                  description: controller.tr("ui_cloud_sync_intro_keys_description"),
                ),
              ];

              final mainContent = Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _buildCloudSyncIntroIcon(Icons.cloud_queue_outlined, size: 56, iconSize: 28),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              controller.tr("ui_cloud_sync_intro_headline"),
                              style: TextStyle(color: getThemeTextColor(), fontSize: 20, fontWeight: FontWeight.w700, height: 1.2),
                            ),
                            const SizedBox(height: 8),
                            Text(controller.tr("ui_cloud_sync_intro_description"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.4)),
                          ],
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 20),
                  Container(height: 1, color: getThemeSettingDividerColor().withValues(alpha: isThemeDark() ? 0.72 : 0.48)),
                ],
              );
              final featuresContent = Column(
                children: [
                  for (var i = 0; i < featureItems.length; i++) ...[_buildCloudSyncIntroFeature(featureItems[i]), if (i < featureItems.length - 1) const SizedBox(height: 14)],
                ],
              );

              final priceContent = buildCloudSyncIntroPrice();
              if (useStackedLayout) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    mainContent,
                    const SizedBox(height: 18),
                    featuresContent,
                    const SizedBox(height: 20),
                    Container(height: 1, color: getThemeSettingDividerColor().withValues(alpha: isThemeDark() ? 0.72 : 0.48)),
                    const SizedBox(height: 18),
                    priceContent,
                  ],
                );
              }

              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  mainContent,
                  const SizedBox(height: 18),
                  IntrinsicHeight(
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Expanded(child: featuresContent),
                        Padding(padding: const EdgeInsets.symmetric(horizontal: 28), child: Container(width: 1, color: getThemeSettingDividerColor().withValues(alpha: 0.72))),
                        SizedBox(width: 300, child: Center(child: priceContent)),
                      ],
                    ),
                  ),
                ],
              );
            },
          ),
        ),
      ],
    );
  }

  /// Builds the shared icon treatment used by the cloud sync intro panel.
  Widget _buildCloudSyncIntroIcon(IconData icon, {double size = 42, double iconSize = 22}) {
    final accentColor = getThemeActionItemActiveColor();
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.18 : 0.10),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: accentColor.withValues(alpha: isThemeDark() ? 0.34 : 0.22)),
      ),
      child: Icon(icon, size: iconSize, color: accentColor),
    );
  }

  /// Builds one scannable feature row in the cloud sync intro panel.
  Widget _buildCloudSyncIntroFeature(_CloudSyncIntroFeature feature) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildCloudSyncIntroIcon(feature.icon),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(feature.title, style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w700, height: 1.2)),
              const SizedBox(height: 6),
              Text(feature.description, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.35)),
            ],
          ),
        ),
      ],
    );
  }

  /// Builds the pricing summary without implying an unconfirmed billing state.
  Widget buildCloudSyncIntroPrice() {
    return SizedBox(
      width: 250,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              _buildCloudSyncIntroIcon(Icons.local_offer_outlined, size: 48, iconSize: 24),
              const SizedBox(width: 14),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(controller.tr("ui_cloud_sync_intro_price_label"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, fontWeight: FontWeight.w600)),
                  const SizedBox(height: 4),
                  Text(controller.tr("ui_cloud_sync_intro_price_value"), style: TextStyle(color: getThemeTextColor(), fontSize: 22, fontWeight: FontWeight.w700, height: 1.1)),
                ],
              ),
            ],
          ),
          const SizedBox(height: 18),
          Text(controller.tr("ui_cloud_sync_intro_price_description"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.45)),
        ],
      ),
    );
  }

  Widget buildCloudSyncActionSection(BuildContext context) {
    return Obx(() {
      final status = controller.cloudSyncStatus.value;
      final account = controller.accountStatus.value;
      final isBusy = controller.isCloudSyncActionLoading.value;
      final state = status.state;
      final isSynced = account.syncEnabled && status.keyStatus.available && state != null && state.bootstrapped;
      final canStartSync = account.loggedIn && account.syncEligible && !isSynced && controller.cloudSyncStatusError.value.isEmpty;
      if (!canStartSync) {
        return const SizedBox.shrink();
      }

      return formSection(
        title: controller.tr("ui_operation"),
        children: [
          formField(
            label: controller.tr("ui_operation"),
            labelWidth: _cloudSyncLabelWidth,
            child: Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                if (canStartSync)
                  WoxButton.primary(
                    text: controller.tr("ui_cloud_sync_sync"),
                    onPressed:
                        !isBusy
                            ? () async {
                              final bootstrapStatus = await controller.cloudSyncBootstrapStatus();
                              if (!context.mounted || bootstrapStatus == null) {
                                return;
                              }
                              await showCloudSyncBootstrapDialog(context, bootstrapStatus);
                            }
                            : null,
                  ),
              ],
            ),
          ),
        ],
      );
    });
  }

  Widget buildCloudSyncPluginExclusions() {
    return Obx(() {
      final disabledPluginIds = controller.woxSetting.value.cloudSyncDisabledPlugins;
      if (controller.isCloudSyncPluginListLoading.value) {
        return Text(controller.tr("ui_cloud_sync_plugin_exclusions_loading"));
      }
      if (controller.installedPlugins.isEmpty && disabledPluginIds.isEmpty) {
        return Text(controller.tr("ui_cloud_sync_plugin_exclusions_empty"));
      }

      return WoxSettingPluginTable(
        inlineTitleActions: true,
        tableWidth: GENERAL_SETTING_TABLE_WIDTH,
        showCloneAction: false,
        value: _encodePluginExclusionRows(disabledPluginIds),
        trailingActions: [
          WoxButton.secondary(
            text: controller.tr("ui_cloud_sync_plugin_exclusions_refresh"),
            height: 30,
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            onPressed: () async {
              controller.isCloudSyncPluginListLoading.value = true;
              try {
                await controller.loadInstalledPlugins(const UuidV4().generate());
              } finally {
                controller.isCloudSyncPluginListLoading.value = false;
              }
            },
          ),
        ],
        item: _buildPluginExclusionTableDefinition(disabledPluginIds),
        onUpdate: (key, value) async {
          final pluginIds = _decodePluginExclusionRows(value);
          await controller.updateCloudSyncDisabledPlugins(pluginIds);
          return null;
        },
      );
    });
  }

  // Builds the editable table definition while keeping stale excluded plugin IDs selectable for existing rows.
  PluginSettingValueTable _buildPluginExclusionTableDefinition(List<String> disabledPluginIds) {
    final installedPluginIds = controller.installedPlugins.map((plugin) => plugin.id).toSet();
    final missingPluginIds = disabledPluginIds.where((pluginId) => pluginId.trim().isNotEmpty && !installedPluginIds.contains(pluginId)).toList();

    return PluginSettingValueTable.fromJson({
      "Key": _pluginExclusionTableKey,
      "Title": "",
      "Tooltip": "",
      "MaxHeight": 260,
      "Columns": [
        {
          "Key": _pluginExclusionPluginIdKey,
          "Label": "i18n:ui_cloud_sync_plugin_exclusions_plugin",
          "Tooltip": "i18n:ui_cloud_sync_plugin_exclusions_plugin_tips",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeSelect,
          "SelectOptions": [
            ...controller.installedPlugins.map((plugin) {
              final title = plugin.name.isNotEmpty ? plugin.name : plugin.id;
              return {"Label": title, "Value": plugin.id, "Icon": plugin.icon.toJson()};
            }),
            ...missingPluginIds.map((pluginId) {
              return {"Label": "$pluginId (${controller.tr("ui_cloud_sync_plugin_exclusions_uninstalled")})", "Value": pluginId};
            }),
          ],
          "Validators": [
            {"Type": "not_empty"},
            {"Type": "unique"},
          ],
        },
      ],
      "SortColumnKey": _pluginExclusionPluginIdKey,
    });
  }

  // Converts persisted disabled plugin IDs into the generic table row shape.
  String _encodePluginExclusionRows(List<String> pluginIds) {
    final rows = pluginIds.where((pluginId) => pluginId.trim().isNotEmpty).map((pluginId) => {_pluginExclusionPluginIdKey: pluginId}).toList();
    return jsonEncode(rows);
  }

  // Converts edited table rows back to the cloud sync setting's string-list contract.
  List<String> _decodePluginExclusionRows(String value) {
    try {
      final decoded = jsonDecode(value.trim().isEmpty ? "[]" : value);
      if (decoded is! List) {
        return <String>[];
      }

      final pluginIds = <String>[];
      for (final row in decoded) {
        if (row is! Map) {
          continue;
        }

        final pluginId = row[_pluginExclusionPluginIdKey]?.toString().trim() ?? "";
        if (pluginId.isNotEmpty && !pluginIds.contains(pluginId)) {
          pluginIds.add(pluginId);
        }
      }
      return pluginIds;
    } catch (_) {
      return <String>[];
    }
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final loggedIn = controller.accountStatus.value.loggedIn;
      return form(
        width: GENERAL_SETTING_WIDE_FORM_WIDTH,
        title: controller.tr("ui_cloud_sync"),
        children: [
          if (!loggedIn) buildCloudSyncIntroSection(),
          buildAccountSection(context),
          if (loggedIn) ...[
            buildCloudSyncStatusSection(),
            if (controller.accountStatus.value.syncEligible) buildCloudSyncActionSection(context),
            formSection(
              title: controller.tr("ui_cloud_sync_plugin_exclusions"),
              children: [
                formField(
                  label: controller.tr("ui_cloud_sync_plugin_exclusions"),
                  labelWidth: _cloudSyncLabelWidth,
                  child: buildCloudSyncPluginExclusions(),
                  tips: controller.tr("ui_cloud_sync_plugin_exclusions_tips"),
                  fullWidth: true,
                ),
              ],
            ),
          ],
        ],
      );
    });
  }
}

/// Data needed for one cloud sync intro feature row.
class _CloudSyncIntroFeature {
  final IconData icon;
  final String title;
  final String description;

  const _CloudSyncIntroFeature({required this.icon, required this.title, required this.description});
}

// Owns login field controllers so route closing animations never read disposed controllers.
class _AccountLoginDialog extends StatefulWidget {
  final WoxSettingController controller;
  final String Function(String error) normalizeAccountActionError;

  const _AccountLoginDialog({required this.controller, required this.normalizeAccountActionError});

  @override
  State<_AccountLoginDialog> createState() => _AccountLoginDialogState();
}

class _AccountLoginDialogState extends State<_AccountLoginDialog> {
  late final TextEditingController _emailController;
  final _passwordController = TextEditingController();
  String? _validationError;
  bool _isSubmitting = false;

  @override
  void initState() {
    super.initState();
    _emailController = TextEditingController(text: widget.controller.accountStatus.value.email);
  }

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  // Keeps login validation and API failures inside the dialog that submitted them.
  Future<void> _submit() async {
    if (_isSubmitting) {
      return;
    }

    final email = _emailController.text.trim();
    final password = _passwordController.text;
    String? nextError;
    if (email.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_email_required");
    } else if (!email.contains('@')) {
      nextError = widget.controller.tr("ui_cloud_sync_account_error_invalid_email");
    } else if (password.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_password_required");
    }

    if (nextError != null) {
      setState(() {
        _validationError = nextError;
      });
      return;
    }

    setState(() {
      _validationError = null;
      _isSubmitting = true;
    });
    try {
      final result = await widget.controller.accountLogin(email, password);
      if (!mounted) {
        return;
      }
      if (result.isOk || result.needsEmailVerification) {
        Navigator.pop(context, result);
        return;
      }
      setState(() {
        _validationError = widget.normalizeAccountActionError(widget.controller.accountActionError.value);
        _isSubmitting = false;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _validationError = widget.normalizeAccountActionError(e.toString());
        _isSubmitting = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_account_login")),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _AccountDialogField(label: widget.controller.tr("ui_cloud_sync_account_email"), child: WoxTextField(controller: _emailController, width: 360, enabled: !_isSubmitting)),
          const SizedBox(height: 12),
          _AccountDialogField(
            label: widget.controller.tr("ui_cloud_sync_account_password"),
            child: WoxTextField(controller: _passwordController, width: 360, obscureText: true, enabled: !_isSubmitting, onSubmitted: (_) => _submit()),
          ),
          if (_validationError != null) ...[const SizedBox(height: 10), Text(_validationError!, style: const TextStyle(color: Colors.red, fontSize: 12))],
          if (_validationError == null && _isSubmitting) ...[
            const SizedBox(height: 10),
            Text(widget.controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          ],
        ],
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: _isSubmitting ? null : () => Navigator.pop(context)),
        WoxButton.primary(
          text: _isSubmitting ? widget.controller.tr("ui_cloud_sync_loading") : widget.controller.tr("ui_cloud_sync_confirm"),
          onPressed: _isSubmitting ? null : _submit,
        ),
      ],
    );
  }
}

// Owns registration field controllers for the full dialog route lifecycle.
class _AccountRegisterDialog extends StatefulWidget {
  final WoxSettingController controller;
  final String Function(String error) normalizeAccountActionError;

  const _AccountRegisterDialog({required this.controller, required this.normalizeAccountActionError});

  @override
  State<_AccountRegisterDialog> createState() => _AccountRegisterDialogState();
}

class _AccountRegisterDialogState extends State<_AccountRegisterDialog> {
  late final TextEditingController _emailController;
  final _passwordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();
  String? _validationError;
  bool _isSubmitting = false;

  @override
  void initState() {
    super.initState();
    _emailController = TextEditingController(text: widget.controller.accountStatus.value.email);
  }

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    _confirmPasswordController.dispose();
    super.dispose();
  }

  // Keeps registration validation and API failures inside the dialog that submitted them.
  Future<void> _submit() async {
    if (_isSubmitting) {
      return;
    }

    final email = _emailController.text.trim();
    final password = _passwordController.text;
    final confirmPassword = _confirmPasswordController.text;
    String? nextError;
    if (email.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_email_required");
    } else if (!email.contains('@')) {
      nextError = widget.controller.tr("ui_cloud_sync_account_error_invalid_email");
    } else if (password.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_password_required");
    } else if (password.length < 12) {
      nextError = widget.controller.tr("ui_cloud_sync_account_password_min_length");
    } else if (password != confirmPassword) {
      nextError = widget.controller.tr("ui_cloud_sync_account_password_mismatch");
    }

    if (nextError != null) {
      setState(() {
        _validationError = nextError;
      });
      return;
    }

    setState(() {
      _validationError = null;
      _isSubmitting = true;
    });
    try {
      final result = await widget.controller.accountRegister(email, password);
      if (!mounted) {
        return;
      }
      if (result.isOk || result.needsEmailVerification) {
        Navigator.pop(context, result);
        return;
      }
      setState(() {
        _validationError = widget.normalizeAccountActionError(widget.controller.accountActionError.value);
        _isSubmitting = false;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _validationError = widget.normalizeAccountActionError(e.toString());
        _isSubmitting = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_account_register")),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _AccountDialogField(label: widget.controller.tr("ui_cloud_sync_account_email"), child: WoxTextField(controller: _emailController, width: 360, enabled: !_isSubmitting)),
          const SizedBox(height: 12),
          _AccountDialogField(
            label: widget.controller.tr("ui_cloud_sync_account_password"),
            child: WoxTextField(controller: _passwordController, width: 360, obscureText: true, enabled: !_isSubmitting),
          ),
          const SizedBox(height: 12),
          _AccountDialogField(
            label: widget.controller.tr("ui_cloud_sync_account_confirm_password"),
            child: WoxTextField(controller: _confirmPasswordController, width: 360, obscureText: true, enabled: !_isSubmitting, onSubmitted: (_) => _submit()),
          ),
          if (_validationError != null) ...[const SizedBox(height: 10), Text(_validationError!, style: const TextStyle(color: Colors.red, fontSize: 12))],
          if (_validationError == null && _isSubmitting) ...[
            const SizedBox(height: 10),
            Text(widget.controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          ],
        ],
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: _isSubmitting ? null : () => Navigator.pop(context)),
        WoxButton.primary(
          text: _isSubmitting ? widget.controller.tr("ui_cloud_sync_loading") : widget.controller.tr("ui_cloud_sync_confirm"),
          onPressed: _isSubmitting ? null : _submit,
        ),
      ],
    );
  }
}

// Owns the email verification code flow so failures stay in the active dialog.
class _AccountVerifyEmailDialog extends StatefulWidget {
  final WoxSettingController controller;
  final String email;
  final String Function(String error) normalizeAccountActionError;

  const _AccountVerifyEmailDialog({required this.controller, required this.email, required this.normalizeAccountActionError});

  @override
  State<_AccountVerifyEmailDialog> createState() => _AccountVerifyEmailDialogState();
}

class _AccountVerifyEmailDialogState extends State<_AccountVerifyEmailDialog> {
  final _codeController = TextEditingController();
  String? _validationError;
  bool _isSubmitting = false;

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  // Validates the server-issued six digit code before attempting verification.
  Future<void> _submit() async {
    if (_isSubmitting) {
      return;
    }

    final code = _codeController.text.trim();
    String? nextError;
    if (code.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_verify_code_required");
    } else if (!RegExp(r'^\d{6}$').hasMatch(code)) {
      nextError = widget.controller.tr("ui_cloud_sync_account_verify_code_invalid");
    }

    if (nextError != null) {
      setState(() {
        _validationError = nextError;
      });
      return;
    }

    setState(() {
      _validationError = null;
      _isSubmitting = true;
    });
    try {
      final result = await widget.controller.accountVerifyEmail(widget.email, code);
      if (!mounted) {
        return;
      }
      if (result.isOk) {
        Navigator.pop(context, true);
        return;
      }
      setState(() {
        final error = result.message.isNotEmpty ? result.message : widget.controller.accountActionError.value;
        _validationError = widget.normalizeAccountActionError(error.isNotEmpty ? error : widget.controller.tr("ui_cloud_sync_account_verify_code_invalid"));
        _isSubmitting = false;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _validationError = widget.normalizeAccountActionError(e.toString());
        _isSubmitting = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_account_verify")),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _AccountDialogField(
            label: widget.controller.tr("ui_cloud_sync_account_verify_code"),
            child: WoxTextField(controller: _codeController, width: 360, enabled: !_isSubmitting, onSubmitted: (_) => _submit()),
          ),
          if (_validationError != null) ...[const SizedBox(height: 10), Text(_validationError!, style: const TextStyle(color: Colors.red, fontSize: 12))],
          if (_validationError == null && _isSubmitting) ...[
            const SizedBox(height: 10),
            Text(widget.controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          ],
        ],
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: _isSubmitting ? null : () => Navigator.pop(context, false)),
        WoxButton.primary(
          text: _isSubmitting ? widget.controller.tr("ui_cloud_sync_loading") : widget.controller.tr("ui_cloud_sync_confirm"),
          onPressed: _isSubmitting ? null : _submit,
        ),
      ],
    );
  }
}

// Owns password change validation so account errors stay in the active dialog.
class _AccountChangePasswordDialog extends StatefulWidget {
  final WoxSettingController controller;
  final String Function(String error) normalizeAccountActionError;

  const _AccountChangePasswordDialog({required this.controller, required this.normalizeAccountActionError});

  @override
  State<_AccountChangePasswordDialog> createState() => _AccountChangePasswordDialogState();
}

class _AccountChangePasswordDialogState extends State<_AccountChangePasswordDialog> {
  final _currentPasswordController = TextEditingController();
  final _newPasswordController = TextEditingController();
  final _confirmNewPasswordController = TextEditingController();
  String? _validationError;
  bool _isSubmitting = false;

  @override
  void dispose() {
    _currentPasswordController.dispose();
    _newPasswordController.dispose();
    _confirmNewPasswordController.dispose();
    super.dispose();
  }

  // Validates all password fields before sending the authenticated change request.
  Future<void> _submit() async {
    if (_isSubmitting) {
      return;
    }

    final currentPassword = _currentPasswordController.text;
    final newPassword = _newPasswordController.text;
    final confirmNewPassword = _confirmNewPasswordController.text;
    String? nextError;
    if (currentPassword.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_current_password_required");
    } else if (newPassword.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_new_password_required");
    } else if (confirmNewPassword.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_account_confirm_password_required");
    } else if (newPassword != confirmNewPassword) {
      nextError = widget.controller.tr("ui_cloud_sync_account_password_mismatch");
    } else if (newPassword.length < 12) {
      nextError = widget.controller.tr("ui_cloud_sync_account_password_min_length");
    }

    if (nextError != null) {
      setState(() {
        _validationError = nextError;
      });
      return;
    }

    setState(() {
      _validationError = null;
      _isSubmitting = true;
    });
    try {
      final changed = await widget.controller.accountChangePassword(currentPassword, newPassword);
      if (!mounted) {
        return;
      }
      if (changed) {
        Navigator.pop(context, true);
        return;
      }
      setState(() {
        _validationError = widget.normalizeAccountActionError(widget.controller.accountActionError.value);
        _isSubmitting = false;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _validationError = widget.normalizeAccountActionError(e.toString());
        _isSubmitting = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_account_change_password")),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _AccountDialogField(
            label: widget.controller.tr("ui_cloud_sync_account_current_password"),
            child: WoxTextField(controller: _currentPasswordController, width: 360, obscureText: true, enabled: !_isSubmitting),
          ),
          const SizedBox(height: 12),
          _AccountDialogField(
            label: widget.controller.tr("ui_cloud_sync_account_new_password"),
            child: WoxTextField(controller: _newPasswordController, width: 360, obscureText: true, enabled: !_isSubmitting),
          ),
          const SizedBox(height: 12),
          _AccountDialogField(
            label: widget.controller.tr("ui_cloud_sync_account_confirm_new_password"),
            child: WoxTextField(controller: _confirmNewPasswordController, width: 360, obscureText: true, enabled: !_isSubmitting, onSubmitted: (_) => _submit()),
          ),
          if (_validationError != null) ...[const SizedBox(height: 10), Text(_validationError!, style: const TextStyle(color: Colors.red, fontSize: 12))],
          if (_validationError == null && _isSubmitting) ...[
            const SizedBox(height: 10),
            Text(widget.controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          ],
        ],
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: _isSubmitting ? null : () => Navigator.pop(context, false)),
        WoxButton.primary(
          text: _isSubmitting ? widget.controller.tr("ui_cloud_sync_loading") : widget.controller.tr("ui_cloud_sync_confirm"),
          onPressed: _isSubmitting ? null : _submit,
        ),
      ],
    );
  }
}

class _AccountDialogField extends StatelessWidget {
  final String label;
  final Widget child;

  const _AccountDialogField({required this.label, required this.child});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [Text(label, style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600)), const SizedBox(height: 6), child],
    );
  }
}
