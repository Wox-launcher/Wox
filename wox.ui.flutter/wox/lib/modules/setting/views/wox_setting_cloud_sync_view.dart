import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_cloud_sync.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';

enum _CloudSyncAccountAction { changePassword, logout }

enum _CloudSyncSubscriptionAction { refreshStatus, subscribePro, manageSubscription }

typedef _CloudSyncErrorNormalizer = String Function(String error);

class WoxSettingCloudSyncView extends WoxSettingBaseView {
  const WoxSettingCloudSyncView({super.key});

  static const String _pluginExclusionTableKey = "CloudSyncDisabledPluginsTable";
  static const String _pluginExclusionPluginIdKey = "PluginId";
  static const double _cloudSyncLabelWidth = 520.0;
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
    if (error.contains('device_limit_exceeded')) {
      return controller.tr("ui_cloud_sync_device_limit_exceeded");
    }
    if (error.contains('device_revoked')) {
      return controller.tr("ui_cloud_sync_device_revoked");
    }
    if (error.contains('failed to decrypt payload') || error.contains('message authentication failed')) {
      return controller.tr("ui_cloud_sync_recovery_code_invalid");
    }
    return serverProvidedErrorMessage(error);
  }

  // Removes local wrapper text while keeping the remote server's localized error message unchanged.
  String serverProvidedErrorMessage(String error) {
    final message = error.replaceFirst('Exception: ', '').trim();
    final codePrefix = RegExp(r'^[a-z][a-z0-9_]*:\s*(.+)$').firstMatch(message);
    return codePrefix?.group(1)?.trim() ?? message;
  }

  int lastCloudSyncTimestamp(WoxCloudSyncState? state) {
    if (state == null) {
      return 0;
    }
    return state.lastPullTs > state.lastPushTs ? state.lastPullTs : state.lastPushTs;
  }

  // Uses the device list so localized server messages do not hide revoked-device state.
  bool isCurrentCloudSyncDeviceRevoked(WoxCloudSyncDeviceList deviceList) {
    return deviceList.devices.any((device) => device.revoked && (device.current || (deviceList.currentDeviceId.isNotEmpty && device.deviceId == deviceList.currentDeviceId)));
  }

  String formatCloudSyncProgress(WoxCloudSyncProgress? progress, {required bool isBusy}) {
    if (progress == null || !progress.active || progress.operation.isEmpty) {
      return isBusy ? controller.tr("ui_cloud_sync_progress_starting") : '';
    }

    final countText = progress.total > 0 ? "${progress.current}/${progress.total}" : progress.current.toString();
    switch (progress.operation) {
      case 'snapshot':
        return controller.tr("ui_cloud_sync_progress_snapshot");
      case 'push':
        return controller.tr("ui_cloud_sync_progress_uploading").replaceAll("{target}", cloudSyncProgressTarget(progress)).replaceAll("{count}", countText);
      case 'pull':
        return controller.tr("ui_cloud_sync_progress_downloading").replaceAll("{target}", cloudSyncProgressTarget(progress)).replaceAll("{count}", countText);
      case 'restore':
        return controller.tr("ui_cloud_sync_progress_restoring").replaceAll("{count}", countText);
      default:
        return controller.tr("ui_cloud_sync_progress_starting");
    }
  }

  String cloudSyncProgressTarget(WoxCloudSyncProgress progress) {
    if (progress.entityType == 'wox_setting') {
      return controller.tr("ui_cloud_sync_progress_wox_setting");
    }
    if (progress.entityType == 'plugin_setting') {
      final pluginName = cloudSyncProgressPluginName(progress.pluginId);
      return controller.tr("ui_cloud_sync_progress_plugin").replaceAll("{plugin}", pluginName);
    }
    return controller.tr("ui_cloud_sync_progress_data");
  }

  String cloudSyncProgressPluginName(String pluginId) {
    for (final plugin in controller.installedPlugins) {
      if (plugin.id == pluginId) {
        if (plugin.name.isNotEmpty) {
          return plugin.name;
        }
        if (plugin.nameEn.isNotEmpty) {
          return plugin.nameEn;
        }
      }
    }
    return pluginId.isNotEmpty ? pluginId : controller.tr("ui_cloud_sync_progress_data");
  }

  String normalizeAccountActionError(String error) {
    return serverProvidedErrorMessage(error);
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
    return await showDialog<String>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _CloudSyncTokenDialog(controller: controller, title: title, hint: hint),
    );
  }

  Future<Map<String, String>?> showResetPasswordDialog(BuildContext context) async {
    return await showDialog<Map<String, String>>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _CloudSyncResetPasswordDialog(controller: controller),
    );
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
    return await showDialog<Map<String, String>>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _CloudSyncInitKeyDialog(controller: controller),
    );
  }

  Future<String?> showCloudSyncFetchKeyDialog(BuildContext context) async {
    return await showDialog<String>(context: context, barrierColor: getThemePopupBarrierColor(), builder: (context) => _CloudSyncFetchKeyDialog(controller: controller));
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

  // Resets the lost encryption password path and immediately starts a new local-to-cloud sync.
  Future<bool?> showCloudSyncForgotRecoveryCodeDialog(BuildContext context) async {
    return await showDialog<bool>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _CloudSyncForgotRecoveryCodeDialog(controller: controller, normalizeCloudSyncError: normalizeCloudSyncError),
    );
  }

  Future<void> showCloudSyncBootstrapDialog(BuildContext context, WoxCloudSyncBootstrapStatus status) async {
    await showDialog<void>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder:
          (context) => _CloudSyncBootstrapDialog(
            controller: controller,
            status: status,
            normalizeCloudSyncError: normalizeCloudSyncError,
            showForgotRecoveryCodeDialog: showCloudSyncForgotRecoveryCodeDialog,
          ),
    );
  }

  Widget buildCloudSyncStatusSection(BuildContext context) {
    return Obx(() {
      final status = controller.cloudSyncStatus.value;
      final account = controller.accountStatus.value;
      final state = status.state;
      final isLoading = controller.isCloudSyncStatusLoading.value;
      final isBusy = controller.isCloudSyncActionLoading.value;
      final statusError = controller.cloudSyncStatusError.value;
      final actionError = controller.cloudSyncActionError.value;
      final stateError = state?.lastError ?? '';
      final isCurrentDeviceRevoked = isCurrentCloudSyncDeviceRevoked(controller.cloudSyncDeviceList.value);
      final lastSyncTime = formatCloudSyncTime(lastCloudSyncTimestamp(state));
      final nextAvailableSyncTime = state != null && state.backoffUntil > DateTime.now().millisecondsSinceEpoch ? formatCloudSyncTime(state.backoffUntil) : '';
      final progressText = formatCloudSyncProgress(status.progress, isBusy: isBusy);
      final isSynced = account.syncEnabled && status.keyStatus.available && state != null && state.bootstrapped;
      final isBootstrapInProgress = account.syncEnabled && status.keyStatus.available && state != null && !state.bootstrapped && stateError.isEmpty;
      final String statusText;
      final Color? statusColor;
      final String? statusDetailText;
      final Color? statusDetailColor;
      if (isLoading) {
        statusText = controller.tr("ui_cloud_sync_loading");
        statusColor = getThemeSubTextColor();
        statusDetailText = null;
        statusDetailColor = null;
      } else if (statusError.isNotEmpty) {
        statusText = controller.tr("ui_cloud_sync_sync_error");
        statusColor = Colors.red;
        statusDetailText = normalizeCloudSyncError(statusError);
        statusDetailColor = Colors.red;
      } else if (actionError.isNotEmpty) {
        statusText = controller.tr("ui_cloud_sync_sync_error");
        statusColor = Colors.red;
        statusDetailText = normalizeCloudSyncError(actionError);
        statusDetailColor = Colors.red;
      } else if (stateError.isNotEmpty) {
        statusText = controller.tr("ui_cloud_sync_sync_error");
        statusColor = Colors.red;
        statusDetailText = normalizeCloudSyncError(stateError);
        statusDetailColor = Colors.red;
      } else if (progressText.isNotEmpty) {
        statusText = controller.tr("ui_cloud_sync_syncing");
        statusColor = getThemeSubTextColor();
        statusDetailText = progressText;
        statusDetailColor = getThemeSubTextColor();
      } else if (isBootstrapInProgress) {
        statusText = controller.tr("ui_cloud_sync_syncing");
        statusColor = getThemeSubTextColor();
        statusDetailText = null;
        statusDetailColor = null;
      } else if (account.loggedIn && account.syncEligible && (!account.syncEnabled || !status.keyStatus.available || state == null || !state.bootstrapped)) {
        statusText = controller.tr("ui_cloud_sync_unsynced");
        statusColor = getThemeSubTextColor();
        statusDetailText = null;
        statusDetailColor = null;
      } else if (!account.syncEnabled || !status.enabled) {
        statusText = controller.tr("ui_cloud_sync_disabled");
        statusColor = getThemeSubTextColor();
        statusDetailText = null;
        statusDetailColor = null;
      } else if (!status.keyStatus.available) {
        statusText = controller.tr("ui_cloud_sync_sync_error");
        statusColor = Colors.red;
        statusDetailText = controller.tr("ui_cloud_sync_key_missing");
        statusDetailColor = Colors.red;
      } else if (state != null && !state.bootstrapped) {
        statusText = controller.tr("ui_cloud_sync_sync_error");
        statusColor = Colors.red;
        statusDetailText = controller.tr("ui_cloud_sync_not_initialized");
        statusDetailColor = Colors.red;
      } else {
        statusText = controller.tr("ui_cloud_sync_synced");
        statusColor = null;
        statusDetailText = "${controller.tr("ui_cloud_sync_last_sync_time")}: $lastSyncTime";
        statusDetailColor = getThemeSubTextColor();
      }
      final shouldBootstrap = account.loggedIn && account.syncEligible && !isSynced && !isBootstrapInProgress;
      final syncButtonEnabled = account.loggedIn && account.syncEligible && !isLoading && !isBusy && !isBootstrapInProgress && !isCurrentDeviceRevoked;
      final joinButtonEnabled = account.loggedIn && account.syncEligible && isCurrentDeviceRevoked && !isLoading && !isBusy;
      final statusDetailParts = [
        if (statusDetailText != null && statusDetailText.isNotEmpty) statusDetailText,
        if (nextAvailableSyncTime.isNotEmpty) "${controller.tr("ui_cloud_sync_backoff_until")}: $nextAvailableSyncTime",
      ];
      final statusLineText = statusDetailParts.isEmpty ? statusText : "$statusText, ${statusDetailParts.join(" ")}";
      final statusLineColor = statusDetailColor ?? statusColor;

      return formSection(
        title: controller.tr("ui_cloud_sync_sync"),
        children: [
          formField(
            settingKey: "CloudSyncStatus",
            label: controller.tr("ui_cloud_sync_sync_status"),
            labelWidth: _cloudSyncLabelWidth,
            tipsWidget: buildCloudSyncInfoValue(statusLineText, color: statusLineColor),
            child: WoxButton.primary(
              text: isCurrentDeviceRevoked ? controller.tr("ui_cloud_sync_join") : controller.tr("ui_cloud_sync_sync"),
              onPressed:
                  isCurrentDeviceRevoked
                      ? (joinButtonEnabled ? () => controller.cloudSyncJoinDevice() : null)
                      : (syncButtonEnabled ? () async => handleCloudSyncButtonPressed(context, shouldBootstrap: shouldBootstrap) : null),
            ),
          ),
        ],
      );
    });
  }

  // Routes the single sync button to bootstrap when setup is incomplete, otherwise performs an immediate sync.
  Future<void> handleCloudSyncButtonPressed(BuildContext context, {required bool shouldBootstrap}) async {
    if (shouldBootstrap) {
      final bootstrapStatus = await controller.cloudSyncBootstrapStatus();
      if (!context.mounted || bootstrapStatus == null) {
        return;
      }
      await showCloudSyncBootstrapDialog(context, bootstrapStatus);
      return;
    }
    await controller.cloudSyncSyncNow();
  }

  Widget buildAccountSection(BuildContext context) {
    return Obx(() {
      final account = controller.accountStatus.value;
      final isBusy = controller.isCloudSyncActionLoading.value;
      final isBillingWaiting = controller.isAccountBillingWaiting.value;
      final billingWaitingMessageKey = controller.accountBillingWaitingMessageKey.value;
      final subscriptionError = controller.accountSubscriptionError.value;
      final isPro = account.isPro;
      final billingWaitingText = billingWaitingMessageKey.isNotEmpty ? controller.tr(billingWaitingMessageKey) : controller.tr("ui_cloud_sync_subscription_waiting_payment");
      final subscriptionStatusText =
          isBillingWaiting
              ? billingWaitingText
              : subscriptionError.isNotEmpty
              ? normalizeAccountActionError(subscriptionError)
              : isPro
              ? controller.tr("ui_cloud_sync_plan_pro_status")
              : controller.tr("ui_cloud_sync_plan_free_status");
      final subscriptionStatusColor =
          isBillingWaiting
              ? getThemeSubTextColor()
              : subscriptionError.isNotEmpty
              ? Colors.red
              : null;
      return formSection(
        title: controller.tr("ui_cloud_sync_account"),
        children: [
          if (account.loggedIn) ...[
            formField(
              settingKey: "CloudSyncAccount",
              label: controller.tr("ui_cloud_sync_account_email"),
              labelWidth: _cloudSyncLabelWidth,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [Flexible(child: buildCloudSyncInfoValue(account.email)), const SizedBox(width: 6), buildAccountActionMenu(context: context, isBusy: isBusy)],
              ),
            ),
            formField(
              settingKey: "CloudSyncSubscriptionStatus",
              label: controller.tr("ui_cloud_sync_plan_status"),
              labelTrailing: _buildCloudSyncPlanStatusTooltip(),
              labelWidth: _cloudSyncLabelWidth,
              tips: controller.tr("ui_cloud_sync_plan_status_tips"),
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
                    const SizedBox(width: 6),
                    buildSubscriptionActionMenu(isPro: isPro, isBusy: isBusy, isBillingWaiting: isBillingWaiting),
                  ],
                ),
              ),
            ),
            formField(
              settingKey: "CloudSyncBillingHelp",
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
              settingKey: "CloudSyncAccount",
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

  Widget _buildCloudSyncPlanStatusTooltip() {
    final plan = controller.cloudSyncBillingPlan.value;
    final freePrice = _cloudSyncPlanPriceText(plan.free.price);
    final proPrice = _cloudSyncPlanPriceText(plan.pro.price);

    return Tooltip(
      richMessage: WidgetSpan(child: SizedBox(width: 560, child: _buildCloudSyncPlanTable(compact: false, freePrice: freePrice, proPrice: proPrice))),
      padding: const EdgeInsets.all(10),
      margin: const EdgeInsets.all(12),
      waitDuration: const Duration(milliseconds: 250),
      showDuration: const Duration(seconds: 20),
      preferBelow: false,
      decoration: BoxDecoration(color: getThemePopupSurfaceColor(), borderRadius: BorderRadius.circular(8), border: Border.all(color: getThemePopupOutlineColor())),
      child: Icon(Icons.info_outline, size: 14, color: getThemeSubTextColor().withValues(alpha: 0.82)),
    );
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
  Widget buildSubscriptionActionMenu({required bool isPro, required bool isBusy, required bool isBillingWaiting}) {
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
          case _CloudSyncSubscriptionAction.subscribePro:
            await controller.accountOpenCheckout();
            break;
          case _CloudSyncSubscriptionAction.manageSubscription:
            await controller.accountOpenBillingPortal();
            break;
        }
      },
      itemBuilder:
          (context) => [
            PopupMenuItem(
              value: _CloudSyncSubscriptionAction.refreshStatus,
              enabled: !isBillingWaiting,
              child: Text(controller.tr("ui_cloud_sync_refresh_status"), style: textStyle),
            ),
            PopupMenuItem(
              value: isPro ? _CloudSyncSubscriptionAction.manageSubscription : _CloudSyncSubscriptionAction.subscribePro,
              enabled: !isBillingWaiting,
              child: Text(isPro ? controller.tr("ui_cloud_sync_manage_subscription") : _cloudSyncSubscribeProLabel(), style: textStyle),
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

  String _cloudSyncSubscribeProLabel() {
    final priceText = _cloudSyncPlanPriceText(controller.cloudSyncBillingPlan.value.pro.price);
    if (priceText.isEmpty || priceText == controller.tr("ui_cloud_sync_plan_price_loading") || priceText == controller.tr("ui_cloud_sync_plan_price_unavailable")) {
      return controller.tr("ui_cloud_sync_subscribe");
    }
    return controller.tr("ui_cloud_sync_subscribe_with_price").replaceAll("{price}", priceText);
  }

  Widget buildCloudSyncIntroSection() {
    return formSection(
      title: controller.tr("ui_cloud_sync_intro_title"),
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 24),
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

              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildCloudSyncIntroHero(stacked: useStackedLayout),
                  const SizedBox(height: 18),
                  _buildCloudSyncIntroFeatureSummary(featureItems, stacked: useStackedLayout),
                  const SizedBox(height: 20),
                  buildCloudSyncPlanComparison(),
                ],
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _buildCloudSyncIntroHero({required bool stacked}) {
    final content = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(controller.tr("ui_cloud_sync_intro_headline"), style: TextStyle(color: getThemeTextColor(), fontSize: 20, fontWeight: FontWeight.w700, height: 1.2)),
        const SizedBox(height: 8),
        Text(controller.tr("ui_cloud_sync_intro_description"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.4)),
      ],
    );

    if (stacked) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [_buildCloudSyncIntroIcon(Icons.cloud_queue_outlined, size: 52, iconSize: 27), const SizedBox(height: 14), content],
      );
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [_buildCloudSyncIntroIcon(Icons.cloud_queue_outlined, size: 56, iconSize: 28), const SizedBox(width: 16), Expanded(child: content)],
    );
  }

  /// Builds the shared icon treatment used by the cloud sync intro panel.
  Widget _buildCloudSyncIntroIcon(IconData icon, {double size = 42, double iconSize = 22}) {
    final iconColor = getThemeTextColor().withValues(alpha: isThemeDark() ? 0.76 : 0.68);
    final outlineColor = getThemeSettingDividerColor().withValues(alpha: isThemeDark() ? 0.42 : 0.58);
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(color: Colors.transparent, borderRadius: BorderRadius.circular(8), border: Border.all(color: outlineColor)),
      child: Icon(icon, size: iconSize, color: iconColor),
    );
  }

  Widget _buildCloudSyncIntroFeatureSummary(List<_CloudSyncIntroFeature> features, {required bool stacked}) {
    if (stacked) {
      return Column(
        children: [
          for (var i = 0; i < features.length; i++) ...[_buildCloudSyncIntroFeatureTile(features[i]), if (i < features.length - 1) const SizedBox(height: 10)],
        ],
      );
    }

    return Row(
      children: [
        for (var i = 0; i < features.length; i++) ...[Expanded(child: _buildCloudSyncIntroFeatureTile(features[i])), if (i < features.length - 1) const SizedBox(width: 10)],
      ],
    );
  }

  /// Builds one compact capability tile in the cloud sync intro panel.
  Widget _buildCloudSyncIntroFeatureTile(_CloudSyncIntroFeature feature) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: getThemeSettingDividerColor().withValues(alpha: isThemeDark() ? 0.36 : 0.5)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildCloudSyncIntroIcon(feature.icon, size: 34, iconSize: 17),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(feature.title, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w700, height: 1.2)),
                const SizedBox(height: 5),
                Text(feature.description, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.32)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget buildCloudSyncPlanComparison() {
    return Obx(() {
      final plan = controller.cloudSyncBillingPlan.value;
      final proPrice = _cloudSyncPlanPriceText(plan.pro.price);
      return LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 620;
          return _buildCloudSyncPlanTable(compact: compact, freePrice: plan.free.price.formatted, proPrice: proPrice);
        },
      );
    });
  }

  String _cloudSyncPlanPriceText(WoxBillingPlanPrice price) {
    if (price.formatted.isNotEmpty) {
      return price.formatted;
    }
    if (price.unitAmount != null && price.currency.isNotEmpty) {
      final amount = price.unitAmount! / 100;
      final normalizedAmount = amount == amount.roundToDouble() ? amount.toStringAsFixed(0) : amount.toStringAsFixed(2);
      final interval = price.interval.isNotEmpty ? "/${price.interval}" : "";
      return "${price.currency.toUpperCase()} $normalizedAmount$interval";
    }
    if (!controller.cloudSyncBillingPlanLoaded.value) {
      return controller.tr("ui_cloud_sync_plan_price_loading");
    }
    return controller.tr("ui_cloud_sync_plan_price_unavailable");
  }

  Widget _buildCloudSyncPlanTable({required bool compact, required String freePrice, required String proPrice}) {
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: getThemeSettingDividerColor().withValues(alpha: isThemeDark() ? 0.54 : 0.64)),
      ),
      child: Column(
        children: [
          _buildCloudSyncPlanHeader(compact: compact),
          _buildCloudSyncPlanTableRow(compact: compact, label: controller.tr("ui_cloud_sync_plan_row_price"), freeValue: freePrice, proValue: proPrice),
          _buildCloudSyncPlanTableRow(
            compact: compact,
            label: controller.tr("ui_cloud_sync_plan_row_devices"),
            freeValue: controller.tr("ui_cloud_sync_plan_feature_two_devices"),
            proValue: controller.tr("ui_cloud_sync_plan_feature_unlimited_devices"),
          ),
          _buildCloudSyncPlanTableRow(
            compact: compact,
            label: controller.tr("ui_cloud_sync_plan_row_sync_mode"),
            freeValue: controller.tr("ui_cloud_sync_plan_feature_manual_sync"),
            proValue: controller.tr("ui_cloud_sync_plan_feature_auto_sync"),
          ),
          _buildCloudSyncPlanTableRow(
            compact: compact,
            label: controller.tr("ui_cloud_sync_plan_row_frequency"),
            freeValue: controller.tr("ui_cloud_sync_plan_feature_strict_sync_limit"),
            proValue: controller.tr("ui_cloud_sync_plan_feature_relaxed_sync_limit"),
          ),
          _buildCloudSyncPlanTableRow(
            compact: compact,
            label: controller.tr("ui_cloud_sync_plan_row_scope"),
            freeValue: controller.tr("ui_cloud_sync_plan_scope_free"),
            proValue: controller.tr("ui_cloud_sync_plan_feature_everything_free"),
            isLast: true,
          ),
        ],
      ),
    );
  }

  Widget _buildCloudSyncPlanHeader({required bool compact}) {
    if (compact) {
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(color: Colors.transparent),
        child: Row(
          children: [
            Expanded(child: _buildCloudSyncPlanHeaderCell(title: controller.tr("ui_cloud_sync_plan_free"))),
            const SizedBox(width: 10),
            Expanded(child: _buildCloudSyncPlanHeaderCell(title: controller.tr("ui_cloud_sync_plan_pro"), highlighted: true)),
          ],
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(color: Colors.transparent),
      child: Row(
        children: [
          const SizedBox(width: 132),
          Expanded(child: _buildCloudSyncPlanHeaderCell(title: controller.tr("ui_cloud_sync_plan_free"))),
          const SizedBox(width: 10),
          Expanded(child: _buildCloudSyncPlanHeaderCell(title: controller.tr("ui_cloud_sync_plan_pro"), highlighted: true)),
        ],
      ),
    );
  }

  Widget _buildCloudSyncPlanHeaderCell({required String title, bool highlighted = false}) {
    const tagBackgroundColor = Color(0xFF0B6BD3);
    const tagBorderColor = Color(0xFF0757AE);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(title, style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w800, height: 1.2)),
        if (highlighted) ...[
          const SizedBox(width: 8),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
            decoration: BoxDecoration(color: tagBackgroundColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: tagBorderColor)),
            child: Text(controller.tr("ui_cloud_sync_plan_recommended"), style: const TextStyle(color: Colors.white, fontSize: 10, fontWeight: FontWeight.w700, height: 1.1)),
          ),
        ],
      ],
    );
  }

  Widget _buildCloudSyncPlanTableRow({required bool compact, required String label, required String freeValue, required String proValue, bool isLast = false}) {
    final borderColor = getThemeSettingDividerColor().withValues(alpha: isThemeDark() ? 0.36 : 0.5);
    final content =
        compact
            ? _buildCloudSyncPlanCompactRow(label: label, freeValue: freeValue, proValue: proValue)
            : _buildCloudSyncPlanWideRow(label: label, freeValue: freeValue, proValue: proValue);
    return Container(
      padding: EdgeInsets.symmetric(horizontal: compact ? 12 : 14, vertical: 12),
      decoration: BoxDecoration(border: Border(top: BorderSide(color: borderColor), bottom: isLast ? BorderSide.none : BorderSide(color: Colors.transparent))),
      child: content,
    );
  }

  Widget _buildCloudSyncPlanWideRow({required String label, required String freeValue, required String proValue}) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(width: 132, child: Text(label, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, fontWeight: FontWeight.w600, height: 1.35))),
        Expanded(child: Text(freeValue, style: TextStyle(color: getThemeTextColor(), fontSize: 13, height: 1.35))),
        const SizedBox(width: 10),
        Expanded(child: Text(proValue, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w600, height: 1.35))),
      ],
    );
  }

  Widget _buildCloudSyncPlanCompactRow({required String label, required String freeValue, required String proValue}) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, fontWeight: FontWeight.w600, height: 1.3)),
        const SizedBox(height: 8),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: Text(freeValue, style: TextStyle(color: getThemeTextColor(), fontSize: 13, height: 1.35))),
            const SizedBox(width: 10),
            Expanded(child: Text(proValue, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w600, height: 1.35))),
          ],
        ),
      ],
    );
  }

  Widget buildCloudSyncDeviceSection() {
    return Obx(() {
      final deviceList = controller.cloudSyncDeviceList.value;
      final account = controller.accountStatus.value;
      final devices = account.isPro ? deviceList.devices.where((device) => !device.revoked).toList() : deviceList.devices;
      final deviceLimitText = (deviceList.deviceLimit ?? account.syncLimits.deviceLimit ?? 2).toString();
      final deviceTips =
          account.isPro
              ? controller.tr("ui_cloud_sync_devices_pro_tips")
              : controller.tr("ui_cloud_sync_devices_free_tips").replaceAll("{count}", deviceList.deviceCount.toString()).replaceAll("{limit}", deviceLimitText);
      final isBusy = controller.isCloudSyncActionLoading.value;
      return formSection(
        title: controller.tr("ui_cloud_sync_devices"),
        children: [
          formField(
            settingKey: "CloudSyncDevices",
            label: controller.tr("ui_cloud_sync_devices"),
            labelWidth: _cloudSyncLabelWidth,
            tips: deviceTips,
            child: SizedBox(
              width: _cloudSyncValueWidth,
              child: Align(
                alignment: Alignment.centerRight,
                child: WoxButton.secondary(
                  text: controller.tr("ui_cloud_sync_refresh"),
                  icon: const Icon(Icons.refresh_outlined, size: 16),
                  onPressed: isBusy ? null : () => controller.refreshCloudSyncDevices(),
                ),
              ),
            ),
          ),
          if (devices.isEmpty)
            formField(
              settingKey: "CloudSyncDeviceEmpty",
              label: "",
              labelWidth: _cloudSyncLabelWidth,
              child: SizedBox(
                width: _cloudSyncValueWidth,
                child: Align(alignment: Alignment.centerRight, child: buildCloudSyncInfoValue(controller.tr("ui_cloud_sync_devices_empty"), color: getThemeSubTextColor())),
              ),
            )
          else
            for (final device in devices)
              settingTarget(settingKey: "CloudSyncDevice-${device.deviceId}", child: _buildCloudSyncDeviceRow(device, isBusy: isBusy, showRevokeState: !account.isPro)),
        ],
      );
    });
  }

  Widget _buildCloudSyncDeviceRow(WoxCloudSyncDevice device, {required bool isBusy, required bool showRevokeState}) {
    final deviceTitle = device.deviceName.isNotEmpty ? device.deviceName : device.deviceId;
    return Padding(
      padding: const EdgeInsets.only(bottom: 18),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  device.current ? "$deviceTitle ${controller.tr("ui_cloud_sync_devices_current")}" : deviceTitle,
                  style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w600, height: 1.25),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 4),
                Text(
                  _formatCloudSyncDevicePlatform(device.platform),
                  style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.25),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          const SizedBox(width: 18),
          Text(formatCloudSyncTime(device.lastSeenAt), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.25), maxLines: 1, overflow: TextOverflow.ellipsis),
          if (showRevokeState) ...[
            const SizedBox(width: 10),
            if (device.revoked)
              _buildCloudSyncDeviceRevokedStatus()
            else
              WoxButton.secondary(
                text: controller.tr("ui_cloud_sync_devices_revoke"),
                onPressed: isBusy || device.current ? null : () => controller.cloudSyncRevokeDevice(device.deviceId),
              ),
          ],
        ],
      ),
    );
  }

  // Shows revoked devices as a read-only state instead of a disabled action button.
  Widget _buildCloudSyncDeviceRevokedStatus() {
    final color = getThemeSubTextColor();
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(controller.tr("ui_cloud_sync_devices_revoked"), style: TextStyle(color: color, fontSize: 12, fontWeight: FontWeight.w600, height: 1.25)),
        const SizedBox(width: 5),
        WoxTooltip(
          message: controller.tr("ui_cloud_sync_devices_revoked_tips"),
          preferSide: WoxTooltipSide.left,
          child: Icon(Icons.info_outline, size: 14, color: color.withValues(alpha: 0.82)),
        ),
      ],
    );
  }

  String _formatCloudSyncDevicePlatform(String platform) {
    switch (platform.trim().toLowerCase()) {
      case "windows":
        return "Windows";
      case "darwin":
        return "macOS";
      case "linux":
        return "Linux";
      default:
        return controller.tr("ui_cloud_sync_devices_unknown_platform");
    }
  }

  Widget buildCloudSyncPluginExclusions() {
    return Obx(() {
      final _ = controller.installedPluginListRevision.value;
      final disabledPluginIds = controller.woxSetting.value.cloudSyncDisabledPlugins;
      if (controller.installedPlugins.isEmpty && disabledPluginIds.isEmpty) {
        return Text(controller.tr("ui_cloud_sync_plugin_exclusions_empty"));
      }

      return WoxSettingPluginTable(
        inlineTitleActions: true,
        tableWidth: GENERAL_SETTING_TABLE_WIDTH,
        showCloneAction: false,
        value: _encodePluginExclusionRows(disabledPluginIds),
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
      "Title": "i18n:ui_cloud_sync_plugin_exclusions",
      "Tooltip": "i18n:ui_cloud_sync_plugin_exclusions_tips",
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
        description: controller.tr("ui_cloud_sync_description"),
        children: [
          if (!loggedIn) buildCloudSyncIntroSection(),
          buildAccountSection(context),
          if (loggedIn) ...[
            buildCloudSyncStatusSection(context),
            buildCloudSyncDeviceSection(),
            settingTarget(settingKey: "CloudSyncDisabledPlugins", child: Padding(padding: const EdgeInsets.only(bottom: 24), child: buildCloudSyncPluginExclusions())),
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

// Owns a single token controller for the dialog route lifecycle, including close animations.
class _CloudSyncTokenDialog extends StatefulWidget {
  final WoxSettingController controller;
  final String title;
  final String hint;

  const _CloudSyncTokenDialog({required this.controller, required this.title, required this.hint});

  @override
  State<_CloudSyncTokenDialog> createState() => _CloudSyncTokenDialogState();
}

class _CloudSyncTokenDialogState extends State<_CloudSyncTokenDialog> {
  final _tokenController = TextEditingController();

  @override
  void dispose() {
    _tokenController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.title),
      content: WoxTextField(controller: _tokenController, hintText: widget.hint, width: 360),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: () => Navigator.pop(context)),
        WoxButton.primary(text: widget.controller.tr("ui_cloud_sync_confirm"), onPressed: () => Navigator.pop(context, _tokenController.text.trim())),
      ],
    );
  }
}

// Owns reset-password field controllers until the dialog is fully disposed.
class _CloudSyncResetPasswordDialog extends StatefulWidget {
  final WoxSettingController controller;

  const _CloudSyncResetPasswordDialog({required this.controller});

  @override
  State<_CloudSyncResetPasswordDialog> createState() => _CloudSyncResetPasswordDialogState();
}

class _CloudSyncResetPasswordDialogState extends State<_CloudSyncResetPasswordDialog> {
  final _tokenController = TextEditingController();
  final _passwordController = TextEditingController();

  @override
  void dispose() {
    _tokenController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_account_reset_confirm")),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          WoxTextField(controller: _tokenController, hintText: widget.controller.tr("ui_cloud_sync_account_reset_token"), width: 360),
          const SizedBox(height: 10),
          WoxTextField(controller: _passwordController, hintText: widget.controller.tr("ui_cloud_sync_account_new_password"), width: 360, obscureText: true),
        ],
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: () => Navigator.pop(context)),
        WoxButton.primary(
          text: widget.controller.tr("ui_cloud_sync_confirm"),
          onPressed: () => Navigator.pop(context, {"token": _tokenController.text.trim(), "password": _passwordController.text}),
        ),
      ],
    );
  }
}

