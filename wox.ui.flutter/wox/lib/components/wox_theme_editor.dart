import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_dialog_util.dart';
import 'package:wox/utils/wox_system_wallpaper_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxThemeEditor extends StatefulWidget {
  final WoxTheme initialTheme;

  const WoxThemeEditor({super.key, required this.initialTheme});

  @override
  State<WoxThemeEditor> createState() => _WoxThemeEditorState();
}

class _WoxThemeEditorState extends State<WoxThemeEditor> {
  static const double _controlPaneHeight = 140;
  static const int _queryBoxGroupIndex = 1;
  static const int _previewGroupIndex = 3;
  static const int _actionPanelGroupIndex = 4;
  static const List<_ThemeColorGroup> _colorGroups = [
    _ThemeColorGroup(labelKey: 'ui_theme_editor_group_window', tokens: [_ThemeColorToken(key: 'AppBackgroundColor', labelKey: 'ui_theme_editor_token_app_background')]),
    _ThemeColorGroup(
      labelKey: 'ui_theme_editor_group_query_box',
      tokens: [
        _ThemeColorToken(key: 'QueryBoxBackgroundColor', labelKey: 'ui_theme_editor_token_query_background'),
        _ThemeColorToken(key: 'QueryBoxFontColor', labelKey: 'ui_theme_editor_token_query_text'),
        _ThemeColorToken(key: 'QueryBoxCursorColor', labelKey: 'ui_theme_editor_token_query_cursor'),
        _ThemeColorToken(key: 'QueryBoxTextSelectionBackgroundColor', labelKey: 'ui_theme_editor_token_query_selection'),
      ],
    ),
    _ThemeColorGroup(
      labelKey: 'ui_theme_editor_group_results',
      tokens: [
        _ThemeColorToken(key: 'ResultItemTitleColor', labelKey: 'ui_theme_editor_token_result_title'),
        _ThemeColorToken(key: 'ResultItemSubTitleColor', labelKey: 'ui_theme_editor_token_result_subtitle'),
        _ThemeColorToken(key: 'ResultItemTailTextColor', labelKey: 'ui_theme_editor_token_result_tail'),
        _ThemeColorToken(key: 'ResultItemActiveBackgroundColor', labelKey: 'ui_theme_editor_token_result_active_background'),
        _ThemeColorToken(key: 'ResultItemActiveTitleColor', labelKey: 'ui_theme_editor_token_result_active_title'),
      ],
    ),
    _ThemeColorGroup(
      labelKey: 'ui_theme_editor_group_preview',
      tokens: [
        _ThemeColorToken(key: 'PreviewFontColor', labelKey: 'ui_theme_editor_token_preview_text'),
        _ThemeColorToken(key: 'PreviewPropertyTitleColor', labelKey: 'ui_theme_editor_token_preview_tag_border'),
        _ThemeColorToken(key: 'PreviewPropertyContentColor', labelKey: 'ui_theme_editor_token_preview_tag_text'),
        _ThemeColorToken(key: 'PreviewSplitLineColor', labelKey: 'ui_theme_editor_token_preview_split'),
        _ThemeColorToken(key: 'PreviewTextSelectionColor', labelKey: 'ui_theme_editor_token_preview_selection'),
      ],
    ),
    _ThemeColorGroup(
      labelKey: 'ui_theme_editor_group_action_panel',
      tokens: [
        _ThemeColorToken(key: 'ActionContainerBackgroundColor', labelKey: 'ui_theme_editor_token_action_background'),
        _ThemeColorToken(key: 'ActionContainerHeaderFontColor', labelKey: 'ui_theme_editor_token_action_header'),
        _ThemeColorToken(key: 'ActionItemFontColor', labelKey: 'ui_theme_editor_token_action_text'),
        _ThemeColorToken(key: 'ActionItemActiveBackgroundColor', labelKey: 'ui_theme_editor_token_action_active_background'),
        _ThemeColorToken(key: 'ActionItemActiveFontColor', labelKey: 'ui_theme_editor_token_action_active_text'),
        _ThemeColorToken(key: 'ActionQueryBoxBackgroundColor', labelKey: 'ui_theme_editor_token_action_query_background'),
      ],
    ),
    _ThemeColorGroup(
      labelKey: 'ui_theme_editor_group_toolbar',
      tokens: [
        _ThemeColorToken(key: 'ToolbarBackgroundColor', labelKey: 'ui_theme_editor_token_toolbar_background'),
        _ThemeColorToken(key: 'ToolbarFontColor', labelKey: 'ui_theme_editor_token_toolbar_text'),
      ],
    ),
  ];

  late WoxTheme _restoreTheme;
  late WoxTheme _sourceTheme;
  late WoxTheme _draftTheme;
  final ScrollController _controlScrollController = ScrollController();
  Timer? _previewFlashTimer;
  int _activeGroupIndex = 0;
  int _previewFlashNonce = 0;
  String _previewFlashTokenKey = '';
  String _systemWallpaperPath = '';
  bool _isSaving = false;
  String _errorMessage = '';

  WoxSettingController? get _settingController => Get.isRegistered<WoxSettingController>() ? Get.find<WoxSettingController>() : null;

  bool get _hasDraftChanges => jsonEncode(_draftTheme.toJson()) != jsonEncode(_sourceTheme.toJson());

  bool get _canOverwriteCurrentTheme => _sourceTheme.themeId.isNotEmpty && !_sourceTheme.isSystem && !_sourceTheme.isAutoAppearance;

  @override
  void initState() {
    super.initState();
    _startEditing(widget.initialTheme);
    unawaited(_loadSystemWallpaper());
  }

