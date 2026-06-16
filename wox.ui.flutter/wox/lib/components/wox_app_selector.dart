import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

class WoxAppSelector extends StatefulWidget {
  final IgnoredHotkeyApp value;
  final ValueChanged<IgnoredHotkeyApp> onChanged;

  const WoxAppSelector({super.key, required this.value, required this.onChanged});

  @override
  State<WoxAppSelector> createState() => _WoxAppSelectorState();
}

class _WoxAppSelectorState extends State<WoxAppSelector> {
  List<IgnoredHotkeyApp> _availableApps = <IgnoredHotkeyApp>[];

  String tr(String key) => Get.find<WoxSettingController>().tr(key);

  List<IgnoredHotkeyApp> _mergeSelectedApp(List<IgnoredHotkeyApp> candidates) {
    final merged = <IgnoredHotkeyApp>[];
    final seen = <String>{};

    if (widget.value.identity.trim().isNotEmpty) {
      merged.add(widget.value);
      seen.add(widget.value.identity.trim().toLowerCase());
    }

    for (final app in candidates) {
      final identity = app.identity.trim().toLowerCase();
      if (identity.isEmpty || seen.contains(identity)) {
        continue;
      }
      seen.add(identity);
      merged.add(app);
    }

    return merged;
  }

  Future<List<IgnoredHotkeyApp>> _loadAvailableApps() async {
    if (_availableApps.isNotEmpty) {
      return _availableApps;
    }

    final traceId = const UuidV4().generate();
    try {
      final apps = await WoxApi.instance.getHotkeyAppCandidates(traceId);
      final mergedApps = _mergeSelectedApp(apps);

      if (mounted) {
        setState(() {
          _availableApps = mergedApps;
        });
      } else {
        _availableApps = mergedApps;
      }

      return mergedApps;
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to load app selector candidates: $e');
      return _mergeSelectedApp(const <IgnoredHotkeyApp>[]);
    }
  }

  Future<void> _openSelector() async {
    final selectedApp = await showDialog<IgnoredHotkeyApp>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (ctx) => _AppSelectorDialog(initialApps: _availableApps, selectedApp: widget.value, loadApps: _loadAvailableApps),
    );

    if (selectedApp != null) {
      widget.onChanged(selectedApp);
    }
  }

  @override
  Widget build(BuildContext context) {
    final borderColor = getThemeSubTextColor().withValues(alpha: 0.45);
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final hasValue = widget.value.identity.trim().isNotEmpty;
    final subtitle = widget.value.path.trim().isNotEmpty ? widget.value.path.trim() : widget.value.identity.trim();

    return Row(
      children: [
        Expanded(
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            decoration: BoxDecoration(color: Colors.transparent, borderRadius: BorderRadius.circular(4), border: Border.all(color: borderColor)),
            child:
                hasValue
                    ? Row(
                      children: [
                        if (widget.value.icon.imageData.isNotEmpty) ...[
                          ClipRRect(borderRadius: BorderRadius.circular(6), child: WoxImageView(woxImage: widget.value.icon, width: 24, height: 24)),
                          const SizedBox(width: 10),
                        ],
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(widget.value.name, style: TextStyle(color: textColor, fontSize: 13)),
                              if (subtitle.isNotEmpty)
                                Padding(
                                  padding: const EdgeInsets.only(top: 2),
                                  child: Text(subtitle, style: TextStyle(color: subTextColor, fontSize: 11), maxLines: 1, overflow: TextOverflow.ellipsis),
                                ),
                            ],
                          ),
                        ),
                      ],
                    )
                    : Text(tr('ui_hotkey_ignore_apps_app_placeholder'), style: TextStyle(color: subTextColor, fontSize: 13)),
          ),
        ),
        const SizedBox(width: 10),
        WoxButton.primary(text: tr('ui_hotkey_ignore_apps_select'), onPressed: _openSelector),
      ],
    );
  }
}

class _AppSelectorDialog extends StatefulWidget {
  final List<IgnoredHotkeyApp> initialApps;
  final IgnoredHotkeyApp selectedApp;
  final Future<List<IgnoredHotkeyApp>> Function() loadApps;

  const _AppSelectorDialog({required this.initialApps, required this.selectedApp, required this.loadApps});

  @override
  State<_AppSelectorDialog> createState() => _AppSelectorDialogState();
}

class _AppSelectorDialogState extends State<_AppSelectorDialog> {
  late final TextEditingController _filterController;
  late String _selectedIdentity;
  late List<IgnoredHotkeyApp> _apps;
  bool _isLoading = false;
  String _loadError = '';

  String tr(String key) => Get.find<WoxSettingController>().tr(key);

  @override
  void initState() {
    super.initState();
    _filterController = TextEditingController();
    _selectedIdentity = widget.selectedApp.identity.trim().toLowerCase();
    _apps = List<IgnoredHotkeyApp>.from(widget.initialApps);
    _filterController.addListener(() {
      if (mounted) {
        setState(() {});
      }
    });

    if (_apps.isEmpty) {
      _loadApps();
    }
  }

  @override
  void dispose() {
    _filterController.dispose();
    super.dispose();
  }

  List<IgnoredHotkeyApp> get _filteredApps {
    final keyword = _filterController.text.trim().toLowerCase();
    if (keyword.isEmpty) {
      return _apps;
    }

    return _apps.where((app) => app.name.toLowerCase().contains(keyword) || app.identity.toLowerCase().contains(keyword) || app.path.toLowerCase().contains(keyword)).toList();
  }