// Owns the first-device key fields until the route has finished closing.
class _CloudSyncInitKeyDialog extends StatefulWidget {
  final WoxSettingController controller;

  const _CloudSyncInitKeyDialog({required this.controller});

  @override
  State<_CloudSyncInitKeyDialog> createState() => _CloudSyncInitKeyDialogState();
}

class _CloudSyncInitKeyDialogState extends State<_CloudSyncInitKeyDialog> {
  final _recoveryController = TextEditingController();
  final _deviceController = TextEditingController();

  @override
  void dispose() {
    _recoveryController.dispose();
    _deviceController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_key_init_title")),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(widget.controller.tr("ui_cloud_sync_recovery_code_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          const SizedBox(height: 6),
          WoxTextField(controller: _recoveryController, hintText: widget.controller.tr("ui_cloud_sync_recovery_code_hint"), width: 360),
          const SizedBox(height: 12),
          Text(widget.controller.tr("ui_cloud_sync_device_name"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          const SizedBox(height: 6),
          WoxTextField(controller: _deviceController, hintText: widget.controller.tr("ui_cloud_sync_device_name_hint"), width: 360),
        ],
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: () => Navigator.pop(context)),
        WoxButton.primary(
          text: widget.controller.tr("ui_cloud_sync_confirm"),
          onPressed: () => Navigator.pop(context, {"recoveryCode": _recoveryController.text.trim(), "deviceName": _deviceController.text.trim()}),
        ),
      ],
    );
  }
}

