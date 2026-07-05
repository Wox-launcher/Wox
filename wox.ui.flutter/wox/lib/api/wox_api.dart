import 'dart:convert';
import 'dart:core';
import 'dart:io' show pid;

import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_ai_command_template.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_cloud_sync.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_update_channel_version.dart';
import 'package:wox/entity/wox_usage_stats.dart';
import 'package:wox/entity/wox_window_manager.dart';
import 'package:wox/models/doctor_check_result.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxApi {
  WoxApi._privateConstructor();

  static final WoxApi _instance = WoxApi._privateConstructor();

  static WoxApi get instance => _instance;

  Future<WoxTheme> loadTheme(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxTheme>(traceId, "/theme", null);
  }

  Future<WoxSetting> loadSetting(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxSetting>(traceId, "/setting/wox", null);
  }

  Future<void> updateSetting(String traceId, String key, String value) async {
    await WoxHttpUtil.instance.postData(traceId, "/setting/wox/update", {"Key": key, "Value": value});
  }

  Future<void> show(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/show", null);
  }

  Future<List<IgnoredHotkeyApp>> getHotkeyAppCandidates(String traceId) async {
    return await WoxHttpUtil.instance.postData<List<IgnoredHotkeyApp>>(traceId, "/setting/hotkey/apps", null);
  }

  Future<List<WindowManagerDisplay>> getWindowManagerDisplays(String traceId) async {
    return await WoxHttpUtil.instance.postData<List<WindowManagerDisplay>>(traceId, "/setting/window-manager/displays", null);
  }

  Future<bool> getBrowserExtensionConnected(String traceId) async {
    final result = await WoxHttpUtil.instance.postData(traceId, "/browser/extension/status", null);
    if (result is Map) {
      return result['connected'] == true;
    }
    return false;
  }

  Future<List<String>> getSystemFontFamilies(String traceId) async {
    return await WoxHttpUtil.instance.postData<List<String>>(traceId, "/setting/ui/fonts", null);
  }

  Future<List<WoxRuntimeStatus>> getRuntimeStatuses(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/runtime/status", null);
  }

  Future<void> restartRuntime(String traceId, String runtime) async {
    await WoxHttpUtil.instance.postData(traceId, "/runtime/restart", {"Runtime": runtime});
  }

  Future<void> updatePluginSetting(String traceId, String pluginId, String key, String value) async {
    await WoxHttpUtil.instance.postData(traceId, "/setting/plugin/update", {"PluginId": pluginId, "Key": key, "Value": value});
  }

  Future<List<PluginDetail>> findStorePlugins(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/plugin/store", null);
  }

  Future<List<PluginDetail>> findInstalledPlugins(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/plugin/installed", null);
  }

  Future<PluginDetail> getPluginDetail(String traceId, String pluginId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/plugin/detail", {"id": pluginId});
  }

  Future<void> installPlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/install", {"id": id});
  }

  Future<void> uninstallPlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/uninstall", {"id": id});
  }

  Future<void> disablePlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/disable", {"id": id});
  }

  Future<void> enablePlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/enable", {"id": id});
  }

  Future<List<WoxTheme>> findStoreThemes(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/theme/store", null);
  }

  Future<List<WoxTheme>> findInstalledThemes(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/theme/installed", null);
  }

  Future<void> installTheme(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/theme/install", {"id": id});
  }

  Future<void> uninstallTheme(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/theme/uninstall", {"id": id});
  }

  Future<void> applyTheme(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/theme/apply", {"id": id});
  }

  Future<WoxTheme> saveTheme(String traceId, String name, WoxTheme theme, {bool overwrite = false}) async {
    return await WoxHttpUtil.instance.postData<WoxTheme>(traceId, "/theme/save", {"Name": name, "Theme": theme.toJson(), "Overwrite": overwrite});
  }

  Future<bool> isHotkeyAvailable(String traceId, String hotkey) async {
    return await WoxHttpUtil.instance.postData(traceId, "/hotkey/available", {"hotkey": hotkey});
  }

  Future<HotkeyAvailability> checkHotkeyAvailability(String traceId, String hotkey) async {
    return await WoxHttpUtil.instance.postData<HotkeyAvailability>(traceId, "/hotkey/availability", {"hotkey": hotkey});
  }

  Future<void> onUIReady(String traceId) async {
    // Dev mode starts Flutter outside the backend process tree, so the ready
    // callback reports the UI PID for core-side memory diagnostics.
    await WoxHttpUtil.instance.postData(traceId, "/on/ready", {"Pid": pid});
  }

  Future<void> onFocusLost(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/focus/lost", {});
  }

  Future<void> onShow(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/show", {});
  }

  Future<void> showTooltipOverlay(String traceId, String name, String text, String side, double anchorX, double anchorY, double anchorWidth, double anchorHeight) async {
    await WoxHttpUtil.instance.postData(traceId, "/tooltip/show", {
      "name": name,
      "text": text,
      "side": side,
      "anchorX": anchorX,
      "anchorY": anchorY,
      "anchorWidth": anchorWidth,
      "anchorHeight": anchorHeight,
    });
  }

  Future<void> hideTooltipOverlay(String traceId, String name) async {
    await WoxHttpUtil.instance.postData(traceId, "/tooltip/hide", {"name": name});
  }

  Future<void> onQueryBoxFocus(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/querybox/focus", {});
  }

  Future<void> onHide(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/hide", {});
  }

  Future<void> onSetting(String traceId, bool inSettingView) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/setting", {"inSettingView": inSettingView});
  }

  Future<void> onHotkeyRecording(String traceId, bool isRecording) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/hotkey/recording", {"isRecording": isRecording});
  }

  Future<void> onOnboarding(String traceId, bool inOnboardingView) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/onboarding", {"inOnboardingView": inOnboardingView});
  }

  Future<WoxUsageStats> getUsageStats(String traceId, {String period = '30d'}) async {
    return await WoxHttpUtil.instance.postData<WoxUsageStats>(traceId, "/usage/stats", {"Period": period});
  }

  Future<List<WoxLang>> getAllLanguages(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/lang/available", {});
  }

  Future<Map<String, String>> getLangJson(String traceId, String langCode) async {
    var langJsonStr = await WoxHttpUtil.instance.postData(traceId, "/lang/json", {"langCode": langCode});

    //unmarshal json string to map
    var jsonMap = json.decode(langJsonStr);
    return jsonMap.cast<String, String>();
  }

  Future<void> onProtocolUrlReceived(String traceId, String deeplink) async {
    await WoxHttpUtil.instance.postData(traceId, "/deeplink", {"deeplink": deeplink});
  }

  Future<List<AIModel>> findAIModels(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/models", null);
  }

  Future<List<AIProviderInfo>> findAIProviders(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/providers", null);
  }

  Future<String> pingAIModel(String traceId, String providerName, String apiKey, String host) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/ping", {"name": providerName, "apiKey": apiKey, "host": host});
  }

  Future<List<AIMCPTool>> findAIMCPServerTools(String traceId, dynamic data) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/mcp/tools", data);
  }

  Future<List<AIMCPTool>> findAIMCPServerToolsAll(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/mcp/tools/all", null);
  }

  Future<List<AISkill>> findAISkills(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/skills", null);
  }

  Future<List<AICommandTemplate>> findAICommandTemplates(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/commands/store", null);
  }

  Future<AIModel> findDefaultAIModel(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/model/default", null);
  }

  Future<void> sendChatRequest(String traceId, WoxAIChatData data) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/chat", {"chatData": data.toJson()});
  }

  Future<void> stopChatRequest(String traceId, String chatId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/chat/stop", {"chatId": chatId});
  }

  Future<void> deleteAIChat(String traceId, String chatId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/chat/delete", {"chatId": chatId});
  }

  Future<void> summarizeAIChat(String traceId, String chatId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/chat/summarize", {"chatId": chatId});
  }

  Future<void> answerAIQuestion(String traceId, String questionId, String answer) async {
    await WoxHttpUtil.instance.postData(traceId, "/ai/question/answer", {"questionId": questionId, "answer": answer});
  }

  Future<List<DoctorCheckResult>> doctorCheck(String traceId) async {
    return await WoxHttpUtil.instance.postData<List<DoctorCheckResult>>(traceId, "/doctor/check", null);
  }

  Future<void> openAccessibilityPermission(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/permission/accessibility/open", null);
  }

  Future<void> openPrivacyPermission(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/permission/privacy/open", null);
  }

  Future<List<GlanceItem>> getGlanceItems(String traceId, List<GlanceRef> glances, String reason) async {
    return await WoxHttpUtil.instance.postData<List<GlanceItem>>(traceId, "/glance", {"Glances": glances.map((item) => item.toJson()).toList(), "Reason": reason});
  }

  Future<void> executeGlanceAction(String traceId, String pluginId, String glanceId, String actionId) async {
    await WoxHttpUtil.instance.postData(traceId, "/glance/action", {"PluginId": pluginId, "GlanceId": glanceId, "ActionId": actionId});
  }

  Future<String> getUserDataLocation(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/setting/userdata/location", null);
  }

  Future<void> updateUserDataLocation(String traceId, String location) async {
    await WoxHttpUtil.instance.postData(traceId, "/setting/userdata/location/update", {"location": location});
  }

  Future<void> backupNow(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/backup/now", null);
  }

  Future<List<WoxBackup>> getAllBackups(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/backup/all", null);
  }

  Future<void> restoreBackup(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/backup/restore", {"id": id});
  }

  Future<String> getBackupFolder(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/backup/folder", null);
  }

  Future<void> clearLogs(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/log/clear", null);
  }

  Future<void> openLogFile(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/log/open", null);
  }

  Future<Map<String, dynamic>> getDiagnosticStatus(String traceId) async {
    final data = await WoxHttpUtil.instance.postData<Map<String, dynamic>>(traceId, "/diagnostics/status", null);
    return data;
  }

  Future<String> exportDiagnostics(String traceId) async {
    return await WoxHttpUtil.instance.postData<String>(traceId, "/diagnostics/export", null);
  }

  Future<void> open(String traceId, String path) async {
    await WoxHttpUtil.instance.postData(traceId, "/open", {"path": path});
  }

  Future<void> showPreviewImageOverlay(String traceId, WoxImage image) async {
    final start = DateTime.now();
    // Diagnostic logging: keep the Flutter-to-core boundary visible while investigating slow
    // image overlays without dumping full base64 payloads into ui.log.
    Logger.instance.info(traceId, "show preview image overlay api start: type=${image.imageType}, dataLength=${image.imageData.length}, data=${previewImageOverlayLogData(image)}");
    await WoxHttpUtil.instance.postData(traceId, "/preview/image/overlay", {"Image": image.toJson()});
    Logger.instance.info(traceId, "show preview image overlay api finished, cost ${DateTime.now().difference(start).inMilliseconds} ms");
  }

  String previewImageOverlayLogData(WoxImage image) {
    if (image.imageType == "base64" && image.imageData.length > 120) {
      return "${image.imageData.substring(0, 120)}...<truncated base64>";
    }
    if (image.imageData.length > 300) {
      return "${image.imageData.substring(0, 300)}...<truncated>";
    }
    return image.imageData;
  }

  Future<String> getWoxVersion(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/version", null);
  }

  Future<List<WoxUpdateChannelVersion>> getUpdateChannelVersions(String traceId) async {
    return await WoxHttpUtil.instance.postData<List<WoxUpdateChannelVersion>>(traceId, "/updater/channel/versions", null);
  }

  Future<void> saveWindowPosition(String traceId, int x, int y) async {
    await WoxHttpUtil.instance.postData(traceId, "/setting/position", {"x": x, "y": y});
  }

  Future<WoxCloudSyncStatus> getCloudSyncStatus(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxCloudSyncStatus>(traceId, "/sync/status", null);
  }

  Future<WoxCloudSyncBootstrapStatus> cloudSyncBootstrapStatus(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxCloudSyncBootstrapStatus>(traceId, "/sync/bootstrap/status", null);
  }

  Future<void> cloudSyncBootstrapStart(String traceId, String recoveryCode) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/bootstrap/start", {"recovery_code": recoveryCode});
  }

  Future<WoxAccountStatus> getAccountStatus(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxAccountStatus>(traceId, "/account/status", null);
  }

  Future<WoxAccountStatus> accountRefresh(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxAccountStatus>(traceId, "/account/refresh", null);
  }

  Future<WoxAccountActionResult> accountRegister(String traceId, String email, String password, String lang) async {
    return await WoxHttpUtil.instance.postData<WoxAccountActionResult>(traceId, "/account/register", {"email": email, "password": password, "lang": lang});
  }

  Future<WoxAccountActionResult> accountVerifyEmail(String traceId, String email, String code, String lang) async {
    return await WoxHttpUtil.instance.postData<WoxAccountActionResult>(traceId, "/account/verify_email", {"email": email, "code": code, "lang": lang});
  }

  Future<WoxAccountActionResult> accountLogin(String traceId, String email, String password, String lang) async {
    return await WoxHttpUtil.instance.postData<WoxAccountActionResult>(traceId, "/account/login", {"email": email, "password": password, "lang": lang});
  }

  Future<void> accountLogout(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/account/logout", null);
  }

  Future<void> accountResendVerification(String traceId, String email, String lang) async {
    await WoxHttpUtil.instance.postData(traceId, "/account/resend_verification", {"email": email, "lang": lang});
  }

  Future<void> accountPasswordResetRequest(String traceId, String email, String lang) async {
    await WoxHttpUtil.instance.postData(traceId, "/account/password_reset/request", {"email": email, "lang": lang});
  }

  Future<void> accountPasswordResetConfirm(String traceId, String token, String password, String lang) async {
    await WoxHttpUtil.instance.postData(traceId, "/account/password_reset/confirm", {"token": token, "password": password, "lang": lang});
  }

  Future<void> accountChangePassword(String traceId, String currentPassword, String newPassword, String lang) async {
    await WoxHttpUtil.instance.postData(traceId, "/account/change_password", {"current_password": currentPassword, "new_password": newPassword, "lang": lang});
  }

  Future<WoxBillingSession> accountBillingCheckout(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxBillingSession>(traceId, "/account/billing/checkout", null);
  }

  Future<WoxBillingSession> accountBillingPortal(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxBillingSession>(traceId, "/account/billing/portal", null);
  }

  Future<WoxBillingPlan> accountBillingPlan(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxBillingPlan>(traceId, "/account/billing/plan", null);
  }

  Future<void> cloudSyncEnable(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/enable", null);
  }

  Future<void> cloudSyncDisable(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/disable", null);
  }

  Future<void> cloudSyncPush(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/push", null);
  }

  Future<void> cloudSyncPull(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/pull", null);
  }

  Future<WoxCloudSyncDeviceList> cloudSyncDevicesList(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxCloudSyncDeviceList>(traceId, "/sync/devices/list", null);
  }

  Future<void> cloudSyncDeviceRevoke(String traceId, String targetDeviceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/devices/revoke", {"target_device_id": targetDeviceId});
  }

  Future<void> cloudSyncDeviceJoin(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/devices/join", null);
  }

  Future<void> cloudSyncKeyInit(String traceId, String recoveryCode, String deviceName) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/key/init", {"recovery_code": recoveryCode, "device_name": deviceName});
  }

  Future<void> cloudSyncKeyFetch(String traceId, String recoveryCode) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/key/fetch", {"recovery_code": recoveryCode});
  }

  Future<String> cloudSyncRecoveryCode(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/sync/key/recovery_code", null);
  }

  Future<Map<String, dynamic>> cloudSyncKeyResetPrepare(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/sync/key/reset/prepare", null);
  }

  Future<void> cloudSyncKeyReset(String traceId, String resetToken) async {
    await WoxHttpUtil.instance.postData(traceId, "/sync/key/reset", {"reset_token": resetToken, "confirm": true});
  }

  Future<void> dictationModelDownload(String traceId, String modelId) async {
    await WoxHttpUtil.instance.postData(traceId, "/dictation/model/download", {"modelId": modelId});
  }

  Future<List<dynamic>> dictationModelStatus(String traceId) async {
    final result = await WoxHttpUtil.instance.postData(traceId, "/dictation/model/status", null);
    if (result is List) {
      return result;
    }
    return [];
  }
}