  Future<void> _loadApps() async {
    if (_isLoading) {
      return;
    }

    setState(() {
      _isLoading = true;
      _loadError = '';
    });

    try {
      final apps = await widget.loadApps();
      if (!mounted) {
        return;
      }

      setState(() {
        _apps = apps;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }

      setState(() {
        _loadError = e.toString();
      });
    } finally {
      if (mounted) {
        setState(() {
          _isLoading = false;
        });
      }
    }
  }

  IgnoredHotkeyApp? _getSelectedApp() {
    if (_selectedIdentity.isEmpty) {
      return null;
    }

    for (final app in _apps) {
      if (app.identity.trim().toLowerCase() == _selectedIdentity) {
        return app;
      }
    }

    return null;
  }

  @override
  Widget build(BuildContext context) {
    final filteredApps = _filteredApps;
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final borderColor = getThemePopupOutlineColor();
    final accentColor = getThemeActiveBackgroundColor();

    return AlertDialog(
      backgroundColor: getThemePopupSurfaceColor(),
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20), side: BorderSide(color: borderColor)),
      elevation: 18,
      insetPadding: const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
      contentPadding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
      actionsPadding: const EdgeInsets.fromLTRB(24, 12, 24, 24),
      actionsAlignment: MainAxisAlignment.end,
      title: Text(tr('ui_hotkey_ignore_apps_dialog_title'), style: TextStyle(color: textColor, fontSize: 16)),
      content: SizedBox(
        width: 760,
        height: 540,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            WoxTextField(controller: _filterController, hintText: tr('ui_hotkey_ignore_apps_search'), width: double.infinity),
            if (_loadError.isNotEmpty) Padding(padding: const EdgeInsets.only(top: 10), child: Text(_loadError, style: TextStyle(color: subTextColor, fontSize: 11))),
            const SizedBox(height: 12),
            Expanded(
              child: Container(
                decoration: BoxDecoration(color: Colors.transparent, borderRadius: BorderRadius.circular(12), border: Border.all(color: borderColor.withValues(alpha: 0.9))),
                child:
                    _isLoading
                        ? Center(child: Text(tr('ui_hotkey_ignore_apps_loading'), style: TextStyle(color: subTextColor, fontSize: 13)))
                        : filteredApps.isEmpty
                        ? Center(child: Text(tr('ui_hotkey_ignore_apps_empty'), style: TextStyle(color: subTextColor, fontSize: 13)))
                        : ClipRRect(
                          borderRadius: BorderRadius.circular(12),
                          child: ListView.separated(
                            itemCount: filteredApps.length,
                            separatorBuilder: (context, index) => Divider(height: 1, thickness: 1, color: borderColor.withValues(alpha: 0.5)),
                            itemBuilder: (context, index) {
                              final app = filteredApps[index];
                              final identity = app.identity.trim().toLowerCase();
                              final isSelected = _selectedIdentity == identity;
                              final subtitle = app.path.trim().isNotEmpty ? app.path.trim() : app.identity.trim();

                              return Material(
                                color: isSelected ? accentColor.withValues(alpha: isThemeDark() ? 0.18 : 0.12) : Colors.transparent,
                                child: InkWell(
                                  onTap: () {
                                    setState(() {
                                      _selectedIdentity = identity;
                                    });
                                  },
                                  hoverColor: accentColor.withValues(alpha: 0.06),
                                  splashColor: accentColor.withValues(alpha: 0.08),
                                  highlightColor: accentColor.withValues(alpha: 0.04),
                                  child: Padding(
                                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                                    child: Row(
                                      children: [
                                        Theme(
                                          data: Theme.of(context).copyWith(
                                            radioTheme: RadioThemeData(
                                              fillColor: WidgetStateProperty.resolveWith((states) {
                                                if (states.contains(WidgetState.selected)) {
                                                  return accentColor;
                                                }
                                                return subTextColor.withValues(alpha: 0.75);
                                              }),
                                            ),
                                          ),
                                          child: Radio<String>(
                                            value: identity,
                                            groupValue: _selectedIdentity,
                                            onChanged: (value) => setState(() => _selectedIdentity = value ?? ''),
                                          ),
                                        ),
                                        if (app.icon.imageData.isNotEmpty) ...[
                                          ClipRRect(borderRadius: BorderRadius.circular(8), child: WoxImageView(woxImage: app.icon, width: 28, height: 28)),
                                          const SizedBox(width: 12),
                                        ],
                                        Expanded(
                                          child: Column(
                                            crossAxisAlignment: CrossAxisAlignment.start,
                                            children: [
                                              Text(app.name, style: TextStyle(color: textColor, fontSize: 13, fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400)),
                                              if (subtitle.isNotEmpty)
                                                Padding(
                                                  padding: const EdgeInsets.only(top: 2),
                                                  child: Text(subtitle, style: TextStyle(color: subTextColor, fontSize: 11), maxLines: 1, overflow: TextOverflow.ellipsis),
                                                ),
                                            ],
                                          ),
                                        ),
                                      ],
                                    ),
                                  ),
                                ),
                              );
                            },
                          ),
                        ),
              ),
            ),
          ],
        ),
      ),
      actions: [
        WoxButton.secondary(text: tr('ui_cancel'), onPressed: () => Navigator.of(context).pop()),
        WoxButton.primary(text: tr('ui_ok'), onPressed: () => Navigator.of(context).pop(_getSelectedApp())),
      ],
    );
  }
}