// Owns the recovery-code fetch field until the dialog route is gone.
class _CloudSyncFetchKeyDialog extends StatefulWidget {
  final WoxSettingController controller;

  const _CloudSyncFetchKeyDialog({required this.controller});

  @override
  State<_CloudSyncFetchKeyDialog> createState() => _CloudSyncFetchKeyDialogState();
}

class _CloudSyncFetchKeyDialogState extends State<_CloudSyncFetchKeyDialog> {
  final _recoveryController = TextEditingController();

  @override
  void dispose() {
    _recoveryController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_key_fetch_title")),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(widget.controller.tr("ui_cloud_sync_recovery_code_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          const SizedBox(height: 6),
          WoxTextField(controller: _recoveryController, hintText: widget.controller.tr("ui_cloud_sync_recovery_code_hint"), width: 360),
        ],
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: () => Navigator.pop(context)),
        WoxButton.primary(text: widget.controller.tr("ui_cloud_sync_confirm"), onPressed: () => Navigator.pop(context, _recoveryController.text.trim())),
      ],
    );
  }
}

// Owns the reset bootstrap controllers and async state while the forgot-code dialog is mounted.
class _CloudSyncForgotRecoveryCodeDialog extends StatefulWidget {
  final WoxSettingController controller;
  final _CloudSyncErrorNormalizer normalizeCloudSyncError;