  @override
  void didUpdateWidget(covariant WoxThemeEditor oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.initialTheme.themeId != widget.initialTheme.themeId) {
      if (_settingController == null) {
        WoxThemeUtil.instance.changeTheme(_restoreTheme);
      }
      _startEditing(widget.initialTheme);
    }
  }

  @override
  void dispose() {
    if (_settingController == null) {
      WoxThemeUtil.instance.changeTheme(_restoreTheme);
    }
    _previewFlashTimer?.cancel();
    _controlScrollController.dispose();
    super.dispose();
  }

  // Capture the active theme separately from the edited source so discard can undo previews.
  void _startEditing(WoxTheme sourceTheme) {
    final settingController = _settingController;
    if (settingController != null) {
      final session = settingController.getOrCreateThemeEditorDraftSession(requestedTheme: sourceTheme, currentTheme: WoxThemeUtil.instance.currentTheme.value);
      _restoreTheme = _cloneTheme(session.restoreTheme);
      _sourceTheme = _cloneTheme(session.sourceTheme);
      _draftTheme = _cloneTheme(session.draftTheme);
    } else {
      _restoreTheme = _cloneTheme(WoxThemeUtil.instance.currentTheme.value);
      _sourceTheme = _cloneTheme(sourceTheme.themeId.isEmpty ? _restoreTheme : sourceTheme);
      _draftTheme = _cloneTheme(_sourceTheme);
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        WoxThemeUtil.instance.changeTheme(_draftTheme);
      }
    });
  }

  // Load the host desktop wallpaper for translucent-theme preview backdrops.
  Future<void> _loadSystemWallpaper() async {
    final wallpaperPath = await WoxSystemWallpaperUtil.instance.loadSystemWallpaperPath();
    if (wallpaperPath == null || !mounted) {
      return;
    }
    setState(() => _systemWallpaperPath = wallpaperPath);
  }

  WoxTheme _cloneTheme(WoxTheme theme) {
    return WoxTheme.fromJson(Map<String, dynamic>.from(theme.toJson()));
  }

  String _tr(String key) {
    return _settingController?.tr(key) ?? key;
  }

  String _themeColorValue(String key) {
    return (_draftTheme.toJson()[key] ?? '').toString();
  }

  void _updateThemeColor(String key, String cssColor) {
    final themeJson = Map<String, dynamic>.from(_draftTheme.toJson());
    themeJson[key] = cssColor;
    setState(() {
      _draftTheme = WoxTheme.fromJson(themeJson);
      _errorMessage = '';
    });
    _settingController?.updateThemeEditorDraft(_draftTheme);
    WoxThemeUtil.instance.changeTheme(_draftTheme);
  }

  void _flashPreviewToken(String key) {
    _previewFlashTimer?.cancel();
    setState(() {
      _previewFlashTokenKey = key;
      _previewFlashNonce++;
    });
    _previewFlashTimer = Timer(const Duration(milliseconds: 780), () {
      if (!mounted || _previewFlashTokenKey != key) {
        return;
      }
      setState(() => _previewFlashTokenKey = '');
    });
  }

  bool _isPreviewTokenFlashing(String key) {
    return _previewFlashTokenKey == key;
  }

  Widget _buildPreviewFlashMarker({required bool isVisible, double? left, double? top, double? right, double? bottom, double? width, double? height, BorderRadius? borderRadius}) {
    if (!isVisible) {
      return const SizedBox.shrink();
    }

    final fillsParent = left == null && top == null && right == null && bottom == null && width == null && height == null;
    return Positioned(
      left: fillsParent ? 0 : left,
      top: fillsParent ? 0 : top,
      right: fillsParent ? 0 : right,
      bottom: fillsParent ? 0 : bottom,
      width: width,
      height: height,
      child: _buildPreviewFlashOverlay(isVisible: true, borderRadius: borderRadius, child: const SizedBox.expand()),
    );
  }

  int _colorChannelToByte(double channel) {
    return (channel.clamp(0.0, 1.0) * 255).round().clamp(0, 255).toInt();
  }

  String _byteToHex(int byte) {
    return byte.clamp(0, 255).toInt().toRadixString(16).padLeft(2, '0');
  }

  String _colorToHex(Color color, {bool includeAlpha = false}) {
    final red = _byteToHex(_colorChannelToByte(color.r));
    final green = _byteToHex(_colorChannelToByte(color.g));
    final blue = _byteToHex(_colorChannelToByte(color.b));
    if (!includeAlpha) {
      return '#$red$green$blue'.toUpperCase();
    }

    final alpha = _byteToHex(_colorChannelToByte(color.a));
    return '#$red$green$blue$alpha'.toUpperCase();
  }

  String _colorToCss(Color color) {
    final alpha = color.a.clamp(0.0, 1.0).toDouble();
    return _colorToHex(color.withValues(alpha: alpha), includeAlpha: alpha < 0.995);
  }

  Color? _tryParseColorInput(String value) {
    final normalizedValue = value.trim();
    if (normalizedValue.isEmpty) {
      return null;
    }

    try {
      return fromCssColor(normalizedValue);
    } catch (_) {
      return null;
    }
  }

  HSVColor _colorToWheelHsv(Color color) {
    final hsvColor = HSVColor.fromColor(color);
    return hsvColor.withAlpha(color.a);
  }

  Future<void> _openColorDialog(_ThemeColorToken token) async {
    final originalCssColor = _themeColorValue(token.key);
    var selectedColor = safeFromCssColor(_themeColorValue(token.key));
    var selectedHsvColor = _colorToWheelHsv(selectedColor);
    final colorTextController = TextEditingController(text: _colorToCss(selectedColor));

    await showWoxDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (dialogContext, setDialogState) {
            void syncColorTextField(Color color) {
              final cssColor = _colorToCss(color);
              if (colorTextController.text == cssColor) {
                return;
              }
              colorTextController.value = TextEditingValue(text: cssColor, selection: TextSelection.collapsed(offset: cssColor.length));
            }

            void setColor(HSVColor hsvColor) {
              final color = hsvColor.toColor();
              setDialogState(() {
                selectedColor = color;
                selectedHsvColor = hsvColor;
                syncColorTextField(color);
              });
              _updateThemeColor(token.key, _colorToCss(color));
            }

            void setBrightness(double brightness) {
              final normalizedHsvColor = selectedHsvColor.withValue(brightness.clamp(0.0, 1.0).toDouble());
              final color = normalizedHsvColor.toColor();
              setDialogState(() {
                selectedColor = color;
                selectedHsvColor = normalizedHsvColor;
                syncColorTextField(color);
              });
              _updateThemeColor(token.key, _colorToCss(color));
            }

            void setAlpha(double alpha) {
              final color = selectedColor.withValues(alpha: alpha);
              setDialogState(() {
                selectedColor = color;
                selectedHsvColor = selectedHsvColor.withAlpha(alpha);
                syncColorTextField(color);
              });
              _updateThemeColor(token.key, _colorToCss(color));
            }

            void setColorFromInput(String value) {
              final parsedColor = _tryParseColorInput(value);
              if (parsedColor == null) {
                return;
              }

              final normalizedHsvColor = _colorToWheelHsv(parsedColor);
              setDialogState(() {
                selectedColor = parsedColor;
                selectedHsvColor = normalizedHsvColor;
              });
              _updateThemeColor(token.key, _colorToCss(parsedColor));
            }

            return AlertDialog(
              backgroundColor: getThemePopupSurfaceColor(),
              title: Text(_tr(token.labelKey), style: TextStyle(color: getThemeTextColor(), fontSize: 16)),
              content: SizedBox(
                width: 360,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Center(
                      child: _ColorWheelPicker(
                        hsvColor: selectedHsvColor,
                        onChanged: (hsvColor) {
                          setColor(hsvColor.withAlpha(selectedColor.a));
                        },
                      ),
                    ),
                    const SizedBox(height: 16),
                    Center(
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Container(
                            width: 48,
                            height: 36,
                            decoration: BoxDecoration(color: selectedColor, borderRadius: BorderRadius.circular(6), border: Border.all(color: getThemeDividerColor())),
                          ),
                          const SizedBox(width: 12),
                          WoxTextField(
                            controller: colorTextController,
                            width: 132,
                            contentPadding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
                            style: TextStyle(color: getThemeTextColor(), fontSize: 13),
                            onChanged: setColorFromInput,
                            onSubmitted: setColorFromInput,
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 12),
                    _buildDialogColorSlider(_tr('ui_theme_editor_brightness'), selectedHsvColor.value, 1, (value) {
                      setBrightness(value);
                    }),
                    _buildDialogColorSlider(_tr('ui_theme_editor_opacity'), selectedColor.a, 1, (value) {
                      setAlpha(value);
                    }),
                  ],
                ),
              ),
              actions: [
                WoxButton.secondary(
                  text: _tr('ui_cancel'),
                  onPressed: () {
                    _updateThemeColor(token.key, originalCssColor);
                    Navigator.of(dialogContext).pop();
                  },
                ),
                WoxButton.primary(
                  text: _tr('ui_ok'),
                  onPressed: () {
                    _updateThemeColor(token.key, _colorToCss(selectedColor));
                    Navigator.of(dialogContext).pop();
                  },
                ),
              ],
            );
          },
        );
      },
    );
    colorTextController.dispose();
  }

  Widget _buildDialogColorSlider(String label, double value, double max, ValueChanged<double> onChanged) {
    final displayValue = max == 1 ? '${(value * 100).round()}%' : value.round().toString();
    return Row(
      children: [
        SizedBox(width: 64, child: Text(label, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))),
        Expanded(
          child: Slider(
            min: 0,
            max: max,
            divisions: max == 1 ? 100 : 255,
            value: value.clamp(0.0, max).toDouble(),
            activeColor: getThemeActiveBackgroundColor(),
            inactiveColor: getThemeDividerColor(),
            onChanged: onChanged,
          ),
        ),
        SizedBox(width: 46, child: Text(displayValue, textAlign: TextAlign.right, style: TextStyle(color: getThemeTextColor(), fontSize: 12))),
      ],
    );
  }

  void _discard() {
    final restoreTheme = _settingController?.discardThemeEditorDraft() ?? _restoreTheme;
    setState(() {
      _restoreTheme = _cloneTheme(restoreTheme);
      _sourceTheme = _cloneTheme(restoreTheme);
      _draftTheme = _cloneTheme(restoreTheme);
      _errorMessage = '';
    });
    WoxThemeUtil.instance.changeTheme(restoreTheme);
  }

  // Reset horizontal token scrolling so each group opens from its first color.
  void _selectGroup(int index) {
    if (index == _activeGroupIndex) {
      return;
    }
    setState(() => _activeGroupIndex = index);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && _controlScrollController.hasClients) {
        _controlScrollController.jumpTo(0);
      }
    });
  }

  Future<void> _saveCurrent() async {
    if (!_canOverwriteCurrentTheme || !_hasDraftChanges) {
      return;
    }

    await _saveTheme(name: _sourceTheme.themeName, overwrite: true);
  }

  Future<void> _saveAs() async {
    final themeName = await _requestThemeName();
    if (themeName == null || themeName.trim().isEmpty) {
      return;
    }
    await _saveTheme(name: themeName.trim(), overwrite: false);
  }

  Future<void> _saveTheme({required String name, required bool overwrite}) async {
    setState(() {
      _isSaving = true;
      _errorMessage = '';
    });

    try {
      final savedTheme =
          await (_settingController?.saveThemeAs(_draftTheme, name, overwrite: overwrite) ??
              WoxApi.instance.saveTheme(const UuidV4().generate(), name, _draftTheme, overwrite: overwrite));
      _settingController?.commitThemeEditorDraft(savedTheme);
      WoxThemeUtil.instance.changeTheme(savedTheme);
      if (!mounted) {
        return;
      }
      setState(() {
        _restoreTheme = _cloneTheme(savedTheme);
        _sourceTheme = _cloneTheme(savedTheme);
        _draftTheme = _cloneTheme(savedTheme);
        _errorMessage = '';
      });
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), 'Failed to save edited theme: $e');
      if (mounted) {
        setState(() => _errorMessage = '${_tr('ui_theme_editor_save_failed')}: $e');
      }
    } finally {
      if (mounted) {
        setState(() => _isSaving = false);
      }
    }
  }

  Future<String?> _requestThemeName() async {
    final defaultName = _tr(
      'ui_theme_editor_default_theme_name',
    ).replaceAll('{name}', _draftTheme.themeName.isEmpty ? _tr('ui_theme_editor_default_theme') : _draftTheme.themeName);
    final nameController = TextEditingController(text: defaultName);
    String nameError = '';

    final result = await showWoxDialog<String>(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (dialogContext, setDialogState) {
            return AlertDialog(
              backgroundColor: getThemePopupSurfaceColor(),
              title: Text(_tr('ui_theme_editor_save_as_title'), style: TextStyle(color: getThemeTextColor(), fontSize: 16)),
              content: SizedBox(
                width: 360,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    WoxTextField(
                      controller: nameController,
                      autofocus: true,
                      width: double.infinity,
                      hintText: _tr('ui_theme_editor_save_as_hint'),
                      onSubmitted: (_) {
                        final value = nameController.text.trim();
                        if (value.isEmpty) {
                          setDialogState(() => nameError = _tr('ui_theme_editor_name_required'));
                          return;
                        }
                        Navigator.of(dialogContext).pop(value);
                      },
                    ),
                    if (nameError.isNotEmpty)
                      Padding(padding: const EdgeInsets.only(top: 8), child: Text(nameError, style: const TextStyle(color: Colors.redAccent, fontSize: 12))),
                  ],
                ),
              ),
              actions: [
                WoxButton.secondary(text: _tr('ui_cancel'), onPressed: () => Navigator.of(dialogContext).pop()),
                WoxButton.primary(
                  text: _tr('ui_theme_editor_save_as'),
                  onPressed: () {
                    final value = nameController.text.trim();
                    if (value.isEmpty) {
                      setDialogState(() => nameError = _tr('ui_theme_editor_name_required'));
                      return;
                    }
                    Navigator.of(dialogContext).pop(value);
                  },
                ),
              ],
            );
          },
        );
      },
    );

    nameController.dispose();
    return result;
  }

  Widget _buildControlPane() {
    final activeGroup = _colorGroups[_activeGroupIndex.clamp(0, _colorGroups.length - 1).toInt()];

    return Container(
      decoration: BoxDecoration(border: Border(top: BorderSide(color: getThemeSettingDividerColor().withValues(alpha: 0.72)))),
      padding: const EdgeInsets.fromLTRB(18, 12, 18, 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(crossAxisAlignment: CrossAxisAlignment.center, children: [Expanded(child: _buildGroupSelector()), const SizedBox(width: 14), _buildEditorActions()]),
          if (_errorMessage.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 7),
              child: Text(_errorMessage, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: Colors.redAccent, fontSize: 12)),
            ),
          const SizedBox(height: 10),
          Expanded(child: _buildActiveGroupEditor(activeGroup)),
        ],
      ),
    );
  }

  Widget _buildGroupSelector() {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(children: _colorGroups.asMap().entries.map((entry) => _buildGroupChip(entry.key, entry.value)).toList(growable: false)),
    );
  }

  Widget _buildGroupChip(int index, _ThemeColorGroup group) {
    final isActive = index == _activeGroupIndex;
    final activeColor = getThemeActiveBackgroundColor();
    final textColor = getThemeTextColor();

    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: InkWell(
        borderRadius: BorderRadius.circular(6),
        splashFactory: NoSplash.splashFactory,
        overlayColor: WidgetStateProperty.all(Colors.transparent),
        splashColor: Colors.transparent,
        highlightColor: Colors.transparent,
        hoverColor: Colors.transparent,
        enableFeedback: false,
        onTap: () => _selectGroup(index),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(
            color: isActive ? activeColor.withValues(alpha: 0.16) : Colors.transparent,
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: isActive ? activeColor.withValues(alpha: 0.32) : Colors.transparent),
          ),
          child: Text(
            _tr(group.labelKey),
            style: TextStyle(color: isActive ? textColor : textColor.withValues(alpha: 0.78), fontSize: 12, fontWeight: isActive ? FontWeight.w600 : FontWeight.w500),
          ),
        ),
      ),
    );
  }

  Widget _buildEditorActions() {
    final hasDraftChanges = _hasDraftChanges;
    return SizedBox(
      width: 370,
      child: Row(
        children: [
          Expanded(
            child: WoxButton.secondary(
              text: _tr('ui_theme_editor_discard'),
              icon: Icon(Icons.undo, size: 15, color: getThemeTextColor()),
              height: 40,
              padding: const EdgeInsets.symmetric(horizontal: 10),
              onPressed: _isSaving || !hasDraftChanges ? null : _discard,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: WoxButton.secondary(
              text: _tr('ui_theme_editor_overwrite'),
              icon: Icon(Icons.save_as_outlined, size: 15, color: getThemeTextColor()),
              height: 40,
              padding: const EdgeInsets.symmetric(horizontal: 10),
              onPressed: _isSaving || !hasDraftChanges || !_canOverwriteCurrentTheme ? null : _saveCurrent,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: WoxButton.primary(
              text: _isSaving ? _tr('ui_theme_editor_saving') : _tr('ui_theme_editor_save_as'),
              icon: Icon(Icons.save_outlined, size: 15, color: getThemeActionItemActiveColor()),
              height: 40,
              padding: const EdgeInsets.symmetric(horizontal: 10),
              onPressed: _isSaving ? null : _saveAs,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildActiveGroupEditor(_ThemeColorGroup group) {
    return Scrollbar(
      controller: _controlScrollController,
      thumbVisibility: true,
      child: SingleChildScrollView(
        controller: _controlScrollController,
        scrollDirection: Axis.horizontal,
        child: Row(children: group.tokens.map(_buildColorToken).toList(growable: false)),
      ),
    );
  }

  Widget _buildColorToken(_ThemeColorToken token) {
    final color = safeFromCssColor(_themeColorValue(token.key));

    return InkWell(
      borderRadius: BorderRadius.circular(7),
      splashFactory: NoSplash.splashFactory,
      overlayColor: WidgetStateProperty.all(Colors.transparent),
      splashColor: Colors.transparent,
      highlightColor: Colors.transparent,
      hoverColor: Colors.transparent,
      enableFeedback: false,
      onTap: () => unawaited(_openColorDialog(token)),
      child: Container(
        width: 190,
        margin: const EdgeInsets.only(right: 12, bottom: 8),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(borderRadius: BorderRadius.circular(7), border: Border.all(color: getThemeSettingDividerColor().withValues(alpha: 0.58))),
        child: Row(
          children: [
            Expanded(child: Text(_tr(token.labelKey), maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 12))),
            Tooltip(
              message: _tr('ui_theme_editor_locate_token'),
              child: IconButton(
                visualDensity: VisualDensity.compact,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints.tightFor(width: 26, height: 26),
                icon: Icon(Icons.my_location_outlined, size: 15, color: getThemeSubTextColor()),
                onPressed: () => _flashPreviewToken(token.key),
              ),
            ),
            const SizedBox(width: 8),
            Tooltip(
              message: _tr('ui_theme_editor_choose_color'),
              child: Container(
                width: 28,
                height: 28,
                decoration: BoxDecoration(color: color, borderRadius: BorderRadius.circular(6), border: Border.all(color: getThemeDividerColor())),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPreviewFlashOverlay({required Widget child, required bool isVisible, BorderRadius? borderRadius}) {
    if (!isVisible) {
      return child;
    }

    return Stack(
      fit: StackFit.passthrough,
      children: [
        child,
        Positioned.fill(
          child: IgnorePointer(
            child: TweenAnimationBuilder<double>(
              key: ValueKey('$_previewFlashNonce-$_previewFlashTokenKey'),
              tween: Tween<double>(begin: 1, end: 0),
              duration: const Duration(milliseconds: 720),
              curve: Curves.easeOutCubic,
              builder: (context, value, child) {
                return DecoratedBox(
                  decoration: BoxDecoration(
                    color: Colors.redAccent.withValues(alpha: 0.2 * value),
                    borderRadius: borderRadius ?? BorderRadius.circular(7),
                    border: Border.all(color: Colors.redAccent.withValues(alpha: 0.9 * value), width: 2),
                  ),
                );
              },
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildLivePreview() {
    final theme = WoxThemeUtil.instance.currentTheme.value;
    return Padding(
      padding: const EdgeInsets.fromLTRB(18, 10, 18, 10),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 900, maxHeight: 420),
          child: LayoutBuilder(
            builder: (context, constraints) {
              final stageSize = Size(constraints.maxWidth, constraints.maxHeight);
              final windowSize = Size(math.min(780, math.max(320, stageSize.width * 0.78)).toDouble(), math.min(360, math.max(240, stageSize.height * 0.82)).toDouble());
              return ClipRRect(
                borderRadius: BorderRadius.circular(18),
                child: Container(
                  color: getThemeBackgroundColor(),
                  foregroundDecoration: BoxDecoration(border: Border.all(color: getThemeDividerColor()), borderRadius: BorderRadius.circular(18)),
                  child: Stack(
                    fit: StackFit.expand,
                    children: [_buildSystemWallpaperBackdrop(), Center(child: SizedBox(width: windowSize.width, height: windowSize.height, child: _buildPreviewWindow(theme)))],
                  ),
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  // Build the simulated Wox window as a separate layer above the wallpaper stage.
  Widget _buildPreviewWindow(WoxTheme theme) {
    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.24), blurRadius: 28, offset: const Offset(0, 16))],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(12),
        child: BackdropFilter(
          filter: ui.ImageFilter.blur(sigmaX: 24, sigmaY: 24),
          child: _buildPreviewFlashOverlay(
            isVisible: _isPreviewTokenFlashing('AppBackgroundColor'),
            borderRadius: BorderRadius.circular(12),
            child: Container(
              decoration: BoxDecoration(color: _previewMicaSurfaceColor(theme), border: Border.all(color: getThemeDividerColor()), borderRadius: BorderRadius.circular(12)),
              child: _buildPreviewSurface(theme),
            ),
          ),
        ),
      ),
    );
  }

  Color _previewMicaSurfaceColor(WoxTheme theme) {
    final appColor = safeFromCssColor(theme.appBackgroundColor);
    if (appColor.a >= 0.96) {
      return appColor;
    }

    final isDarkSurface = appColor.computeLuminance() < 0.5;
    final tint = isDarkSurface ? const Color(0xFF202020) : const Color(0xFFF2F2F2);
    final mixed = Color.lerp(appColor.withValues(alpha: 1), tint, 0.18) ?? appColor;
    final alpha = (0.64 + appColor.a * 0.18).clamp(0.64, 0.86).toDouble();
    return mixed.withValues(alpha: alpha);
  }

  Widget _buildSystemWallpaperBackdrop() {
    if (_systemWallpaperPath.isEmpty) {
      return const SizedBox.shrink();
    }

    return Image.file(File(_systemWallpaperPath), fit: BoxFit.cover, errorBuilder: (_, _, _) => const SizedBox.shrink());
  }

  Widget _buildPreviewSurface(WoxTheme theme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final isPreviewVisible = _activeGroupIndex == _previewGroupIndex;
    final isActionPanelVisible = _activeGroupIndex == _actionPanelGroupIndex;
    final contentPadding = EdgeInsets.only(
      top: theme.appPaddingTop.toDouble(),
      right: theme.appPaddingRight.toDouble(),
      bottom: theme.appPaddingBottom.toDouble(),
      left: theme.appPaddingLeft.toDouble(),
    );

    return Column(
      children: [
        Expanded(
          child: Padding(
            padding: contentPadding,
            child: LayoutBuilder(
              builder: (context, constraints) {
                final queryBoxHeight = metrics.queryBoxBaseHeight;
                final previewWidth = math.max(220.0, constraints.maxWidth * 0.43);
                final actionPanelWidth = math.min(metrics.actionPanelMaxWidth, 240.0);
                return Stack(
                  children: [
                    AnimatedPositioned(
                      duration: const Duration(milliseconds: 240),
                      curve: Curves.easeOutCubic,
                      top: queryBoxHeight,
                      left: 0,
                      right: isPreviewVisible ? previewWidth : 0,
                      bottom: 0,
                      child: _buildPreviewResults(theme, hasTrailingPanel: isPreviewVisible),
                    ),
                    Positioned(top: 0, left: 0, right: 0, height: queryBoxHeight, child: _buildPreviewQueryBox(theme)),
                    AnimatedPositioned(
                      duration: const Duration(milliseconds: 240),
                      curve: Curves.easeOutCubic,
                      top: queryBoxHeight,
                      right: isPreviewVisible ? 0 : -previewWidth - 16,
                      bottom: 0,
                      width: previewWidth,
                      child: IgnorePointer(
                        ignoring: !isPreviewVisible,
                        child: AnimatedOpacity(duration: const Duration(milliseconds: 180), opacity: isPreviewVisible ? 1 : 0, child: _buildPreviewTextPanel(theme)),
                      ),
                    ),
                    AnimatedPositioned(
                      duration: const Duration(milliseconds: 220),
                      curve: Curves.easeOutCubic,
                      right: isActionPanelVisible ? metrics.actionPanelOffsetRight : -actionPanelWidth - 16,
                      bottom: isActionPanelVisible ? metrics.actionPanelOffsetBottom : metrics.actionPanelOffsetBottom,
                      width: actionPanelWidth,
                      child: IgnorePointer(
                        ignoring: !isActionPanelVisible,
                        child: AnimatedOpacity(duration: const Duration(milliseconds: 160), opacity: isActionPanelVisible ? 1 : 0, child: _buildPreviewActionPanel(theme)),
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
        ),
        _buildPreviewToolbar(theme),
      ],
    );
  }

  Widget _buildPreviewQueryBox(WoxTheme theme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final queryTextColor = safeFromCssColor(theme.queryBoxFontColor);
    final rightAccessoryWidth =
        math.max(metrics.queryBoxRightAccessoryWidth, metrics.queryBoxGlanceHPadding + metrics.queryBoxGlanceIconAndGapWidth + metrics.scaledSpacing(70)).toDouble();

    return _buildPreviewFlashOverlay(
      isVisible: _isPreviewTokenFlashing('QueryBoxBackgroundColor'),
      borderRadius: BorderRadius.circular(theme.queryBoxBorderRadius.toDouble()),
      child: Container(
        height: metrics.queryBoxBaseHeight,
        decoration: BoxDecoration(color: safeFromCssColor(theme.queryBoxBackgroundColor), borderRadius: BorderRadius.circular(theme.queryBoxBorderRadius.toDouble())),
        child: Stack(
          children: [
            Positioned.fill(
              child: Padding(
                padding: EdgeInsets.only(left: 8, right: rightAccessoryWidth),
                child: Row(
                  children: [
                    Flexible(child: _buildPreviewQueryText(theme, queryTextColor)),
                    const SizedBox(width: 4),
                    _buildPreviewFlashOverlay(
                      isVisible: _isPreviewTokenFlashing('QueryBoxCursorColor'),
                      borderRadius: BorderRadius.circular(2),
                      child: Container(width: 2, height: metrics.queryBoxLineHeight * 0.72, color: safeFromCssColor(theme.queryBoxCursorColor)),
                    ),
                  ],
                ),
              ),
            ),
            Positioned(
              right: 6,
              top: 0,
              bottom: 0,
              width: rightAccessoryWidth,
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  Icon(Icons.memory_rounded, size: metrics.queryBoxGlanceIconAndGapWidth, color: queryTextColor.withValues(alpha: 0.72)),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      '761 MB',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: queryTextColor.withValues(alpha: 0.72), fontSize: metrics.queryBoxGlanceFontSize),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPreviewQueryText(WoxTheme theme, Color queryTextColor) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final isSelectionVisible = _activeGroupIndex == _queryBoxGroupIndex;
    final textStyle = TextStyle(color: queryTextColor, fontSize: metrics.queryBoxFontSize, height: 1);
    final selectedTextStyle = textStyle.copyWith(color: safeFromCssColor(theme.queryBoxTextSelectionColor, defaultColor: queryTextColor));
    final selectionColor = safeFromCssColor(theme.queryBoxTextSelectionBackgroundColor);

    return TweenAnimationBuilder<double>(
      tween: Tween<double>(begin: 0, end: isSelectionVisible ? 1 : 0),
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
      builder: (context, selectionProgress, child) {
        return Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            _buildPreviewFlashOverlay(
              isVisible: _isPreviewTokenFlashing('QueryBoxFontColor'),
              borderRadius: BorderRadius.circular(3),
              child: Text('theme ', maxLines: 1, overflow: TextOverflow.clip, style: textStyle),
            ),
            _buildPreviewFlashOverlay(
              isVisible: _isPreviewTokenFlashing('QueryBoxTextSelectionBackgroundColor'),
              borderRadius: BorderRadius.circular(3),
              child: Stack(
                alignment: Alignment.centerLeft,
                children: [
                  Text('edit', maxLines: 1, overflow: TextOverflow.clip, style: textStyle),
                  ClipRect(
                    child: Align(
                      alignment: Alignment.centerLeft,
                      widthFactor: selectionProgress.clamp(0.0, 1.0).toDouble(),
                      child: ColoredBox(color: selectionColor, child: Text('edit', maxLines: 1, overflow: TextOverflow.clip, style: selectedTextStyle)),
                    ),
                  ),
                ],
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _buildPreviewResults(WoxTheme theme, {required bool hasTrailingPanel}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final items = _buildPreviewResultItems();
    final itemHeight = WoxThemeUtil.instance.getResultItemHeight();
    final trailingInset = hasTrailingPanel ? metrics.scaledSpacing(14) : 0.0;
    return Padding(
      padding: EdgeInsets.only(
        top: metrics.scaledSpacing(theme.resultContainerPaddingTop.toDouble()),
        right: metrics.scaledSpacing(theme.resultContainerPaddingRight.toDouble()) + trailingInset,
        bottom: metrics.scaledSpacing(theme.resultContainerPaddingBottom.toDouble()),
        left: metrics.scaledSpacing(theme.resultContainerPaddingLeft.toDouble()),
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final visibleCount = math.min(items.length, math.max(1, (constraints.maxHeight / itemHeight).floor()));
          return Column(
            children: [
              for (final entry in items.take(visibleCount).toList().asMap().entries)
                SizedBox(height: itemHeight, child: _buildPreviewResultRow(theme, entry.value, isActive: entry.key == 0)),
            ],
          );
        },
      ),
    );
  }

  Widget _buildPreviewResultRow(WoxTheme theme, WoxListItem<WoxQueryResult> item, {required bool isActive}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final itemHeight = WoxThemeUtil.instance.getResultItemHeight();
    final maxBorderWidth = math.max(theme.resultItemBorderLeftWidth.toDouble(), theme.resultItemActiveBorderLeftWidth.toDouble());
    final leftPadding = metrics.scaledSpacing(theme.resultItemPaddingLeft.toDouble() + maxBorderWidth);
    final topPadding = metrics.scaledSpacing(theme.resultItemPaddingTop.toDouble());
    final textLeft = leftPadding + metrics.resultItemIconPaddingLeft + metrics.resultIconSize + metrics.resultItemIconPaddingRight;
    final textWidth = math.max(80.0, 210.0 * metrics.scale);
    final titleHeight = metrics.resultTitleFontSize + 5;
    final subtitleHeight = metrics.resultSubtitleFontSize + 5;
    final titleTop = topPadding + math.max(0.0, (metrics.resultItemBaseHeight - titleHeight - metrics.resultItemSubtitlePaddingTop - subtitleHeight) / 2);
    final subtitleTop = titleTop + titleHeight + metrics.resultItemSubtitlePaddingTop;
    final tailHeight = metrics.tailHotkeyFontSize + metrics.resultItemTextTailVPadding * 2 + 4;
    final tailWidth = math.min(110.0, math.max(76.0, 92.0 * metrics.scale));
    final tailTop = math.max(0.0, (itemHeight - tailHeight) / 2);
    final rowRadius = BorderRadius.circular(theme.resultItemBorderRadius.toDouble());

    return Stack(
      fit: StackFit.expand,
      children: [
        WoxListItemView(item: item, woxTheme: theme, isActive: isActive, isHovered: false, listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code),
        _buildPreviewFlashMarker(isVisible: isActive && _isPreviewTokenFlashing('ResultItemActiveBackgroundColor'), borderRadius: rowRadius),
        _buildPreviewFlashMarker(isVisible: !isActive && _isPreviewTokenFlashing('ResultItemTitleColor'), left: textLeft, top: titleTop, width: textWidth, height: titleHeight),
        _buildPreviewFlashMarker(
          isVisible: !isActive && _isPreviewTokenFlashing('ResultItemSubTitleColor'),
          left: textLeft,
          top: subtitleTop,
          width: textWidth,
          height: subtitleHeight,
        ),
        _buildPreviewFlashMarker(isVisible: !isActive && _isPreviewTokenFlashing('ResultItemTailTextColor'), top: tailTop, right: 8, width: tailWidth, height: tailHeight),
        _buildPreviewFlashMarker(
          isVisible: isActive && _isPreviewTokenFlashing('ResultItemActiveTitleColor'),
          left: textLeft,
          top: titleTop,
          width: textWidth,
          height: titleHeight,
        ),
      ],
    );
  }

  // Build sample results through the real list-item renderer so spacing mirrors the launcher.
  List<WoxListItem<WoxQueryResult>> _buildPreviewResultItems() {
    return [
      _buildPreviewResultItem(
        id: 'theme-editor',
        icon: _previewGearIcon(),
        title: _tr('ui_theme_editor_preview_result_theme'),
        subtitle: _tr('ui_theme_editor_preview_result_current'),
        tails: ['P1', '13ms'],
      ),
      _buildPreviewResultItem(
        id: 'query-box',
        icon: _previewSearchIcon(),
        title: _tr('ui_theme_editor_preview_result_query'),
        subtitle: 'QueryBoxBackgroundColor',
        tails: ['P1', '4ms'],
      ),
      _buildPreviewResultItem(
        id: 'results',
        icon: _previewListIcon(),
        title: _tr('ui_theme_editor_group_results'),
        subtitle: 'ResultItemActiveBackgroundColor',
        tails: ['P1', '4ms'],
      ),
      _buildPreviewResultItem(id: 'toolbar', icon: _previewCommandIcon(), title: _tr('ui_theme_editor_group_toolbar'), subtitle: 'ToolbarBackgroundColor', tails: ['P1', '13ms']),
      _buildPreviewResultItem(
        id: 'action-panel',
        icon: _previewMoreIcon(),
        title: _tr('ui_theme_editor_group_action_panel'),
        subtitle: 'ActionContainerBackgroundColor',
        tails: ['P1', '13ms'],
      ),
    ];
  }

  WoxImage _previewSvgIcon({required String background, required String body}) {
    return WoxImage(
      imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code,
      imageData: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="14" fill="$background"/>$body</svg>',
    );
  }

  WoxImage _previewGearIcon() {
    return _previewSvgIcon(
      background: '#8B5CF6',
      body:
          '<circle cx="32" cy="32" r="11" fill="none" stroke="#FFFFFF" stroke-width="5"/><path d="M32 14v8M32 42v8M14 32h8M42 32h8M19 19l6 6M39 39l6 6M45 19l-6 6M25 39l-6 6" fill="none" stroke="#FFFFFF" stroke-width="5" stroke-linecap="round"/>',
    );
  }

  WoxImage _previewSearchIcon() {
    return _previewSvgIcon(
      background: '#0EA5E9',
      body:
          '<circle cx="29" cy="29" r="13" fill="none" stroke="#FFFFFF" stroke-width="6"/><path d="M39 39l11 11" fill="none" stroke="#FFFFFF" stroke-width="6" stroke-linecap="round"/>',
    );
  }

  WoxImage _previewListIcon() {
    return _previewSvgIcon(
      background: '#22C55E',
      body:
          '<path d="M20 22h24M20 32h24M20 42h24" fill="none" stroke="#FFFFFF" stroke-width="6" stroke-linecap="round"/><circle cx="13" cy="22" r="3" fill="#FFFFFF"/><circle cx="13" cy="32" r="3" fill="#FFFFFF"/><circle cx="13" cy="42" r="3" fill="#FFFFFF"/>',
    );
  }

  WoxImage _previewCommandIcon() {
    return _previewSvgIcon(
      background: '#F59E0B',
      body:
          '<path d="M24 24h16v16H24z" fill="none" stroke="#FFFFFF" stroke-width="5"/><path d="M24 13v8M40 13v8M24 43v8M40 43v8M13 24h8M43 24h8M13 40h8M43 40h8" fill="none" stroke="#FFFFFF" stroke-width="5" stroke-linecap="round"/>',
    );
  }

  WoxImage _previewMoreIcon() {
    return _previewSvgIcon(
      background: '#F43F5E',
      body: '<circle cx="20" cy="32" r="5" fill="#FFFFFF"/><circle cx="32" cy="32" r="5" fill="#FFFFFF"/><circle cx="44" cy="32" r="5" fill="#FFFFFF"/>',
    );
  }

  WoxImage _previewCopyIcon() {
    return _previewSvgIcon(
      background: '#14B8A6',
      body:
          '<rect x="17" y="21" width="24" height="26" rx="4" fill="none" stroke="#FFFFFF" stroke-width="5"/><path d="M25 17h18a4 4 0 0 1 4 4v18" fill="none" stroke="#FFFFFF" stroke-width="5" stroke-linecap="round"/>',
    );
  }

  WoxImage _previewOpenIcon() {
    return _previewSvgIcon(
      background: '#6366F1',
      body:
          '<path d="M22 42l20-20M30 22h12v12" fill="none" stroke="#FFFFFF" stroke-width="6" stroke-linecap="round" stroke-linejoin="round"/><rect x="17" y="27" width="25" height="20" rx="4" fill="none" stroke="#FFFFFF" stroke-width="5"/>',
    );
  }

  WoxListItem<WoxQueryResult> _buildPreviewResultItem({required String id, required WoxImage icon, required String title, required String subtitle, required List<String> tails}) {
    final result = WoxQueryResult(
      queryId: 'theme-editor-preview',
      id: id,
      title: title,
      subTitle: subtitle,
      icon: icon,
      preview: WoxPreview.empty(),
      score: 0,
      group: '',
      groupScore: 0,
      tails: tails.map((tail) => WoxListItemTail(type: WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_TEXT.code, text: tail)).toList(growable: false),
      actions: [],
      isGroup: false,
    );
    return WoxListItem.fromQueryResult(result);
  }

  WoxListItem<WoxResultAction> _buildPreviewActionItem({required String id, required WoxImage icon, required String title}) {
    return WoxListItem<WoxResultAction>(id: id, icon: icon, title: title, subTitle: '', tails: [], isGroup: false, data: WoxResultAction.empty());
  }

  Widget _buildPreviewTextPanel(WoxTheme theme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final fontColor = safeFromCssColor(theme.previewFontColor);
    final splitLineColor = safeFromCssColor(theme.previewSplitLineColor);
    final tagBorderColor = safeFromCssColor(theme.previewPropertyTitleColor, defaultColor: splitLineColor);

    return Stack(
      children: [
        Container(
          decoration: BoxDecoration(border: Border(left: BorderSide(color: splitLineColor))),
          child: Container(
            padding: EdgeInsets.only(top: metrics.scaledSpacing(12), bottom: metrics.scaledSpacing(10), left: metrics.scaledSpacing(14), right: metrics.scaledSpacing(12)),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Container(
                    width: double.infinity,
                    padding: EdgeInsets.all(metrics.previewTextPadding * 0.66),
                    decoration: BoxDecoration(
                      color: fontColor.withValues(alpha: 0.035),
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: splitLineColor.withValues(alpha: 0.45)),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _buildPreviewFlashOverlay(
                          isVisible: _isPreviewTokenFlashing('PreviewFontColor'),
                          child: Text(
                            _tr('ui_theme_editor_preview_title'),
                            style: TextStyle(color: fontColor, fontSize: metrics.previewTextQuoteFontSize, fontWeight: FontWeight.w600),
                          ),
                        ),
                        const SizedBox(height: 10),
                        _buildPreviewFlashOverlay(
                          isVisible: _isPreviewTokenFlashing('PreviewFontColor'),
                          child: Text(
                            _tr('ui_theme_editor_preview_body'),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(color: fontColor.withValues(alpha: 0.86), fontSize: metrics.previewTextFontSize, height: 1.35),
                          ),
                        ),
                        const SizedBox(height: 8),
                        _buildPreviewSelectionSample(theme),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 10),
                _buildPreviewTagStrip(theme, tagBorderColor),
              ],
            ),
          ),
        ),
        _buildPreviewFlashMarker(isVisible: _isPreviewTokenFlashing('PreviewSplitLineColor'), left: 0, top: 0, bottom: 0, width: 3, borderRadius: BorderRadius.zero),
      ],
    );
  }

  Widget _buildPreviewSelectionSample(WoxTheme theme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final fontColor = safeFromCssColor(theme.previewFontColor);
    final selectionColor = safeFromCssColor(theme.previewTextSelectionColor);
    final textStyle = TextStyle(color: fontColor.withValues(alpha: 0.82), fontSize: metrics.previewTextFontSize, height: 1.2);

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text('select ', style: textStyle, maxLines: 1, overflow: TextOverflow.clip),
        _buildPreviewFlashOverlay(
          isVisible: _isPreviewTokenFlashing('PreviewTextSelectionColor'),
          borderRadius: BorderRadius.circular(3),
          child: ColoredBox(color: selectionColor, child: Text('preview', style: textStyle, maxLines: 1, overflow: TextOverflow.clip)),
        ),
      ],
    );
  }

  Widget _buildPreviewTagStrip(WoxTheme theme, Color tagBorderColor) {
    final tags = ['2026-05-26 10:47:08', '2074x679', '702.7 KB', 'OCR'];
    return SizedBox(
      height: WoxInterfaceSizeUtil.instance.current.scaledSpacing(26),
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: tags.length,
        separatorBuilder: (context, index) => SizedBox(width: WoxInterfaceSizeUtil.instance.current.scaledSpacing(8)),
        itemBuilder: (context, index) {
          return _buildPreviewTagPill(theme, tags[index], tagBorderColor: tagBorderColor);
        },
      ),
    );
  }

  Widget _buildPreviewTagPill(WoxTheme theme, String label, {required Color tagBorderColor}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final fontColor = safeFromCssColor(theme.previewFontColor);
    final contentColor = safeFromCssColor(theme.previewPropertyContentColor);

    return _buildPreviewFlashOverlay(
      isVisible: _isPreviewTokenFlashing('PreviewPropertyTitleColor'),
      borderRadius: BorderRadius.circular(8),
      child: Container(
        constraints: BoxConstraints(maxWidth: metrics.scaledSpacing(220)),
        padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(9), vertical: metrics.scaledSpacing(4)),
        decoration: BoxDecoration(
          color: fontColor.withValues(alpha: 0.035),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: tagBorderColor.withValues(alpha: 0.48)),
        ),
        child: _buildPreviewFlashOverlay(
          isVisible: _isPreviewTokenFlashing('PreviewPropertyContentColor'),
          borderRadius: BorderRadius.circular(3),
          child: Text(
            label,
            overflow: TextOverflow.ellipsis,
            maxLines: 1,
            style: TextStyle(color: contentColor.withValues(alpha: 0.9), fontSize: metrics.smallLabelFontSize, height: 1.2, fontWeight: FontWeight.w600),
          ),
        ),
      ),
    );
  }

  Widget _buildPreviewActionPanel(WoxTheme theme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final actions = [
      _buildPreviewActionItem(id: 'copy', icon: _previewCopyIcon(), title: _tr('ui_theme_editor_action_copy')),
      _buildPreviewActionItem(id: 'open', icon: _previewOpenIcon(), title: _tr('ui_theme_editor_action_open')),
    ];

    return _buildPreviewFlashOverlay(
      isVisible: _isPreviewTokenFlashing('ActionContainerBackgroundColor'),
      borderRadius: BorderRadius.circular(theme.actionQueryBoxBorderRadius.toDouble()),
      child: Container(
        padding: EdgeInsets.fromLTRB(
          metrics.scaledSpacing(theme.actionContainerPaddingLeft.toDouble()),
          metrics.scaledSpacing(theme.actionContainerPaddingTop.toDouble()),
          metrics.scaledSpacing(theme.actionContainerPaddingRight.toDouble()),
          metrics.scaledSpacing(theme.actionContainerPaddingBottom.toDouble()),
        ),
        decoration: BoxDecoration(
          color: safeFromCssColor(theme.actionContainerBackgroundColor),
          borderRadius: BorderRadius.circular(theme.actionQueryBoxBorderRadius.toDouble()),
          boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.1), spreadRadius: 2, blurRadius: 8, offset: const Offset(0, 3))],
        ),
        child: Container(
          constraints: const BoxConstraints(minWidth: 180),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildPreviewFlashOverlay(
                isVisible: _isPreviewTokenFlashing('ActionContainerHeaderFontColor'),
                child: Text(_tr('ui_actions'), style: TextStyle(color: safeFromCssColor(theme.actionContainerHeaderFontColor), fontSize: metrics.actionHeaderFontSize)),
              ),
              Divider(color: safeFromCssColor(theme.previewSplitLineColor)),
              for (final entry in actions.asMap().entries)
                SizedBox(height: WoxThemeUtil.instance.getActionItemHeight(), child: _buildPreviewActionRow(theme, entry.value, isActive: entry.key == 0)),
              const SizedBox(height: 8),
              _buildPreviewActionQueryBox(theme),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildPreviewActionRow(WoxTheme theme, WoxListItem<WoxResultAction> item, {required bool isActive}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final itemHeight = WoxThemeUtil.instance.getActionItemHeight();
    final textLeft = metrics.resultItemIconPaddingLeft + metrics.actionIconSize + metrics.resultItemIconPaddingRight;
    final textTop = math.max(0.0, (itemHeight - metrics.actionTitleFontSize - 6) / 2);
    final textWidth = math.max(80.0, 150.0 * metrics.scale);
    final rowRadius = BorderRadius.circular(theme.resultItemBorderRadius.toDouble());

    return Stack(
      fit: StackFit.expand,
      children: [
        WoxListItemView(item: item, woxTheme: theme, isActive: isActive, isHovered: false, listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code),
        _buildPreviewFlashMarker(isVisible: isActive && _isPreviewTokenFlashing('ActionItemActiveBackgroundColor'), borderRadius: rowRadius),
        _buildPreviewFlashMarker(
          isVisible: isActive && _isPreviewTokenFlashing('ActionItemActiveFontColor'),
          left: textLeft,
          top: textTop,
          width: textWidth,
          height: metrics.actionTitleFontSize + 6,
        ),
        _buildPreviewFlashMarker(
          isVisible: !isActive && _isPreviewTokenFlashing('ActionItemFontColor'),
          left: textLeft,
          top: textTop,
          width: textWidth,
          height: metrics.actionTitleFontSize + 6,
        ),
      ],
    );
  }

  Widget _buildPreviewActionQueryBox(WoxTheme theme) {
    return _buildPreviewFlashOverlay(
      isVisible: _isPreviewTokenFlashing('ActionQueryBoxBackgroundColor'),
      borderRadius: BorderRadius.circular(theme.actionQueryBoxBorderRadius.toDouble()),
      child: Container(
        height: 30,
        padding: const EdgeInsets.symmetric(horizontal: 9),
        decoration: BoxDecoration(color: safeFromCssColor(theme.actionQueryBoxBackgroundColor), borderRadius: BorderRadius.circular(theme.actionQueryBoxBorderRadius.toDouble())),
        child: Row(
          children: [
            Icon(Icons.search, size: 13, color: safeFromCssColor(theme.actionItemFontColor).withValues(alpha: 0.68)),
            const SizedBox(width: 6),
            Expanded(
              child: Text(
                _tr('ui_theme_editor_preview_result_actions'),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: safeFromCssColor(theme.actionItemFontColor).withValues(alpha: 0.78), fontSize: 12),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPreviewToolbar(WoxTheme theme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final toolbarFontFlash = _isPreviewTokenFlashing('ToolbarFontColor');

    return _buildPreviewFlashOverlay(
      isVisible: _isPreviewTokenFlashing('ToolbarBackgroundColor'),
      borderRadius: BorderRadius.zero,
      child: Container(
        height: WoxThemeUtil.instance.getToolbarHeight(),
        padding: EdgeInsets.only(left: metrics.scaledSpacing(theme.toolbarPaddingLeft.toDouble()), right: metrics.scaledSpacing(theme.toolbarPaddingRight.toDouble())),
        decoration: BoxDecoration(
          color: safeFromCssColor(theme.toolbarBackgroundColor),
          border: Border(top: BorderSide(color: safeFromCssColor(theme.toolbarFontColor).withValues(alpha: 0.1), width: 1)),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            _buildPreviewToolbarAction(theme, _tr('ui_theme_editor_toolbar_copy'), ['Enter'], isFlashing: toolbarFontFlash),
            SizedBox(width: metrics.toolbarActionSpacing),
            _buildPreviewToolbarAction(theme, _tr('ui_theme_editor_toolbar_more'), ['Cmd', 'J'], isFlashing: toolbarFontFlash),
          ],
        ),
      ),
    );
  }

  Widget _buildPreviewToolbarAction(WoxTheme theme, String label, List<String> keys, {required bool isFlashing}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final color = safeFromCssColor(theme.toolbarFontColor);

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildPreviewFlashOverlay(
          isVisible: isFlashing,
          borderRadius: BorderRadius.circular(4),
          child: Text(label, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: color, fontSize: metrics.toolbarFontSize)),
        ),
        SizedBox(width: metrics.scaledSpacing(8)),
        for (final entry in keys.asMap().entries) ...[
          if (entry.key > 0) SizedBox(width: metrics.toolbarHotkeyKeySpacing),
          _buildPreviewToolbarKey(theme, entry.value, isFlashing: isFlashing),
        ],
      ],
    );
  }

  Widget _buildPreviewToolbarKey(WoxTheme theme, String key, {required bool isFlashing}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final color = safeFromCssColor(theme.toolbarFontColor);
    final minWidth = metrics.scaledSpacing(28);

    return _buildPreviewFlashOverlay(
      isVisible: isFlashing,
      borderRadius: BorderRadius.circular(4),
      child: Container(
        constraints: BoxConstraints(minWidth: minWidth),
        height: metrics.scaledSpacing(22),
        padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(7)),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.025),
          border: Border.all(color: color.withValues(alpha: 0.72)),
          borderRadius: BorderRadius.circular(4),
          boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.1), blurRadius: 2, offset: const Offset(0, 1))],
        ),
        child: Center(child: Text(key, style: TextStyle(fontFamily: 'SFProDisplay', color: color, fontSize: metrics.tailHotkeyFontSize, fontWeight: FontWeight.w500))),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Obx(
      () => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [Expanded(child: _buildLivePreview()), SizedBox(height: _controlPaneHeight, child: _buildControlPane())],
      ),
    );
  }
}

class _ThemeColorGroup {
  final String labelKey;
  final List<_ThemeColorToken> tokens;

  const _ThemeColorGroup({required this.labelKey, required this.tokens});
}

class _ThemeColorToken {
  final String key;
  final String labelKey;

  const _ThemeColorToken({required this.key, required this.labelKey});
}

class _ColorWheelPicker extends StatelessWidget {
  final HSVColor hsvColor;
  final ValueChanged<HSVColor> onChanged;

  const _ColorWheelPicker({required this.hsvColor, required this.onChanged});

  void _handlePosition(Offset localPosition, Size size) {
    final center = Offset(size.width / 2, size.height / 2);
    final offset = localPosition - center;
    final radius = math.min(size.width, size.height) / 2;
    final distance = math.min(offset.distance, radius);
    final saturation = (distance / radius).clamp(0.0, 1.0).toDouble();
    var hue = math.atan2(offset.dy, offset.dx) * 180 / math.pi;
    if (hue < 0) {
      hue += 360;
    }
    onChanged(hsvColor.withHue(hue).withSaturation(saturation));
  }

  @override
  Widget build(BuildContext context) {
    const wheelSize = 220.0;
    return SizedBox(
      width: wheelSize,
      height: wheelSize,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final size = Size(constraints.maxWidth, constraints.maxHeight);
          return GestureDetector(
            behavior: HitTestBehavior.opaque,
            onPanDown: (details) => _handlePosition(details.localPosition, size),
            onPanUpdate: (details) => _handlePosition(details.localPosition, size),
            child: CustomPaint(painter: _ColorWheelPainter(hsvColor)),
          );
        },
      ),
    );
  }
}

class _ColorWheelPainter extends CustomPainter {
  final HSVColor hsvColor;

  const _ColorWheelPainter(this.hsvColor);

  @override
  void paint(Canvas canvas, Size size) {
    final center = Offset(size.width / 2, size.height / 2);
    final radius = math.min(size.width, size.height) / 2;
    final rect = Rect.fromCircle(center: center, radius: radius);
    final clipPath = Path()..addOval(rect);

    canvas.save();
    canvas.clipPath(clipPath);
    canvas.drawCircle(
      center,
      radius,
      Paint()
        ..shader = const SweepGradient(
          colors: [Color(0xFFFF0000), Color(0xFFFFFF00), Color(0xFF00FF00), Color(0xFF00FFFF), Color(0xFF0000FF), Color(0xFFFF00FF), Color(0xFFFF0000)],
        ).createShader(rect),
    );
    canvas.drawCircle(center, radius, Paint()..shader = RadialGradient(colors: [Colors.white, Colors.white.withValues(alpha: 0)]).createShader(rect));
    canvas.restore();

    canvas.drawCircle(
      center,
      radius,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.5
        ..color = Colors.white.withValues(alpha: 0.38),
    );

    final angle = hsvColor.hue * math.pi / 180;
    final thumbOffset = Offset(math.cos(angle), math.sin(angle)) * (hsvColor.saturation * radius);
    final thumbCenter = center + thumbOffset;
    canvas.drawCircle(thumbCenter, 7, Paint()..color = hsvColor.toColor());
    canvas.drawCircle(
      thumbCenter,
      8,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2
        ..color = Colors.white,
    );
    canvas.drawCircle(
      thumbCenter,
      10,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.5
        ..color = Colors.black.withValues(alpha: 0.5),
    );
  }

  @override
  bool shouldRepaint(covariant _ColorWheelPainter oldDelegate) {
    return oldDelegate.hsvColor != hsvColor;
  }
}