  const _CloudSyncForgotRecoveryCodeDialog({required this.controller, required this.normalizeCloudSyncError});

  @override
  State<_CloudSyncForgotRecoveryCodeDialog> createState() => _CloudSyncForgotRecoveryCodeDialogState();
}

class _CloudSyncForgotRecoveryCodeDialogState extends State<_CloudSyncForgotRecoveryCodeDialog> {
  static const double _dialogContentWidth = 360.0;

  final _recoveryController = TextEditingController();
  final _confirmRecoveryController = TextEditingController();
  String? _validationError;
  bool _isSubmitting = false;

  @override
  void dispose() {
    _recoveryController.dispose();
    _confirmRecoveryController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_isSubmitting) {
      return;
    }

    final recoveryCode = _recoveryController.text.trim();
    final confirmRecoveryCode = _confirmRecoveryController.text.trim();
    String? nextError;
    if (recoveryCode.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_recovery_code_required");
    } else if (confirmRecoveryCode.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_recovery_code_confirm_required");
    } else if (recoveryCode != confirmRecoveryCode) {
      nextError = widget.controller.tr("ui_cloud_sync_recovery_code_mismatch");
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
    final resetToken = await widget.controller.cloudSyncPrepareReset();
    if (!mounted) {
      return;
    }
    if (resetToken == null) {
      setState(() {
        _validationError = widget.normalizeCloudSyncError(widget.controller.cloudSyncActionError.value);
        _isSubmitting = false;
      });
      return;
    }

    await widget.controller.cloudSyncReset(resetToken);
    if (!mounted) {
      return;
    }
    if (widget.controller.cloudSyncActionError.value.isNotEmpty) {
      setState(() {
        _validationError = widget.normalizeCloudSyncError(widget.controller.cloudSyncActionError.value);
        _isSubmitting = false;
      });
      return;
    }

    final started = await widget.controller.cloudSyncBootstrapStart(recoveryCode);
    if (!mounted) {
      return;
    }
    if (started) {
      Navigator.pop(context, true);
      return;
    }
    setState(() {
      _validationError = widget.normalizeCloudSyncError(widget.controller.cloudSyncActionError.value);
      _isSubmitting = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.controller.tr("ui_cloud_sync_forgot_recovery_code_title")),
      content: SizedBox(
        width: _dialogContentWidth,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(widget.controller.tr("ui_cloud_sync_forgot_recovery_code_description"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.45)),
            const SizedBox(height: 12),
            Text(widget.controller.tr("ui_cloud_sync_forgot_recovery_code_new_password"), style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600)),
            const SizedBox(height: 6),
            WoxTextField(
              controller: _recoveryController,
              hintText: widget.controller.tr("ui_cloud_sync_recovery_code_hint"),
              width: _dialogContentWidth,
              obscureText: true,
              enabled: !_isSubmitting,
              onSubmitted: (_) => _submit(),
            ),
            const SizedBox(height: 12),
            Text(
              widget.controller.tr("ui_cloud_sync_forgot_recovery_code_confirm_new_password"),
              style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 6),
            WoxTextField(
              controller: _confirmRecoveryController,
              hintText: widget.controller.tr("ui_cloud_sync_recovery_code_confirm_hint"),
              width: _dialogContentWidth,
              obscureText: true,
              enabled: !_isSubmitting,
              onSubmitted: (_) => _submit(),
            ),
            if (_validationError != null) ...[const SizedBox(height: 10), Text(_validationError!, style: const TextStyle(color: Colors.red, fontSize: 12))],
            if (_validationError == null && _isSubmitting) ...[
              const SizedBox(height: 10),
              Text(widget.controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
            ],
          ],
        ),
      ),
      actions: [
        WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: _isSubmitting ? null : () => Navigator.pop(context, false)),
        WoxButton.primary(
          text: _isSubmitting ? widget.controller.tr("ui_cloud_sync_loading") : widget.controller.tr("ui_cloud_sync_forgot_recovery_code_start_new_sync"),
          onPressed: _isSubmitting ? null : _submit,
        ),
      ],
    );
  }
}

// Owns bootstrap recovery-code controllers for both restore and first-sync flows.
class _CloudSyncBootstrapDialog extends StatefulWidget {
  final WoxSettingController controller;
  final WoxCloudSyncBootstrapStatus status;
  final _CloudSyncErrorNormalizer normalizeCloudSyncError;
  final Future<bool?> Function(BuildContext context) showForgotRecoveryCodeDialog;

  const _CloudSyncBootstrapDialog({required this.controller, required this.status, required this.normalizeCloudSyncError, required this.showForgotRecoveryCodeDialog});

  @override
  State<_CloudSyncBootstrapDialog> createState() => _CloudSyncBootstrapDialogState();
}

class _CloudSyncBootstrapDialogState extends State<_CloudSyncBootstrapDialog> {
  static const double _dialogContentWidth = 360.0;

  final _recoveryController = TextEditingController();
  final _confirmRecoveryController = TextEditingController();
  String? _validationError;
  bool _isSubmitting = false;

  @override
  void dispose() {
    _recoveryController.dispose();
    _confirmRecoveryController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_isSubmitting) {
      return;
    }

    final recoveryCode = _recoveryController.text.trim();
    final confirmRecoveryCode = _confirmRecoveryController.text.trim();
    String? nextError;
    if (recoveryCode.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_recovery_code_required");
    } else if (!widget.status.hasRemoteData && confirmRecoveryCode.isEmpty) {
      nextError = widget.controller.tr("ui_cloud_sync_recovery_code_confirm_required");
    } else if (!widget.status.hasRemoteData && recoveryCode != confirmRecoveryCode) {
      nextError = widget.controller.tr("ui_cloud_sync_recovery_code_mismatch");
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
    final started = await widget.controller.cloudSyncBootstrapStart(recoveryCode);
    if (!mounted) {
      return;
    }
    if (started) {
      Navigator.pop(context);
      return;
    }
    setState(() {
      _validationError = widget.normalizeCloudSyncError(widget.controller.cloudSyncActionError.value);
      _isSubmitting = false;
    });
  }

  Future<void> _startForgotRecoveryCodeFlow() async {
    final started = await widget.showForgotRecoveryCodeDialog(context);
    if (!mounted || started != true) {
      return;
    }
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      title: Text(widget.status.hasRemoteData ? widget.controller.tr("ui_cloud_sync_bootstrap_restore_title") : widget.controller.tr("ui_cloud_sync_bootstrap_start_title")),
      content: SizedBox(
        width: _dialogContentWidth,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              widget.status.hasRemoteData ? widget.controller.tr("ui_cloud_sync_bootstrap_restore_description") : widget.controller.tr("ui_cloud_sync_bootstrap_start_description"),
              style: TextStyle(color: getThemeSubTextColor(), fontSize: 12, height: 1.45),
            ),
            const SizedBox(height: 12),
            Text(widget.controller.tr("ui_cloud_sync_recovery_code"), style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600)),
            const SizedBox(height: 6),
            WoxTextField(
              controller: _recoveryController,
              hintText: widget.controller.tr("ui_cloud_sync_recovery_code_hint"),
              width: _dialogContentWidth,
              obscureText: true,
              enabled: !_isSubmitting,
              onSubmitted: (_) => _submit(),
            ),
            if (!widget.status.hasRemoteData) ...[
              const SizedBox(height: 12),
              Text(widget.controller.tr("ui_cloud_sync_recovery_code_confirm"), style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w600)),
              const SizedBox(height: 6),
              WoxTextField(
                controller: _confirmRecoveryController,
                hintText: widget.controller.tr("ui_cloud_sync_recovery_code_confirm_hint"),
                width: _dialogContentWidth,
                obscureText: true,
                enabled: !_isSubmitting,
                onSubmitted: (_) => _submit(),
              ),
            ],
            if (_validationError != null) ...[const SizedBox(height: 10), Text(_validationError!, style: const TextStyle(color: Colors.red, fontSize: 12))],
            if (_validationError == null && _isSubmitting) ...[
              const SizedBox(height: 10),
              Text(widget.controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
            ],
          ],
        ),
      ),
      actionsAlignment: widget.status.hasRemoteData ? MainAxisAlignment.spaceBetween : MainAxisAlignment.end,
      actions: [
        if (widget.status.hasRemoteData)
          WoxButton.text(text: widget.controller.tr("ui_cloud_sync_forgot_recovery_code"), onPressed: _isSubmitting ? null : _startForgotRecoveryCodeFlow),
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            WoxButton.secondary(text: widget.controller.tr("ui_cloud_sync_cancel"), onPressed: _isSubmitting ? null : () => Navigator.pop(context)),
            const SizedBox(width: 8),
            WoxButton.primary(
              text: _isSubmitting ? widget.controller.tr("ui_cloud_sync_loading") : widget.controller.tr("ui_cloud_sync_confirm"),
              onPressed: _isSubmitting ? null : _submit,
            ),
          ],
        ),
      ],
    );
  }
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
  static const _termsPath = "/terms";
  static const _privacyPath = "/privacy";

  late final TextEditingController _emailController;
  final _passwordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();
  String? _validationError;
  bool _isSubmitting = false;
  bool _hasAcceptedLegal = false;

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

  Future<void> _openLegalPage(String path) async {
    final langPrefix = widget.controller.accountRequestLang() == "zh" ? "/zh" : "";
    await launchUrl(Uri.https("sync.woxlauncher.com", "$langPrefix$path"), mode: LaunchMode.externalApplication);
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
    } else if (!_hasAcceptedLegal) {
      nextError = widget.controller.tr("ui_cloud_sync_account_terms_required");
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
    final canSubmit = !_isSubmitting && _hasAcceptedLegal;

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
          const SizedBox(height: 12),
          SizedBox(
            width: 360,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                WoxCheckbox(
                  value: _hasAcceptedLegal,
                  enabled: !_isSubmitting,
                  size: 18,
                  onChanged: (value) {
                    setState(() {
                      _hasAcceptedLegal = value ?? false;
                      if (_hasAcceptedLegal && _validationError == widget.controller.tr("ui_cloud_sync_account_terms_required")) {
                        _validationError = null;
                      }
                    });
                  },
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Wrap(
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      Text(widget.controller.tr("ui_cloud_sync_account_accept_prefix"), style: TextStyle(color: getThemeTextColor(), fontSize: 12)),
                      WoxButton.text(
                        text: widget.controller.tr("ui_cloud_sync_account_terms"),
                        fontSize: 12,
                        padding: EdgeInsets.zero,
                        onPressed: _isSubmitting ? null : () => _openLegalPage(_termsPath),
                      ),
                      Text(widget.controller.tr("ui_cloud_sync_account_accept_and"), style: TextStyle(color: getThemeTextColor(), fontSize: 12)),
                      WoxButton.text(
                        text: widget.controller.tr("ui_cloud_sync_account_privacy"),
                        fontSize: 12,
                        padding: EdgeInsets.zero,
                        onPressed: _isSubmitting ? null : () => _openLegalPage(_privacyPath),
                      ),
                    ],
                  ),
                ),
              ],
            ),
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
          onPressed: canSubmit ? _submit : null,
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
          SizedBox(
            width: 360,
            child: Text(
              widget.controller.tr("ui_cloud_sync_account_verify_message").replaceAll("{0}", widget.email),
              style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.35),
            ),
          ),
          const SizedBox(height: 16),
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
