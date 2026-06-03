import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/onboarding/finish_onboarding.dart';
import 'package:wox/components/onboarding/glance_onboarding.dart';
import 'package:wox/components/onboarding/hotkey_onboarding.dart';
import 'package:wox/components/onboarding/permissions_onboarding.dart';
import 'package:wox/components/onboarding/plugin_store_onboarding.dart';
import 'package:wox/components/onboarding/query_hotkeys_onboarding.dart';
import 'package:wox/components/onboarding/theme_install_onboarding.dart';
import 'package:wox/components/onboarding/tray_queries_onboarding.dart';
import 'package:wox/components/onboarding/welcome_onboarding.dart';
import 'package:wox/components/onboarding/wox_onboarding_style.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window_ids.dart';

const double _onboardingSidebarWidth = 256;
const double _onboardingFooterHeight = 72;

class WoxOnboardingView extends StatefulWidget {
  const WoxOnboardingView({super.key});

  @override
  State<WoxOnboardingView> createState() => _WoxOnboardingViewState();
}

class _OnboardingStep {
  const _OnboardingStep({required this.id, required this.titleKey});

  final String id;
  final String titleKey;
}

class _WoxOnboardingViewState extends State<WoxOnboardingView> {
  final launcherController = Get.find<WoxLauncherController>();
  final settingController = Get.find<WoxSettingController>();
  final FocusNode onboardingFocusNode = FocusNode();
  int activeStepIndex = 0;
  bool isGlanceLoading = false;
  bool isGlanceLoadFailed = false;
  bool hasRequestedGlanceLoad = false;
  bool isPermissionLoading = false;
  bool? accessibilityPassed;
  late final Future<List<WoxLang>> availableLanguagesFuture = WoxApi.instance.getAllLanguages(const UuidV4().generate());

  late final List<_OnboardingStep> steps = [
    const _OnboardingStep(id: 'welcome', titleKey: 'onboarding_welcome_title'),
    // Permission setup is macOS-only because Windows and Linux do not need a
    // first-run system permission page for the Wox features introduced here.
    // Keeping the step out of the list also keeps numbering and progress honest.
    if (Platform.isMacOS) const _OnboardingStep(id: 'permissions', titleKey: 'onboarding_permissions_title'),
    const _OnboardingStep(id: 'mainHotkey', titleKey: 'onboarding_main_hotkey_title'),
    const _OnboardingStep(id: 'selectionHotkey', titleKey: 'onboarding_selection_hotkey_title'),
    // Feature change: width and density tuning made onboarding feel longer than
    // the core first-run flow. Removing that dedicated page keeps the tour
    // focused while the shared Wox preview still demonstrates the real layout.
    const _OnboardingStep(id: 'glance', titleKey: 'onboarding_glance_title'),
    // Feature change: action panel intro was merged into the welcome step demo
    // so the anatomy animation flows directly into the primary-modifier J reveal, giving
    // users the full search-to-action story without an extra navigation step.
    // Feature change: the previous Advanced Queries page bundled three
    // unrelated workflows. Splitting them into dedicated steps lets each query
    // feature get its own explanation and animated demo.
    // Query shortcuts removed from onboarding: the alias expansion concept adds
    // complexity without being critical for first-run setup. Users can discover
    // it naturally via Settings after they are comfortable with the launcher.
    const _OnboardingStep(id: 'queryHotkeys', titleKey: 'onboarding_query_hotkeys_title'),
    const _OnboardingStep(id: 'trayQueries', titleKey: 'onboarding_tray_queries_title'),
    // Feature change: plugin and theme installation are common first-run
    // workflows, so they are taught as standalone sections instead of being
    // hidden behind generic query examples.
    const _OnboardingStep(id: 'wpmInstall', titleKey: 'onboarding_wpm_install_title'),
    const _OnboardingStep(id: 'themeInstall', titleKey: 'onboarding_theme_install_title'),
    const _OnboardingStep(id: 'finish', titleKey: 'onboarding_finish_title'),
  ];

  String tr(String key) => settingController.tr(key);

  _OnboardingStep get activeStep => steps[activeStepIndex];

  bool get isLastStep => activeStepIndex == steps.length - 1;

  Color get activeAccent => _stepAccentColor(activeStep.id);

  WoxImage? _resolveGlanceIcon(MetadataGlance glance, GlanceItem? preview) {
    if (preview != null && preview.icon.imageData.isNotEmpty) {
      // Feature change: the runtime Glance API can return a state-specific icon
      // such as AC power instead of the static metadata glyph, so onboarding uses
      // that live icon first and only falls back when the API has no snapshot yet.
      return preview.icon;
    }

    final metadataIcon = WoxImage.parse(glance.icon);
    return metadataIcon?.imageData.isNotEmpty == true ? metadataIcon : null;
  }

  Color _stepAccentColor(String stepId) {
    // Feature refinement: accents are tied to feature identity instead of list
    // position. Removing the window/interface section should not shift the
    // colors users already saw for Glance, Action Panel, or later sections.
    return switch (stepId) {
      'welcome' => const Color(0xFF2DD4BF),
      'permissions' => const Color(0xFFF97316),
      'mainHotkey' => const Color(0xFFF97316),
      'selectionHotkey' => const Color(0xFF60A5FA),
      'glance' => const Color(0xFFFACC15),
      'actionPanel' => const Color(0xFF34D399),
      'queryHotkeys' => const Color(0xFFF43F5E),
      'trayQueries' => const Color(0xFF22C55E),
      'wpmInstall' => const Color(0xFF38BDF8),
      'themeInstall' => const Color(0xFFE879F9),
      'finish' => const Color(0xFF2DD4BF),
      _ => const Color(0xFF22C55E),
    };
  }

  @override
  void initState() {
    super.initState();
    // Kick off glance loading immediately so the data is ready (or still
    // loading in the background) by the time the user reaches that step.
    // Previously this only started when the glance step was entered, which
    // made every user wait for a network/plugin round-trip at that exact
    // moment instead of during the earlier setup steps.
    hasRequestedGlanceLoad = true;
    unawaited(_loadGlanceChoices());
    _handleStepEntered();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }

      // Claim a page-level focus target and make Escape a no-op here so users
      // leave onboarding only through the explicit Skip/Finish actions.
      onboardingFocusNode.requestFocus();
    });
  }

  @override
  void dispose() {
    onboardingFocusNode.dispose();
    super.dispose();
  }

  KeyEventResult _handleOnboardingKeyEvent(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  void _goToStep(int index) {
    if (index < 0 || index >= steps.length) {
      return;
    }

    setState(() {
      activeStepIndex = index;
    });
    _handleStepEntered();
  }

  void _handleStepEntered() {
    // Glance loading is now started eagerly in initState, so the guard here
    // is kept only as a safety fallback in case the step is somehow reached
    // without the eager load having been requested.
    if (activeStep.id == 'glance' && !hasRequestedGlanceLoad) {
      hasRequestedGlanceLoad = true;
      unawaited(_loadGlanceChoices());
    }
    if (activeStep.id == 'permissions' && Platform.isMacOS && accessibilityPassed == null && !isPermissionLoading) {
      unawaited(_loadPermissionStatus());
    }
  }

  Future<void> _loadPermissionStatus() async {
    setState(() {
      isPermissionLoading = true;
    });
    try {
      final results = await WoxApi.instance.doctorCheck(const UuidV4().generate());
      final accessibility = results.where((item) => item.type.toLowerCase() == 'accessibility').toList();
      if (!mounted) return;
      setState(() {
        accessibilityPassed = accessibility.isEmpty ? true : accessibility.first.passed;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        accessibilityPassed = null;
      });
    } finally {
      if (mounted) {
        setState(() {
          isPermissionLoading = false;
        });
      }
    }
  }

  Future<void> _loadGlanceChoices() async {
    setState(() {
      isGlanceLoading = true;
      isGlanceLoadFailed = false;
    });

    try {
      final traceId = const UuidV4().generate();
      // Plugin and Glance metadata can still be loading during first install.
      // The onboarding page explicitly waits here and renders loading/empty
      // states so users never see a blank selector.
      await settingController.reloadPlugins(traceId);
      settingController.settingGlancePreviewItems.clear();
      await settingController.refreshSettingGlancePreviews(traceId);
    } catch (_) {
      if (!mounted) return;
      setState(() {
        isGlanceLoadFailed = true;
      });
    } finally {
      if (mounted) {
        setState(() {
          isGlanceLoading = false;
        });
      }
    }
  }

  Future<void> _finish({required bool markFinished}) async {
    await launcherController.finishOnboarding(const UuidV4().generate(), markFinished: markFinished);
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final accent = activeAccent;
      return WoxPlatformFocus(
        focusNode: onboardingFocusNode,
        autofocus: true,
        onKeyEvent: _handleOnboardingKeyEvent,
        child: Material(
          // Onboarding uses InkWell, buttons, and dropdowns outside the normal
          // launcher/setting subtree. Providing a local Material ancestor prevents
          // Flutter's debug error surface and gives text the expected app style.
          key: const ValueKey('onboarding-view'),
          // Glass-dark fix: the backdrop painter already applies the theme app
          // background once. Painting the same translucent color here as well
          // made glass themes nearly opaque, hiding the native acrylic layer.
          color: Colors.transparent,
          // Use DefaultTextStyle.merge so that the fontFamily set by SystemChineseFont
          // in the MaterialApp theme is preserved. DefaultTextStyle would fully replace
          // the inherited style, losing the Chinese system font that the setting view
          // and other views correctly inherit from the Material widget chain.
          child: DefaultTextStyle.merge(
            style: TextStyle(color: getThemeTextColor(), fontSize: 13),
            child: Stack(
              children: [
                Positioned.fill(child: const _OnboardingBackdrop()),
                Column(
                  children: [
                    Expanded(child: Row(children: [_buildSidebar(accent), Expanded(child: _buildStepBody(accent))])),
                    _buildFooter(accent),
                  ],
                ),
                Positioned(top: 0, left: 0, right: 0, height: 36, child: WoxMultipleWindowDragMoveArea(windowId: WoxMultipleWindowIds.onboarding, child: const SizedBox.expand())),
              ],
            ),
          ),
        ),
      );
    });
  }

  Widget _buildSidebar(Color accent) {
    return Container(
      width: _onboardingSidebarWidth,
      padding: const EdgeInsets.fromLTRB(24, 30, 18, 22),
      decoration: BoxDecoration(
        // Glass-dark refresh: keep the rail as neutral chrome. The previous
        // frame leaned on per-step accent color, which fought the glass-dark
        // launcher surface and made the active step feel like a separate theme.
        color: WoxOnboardingGlassStyle.chromeSurface(0.28),
        border: Border(right: BorderSide(color: WoxOnboardingGlassStyle.outline(0.10))),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 28,
                height: 28,
                decoration: BoxDecoration(borderRadius: BorderRadius.circular(6), boxShadow: [BoxShadow(color: accent.withValues(alpha: 0.18), blurRadius: 14)]),
                clipBehavior: Clip.antiAlias,
                child: WoxImageView(woxImage: WoxImage.newBase64(WOX_ICON), width: 28, height: 28),
              ),
              const SizedBox(width: 11),
              // The onboarding rail now follows the settings page's text
              // hierarchy instead of using oversized promotional weights, so
              // the guide feels like part of the same management surface.
              Text(tr('onboarding_title'), style: TextStyle(color: getThemeTextColor(), fontSize: 22, fontWeight: FontWeight.w700)),
            ],
          ),
          const SizedBox(height: 10),
          Text(tr('onboarding_subtitle'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.45)),
          const SizedBox(height: 24),
          Expanded(
            child: ListView.builder(
              itemCount: steps.length,
              itemBuilder: (context, index) {
                final step = steps[index];
                final isActive = index == activeStepIndex;
                final isDone = index < activeStepIndex;
                final nodeColor =
                    isActive ? accent : (isDone ? WoxOnboardingGlassStyle.textColor.withValues(alpha: 0.86) : WoxOnboardingGlassStyle.subTextColor.withValues(alpha: 0.46));
                const rowHeight = 58.0;
                const nodeCenterY = rowHeight / 2;
                final nodeSize = isActive ? 24.0 : 18.0;
                final nodeRadius = nodeSize / 2;
                return InkWell(
                  key: ValueKey('onboarding-step-${step.id}'),
                  borderRadius: BorderRadius.circular(10),
                  hoverColor: WoxOnboardingGlassStyle.surface(0.05),
                  splashColor: Colors.transparent,
                  onTap: () => _goToStep(index),
                  child: SizedBox(
                    // A fixed row height lets the step node and label share the
                    // same visual center. The previous top-aligned Row made the
                    // active label drift below the numbered circle.
                    height: rowHeight,
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.center,
                      children: [
                        SizedBox(
                          width: 30,
                          height: rowHeight,
                          child: Stack(
                            clipBehavior: Clip.none,
                            children: [
                              if (index != 0)
                                Positioned(left: 14.5, top: 0, height: nodeCenterY - nodeRadius - 3, child: Container(width: 1, color: WoxOnboardingGlassStyle.mutedOutline(0.18))),
                              if (index != steps.length - 1)
                                Positioned(left: 14.5, top: nodeCenterY + nodeRadius + 3, bottom: 0, child: Container(width: 1, color: WoxOnboardingGlassStyle.mutedOutline(0.18))),
                              Positioned(
                                left: (30 - nodeSize) / 2,
                                top: nodeCenterY - nodeRadius,
                                child: AnimatedContainer(
                                  duration: const Duration(milliseconds: 220),
                                  width: nodeSize,
                                  height: nodeSize,
                                  alignment: Alignment.center,
                                  decoration: BoxDecoration(
                                    color: isActive ? nodeColor.withValues(alpha: 0.18) : nodeColor.withValues(alpha: 0.12),
                                    shape: BoxShape.circle,
                                    border: Border.all(color: nodeColor.withValues(alpha: isActive ? 0.82 : 0.38)),
                                    boxShadow: isActive ? [BoxShadow(color: nodeColor.withValues(alpha: 0.28), blurRadius: 18, spreadRadius: 1)] : const [],
                                  ),
                                  child:
                                      isDone
                                          ? Icon(Icons.check_rounded, size: 12, color: nodeColor)
                                          : Text('${index + 1}', style: TextStyle(color: nodeColor, fontSize: 10, fontWeight: FontWeight.w800)),
                                ),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(width: 10),
                        Expanded(
                          child: AnimatedContainer(
                            duration: const Duration(milliseconds: 220),
                            height: 38,
                            alignment: Alignment.centerLeft,
                            padding: const EdgeInsets.symmetric(horizontal: 10),
                            decoration: BoxDecoration(
                              // Glass-dark refresh: active rows use the
                              // launcher's neutral selected-result surface
                              // instead of tinting the whole row with the
                              // feature accent. The accent remains in the node
                              // so progress is still easy to scan.
                              color: isActive ? WoxOnboardingGlassStyle.activeSurface(0.12) : Colors.transparent,
                              border: Border.all(color: isActive ? WoxOnboardingGlassStyle.outline(0.18) : Colors.transparent),
                              borderRadius: BorderRadius.circular(8),
                            ),
                            child: Text(
                              tr(step.titleKey),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                color: isActive ? WoxOnboardingGlassStyle.textColor : WoxOnboardingGlassStyle.subTextColor,
                                fontSize: 13,
                                fontWeight: isActive ? FontWeight.w600 : FontWeight.normal,
                              ),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStepBody(Color accent) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(38, 30, 38, 20),
      child: LayoutBuilder(
        builder: (context, constraints) {
          const titleHeight = 44.0;
          const titleToSettingsGap = 16.0;
          final availableHeight = constraints.maxHeight;

          return AnimatedSwitcher(
            duration: const Duration(milliseconds: 280),
            switchInCurve: Curves.easeOutCubic,
            switchOutCurve: Curves.easeInCubic,
            transitionBuilder: (child, animation) {
              final offset = Tween<Offset>(begin: const Offset(0.025, 0), end: Offset.zero).animate(animation);
              return FadeTransition(opacity: animation, child: SlideTransition(position: offset, child: child));
            },
            child: SizedBox(
              key: ValueKey('onboarding-stage-${activeStep.id}'),
              height: availableHeight,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  SizedBox(
                    height: titleHeight,
                    child: Align(
                      alignment: Alignment.topLeft,
                      child: Text(
                        tr(activeStep.titleKey),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: getThemeTextColor(), fontSize: 32, fontWeight: FontWeight.w800, height: 1.05),
                      ),
                    ),
                  ),
                  const SizedBox(height: titleToSettingsGap),
                  // Architecture cleanup: the view owns only the common title
                  // and navigation frame. Each concrete step now lives under
                  // components/onboarding and decides which reusable demo it
                  // needs, keeping this file from becoming a catalog of demos.
                  Expanded(child: _buildActiveStep(accent)),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildActiveStep(Color accent) {
    switch (activeStep.id) {
      case 'permissions':
        return WoxPermissionsOnboarding(accent: accent, tr: tr, isPermissionLoading: isPermissionLoading, accessibilityPassed: accessibilityPassed);
      case 'mainHotkey':
        return WoxMainHotkeyOnboarding(
          accent: accent,
          hotkey: settingController.woxSetting.value.mainHotkey,
          tr: tr,
          onHotkeyChanged: (hotkey) {
            // Step extraction: persistence remains in the view/controller layer
            // so the onboarding component stays a focused UI widget.
            settingController.updateConfig('MainHotkey', hotkey);
          },
        );
      case 'selectionHotkey':
        return WoxSelectionHotkeyOnboarding(
          accent: accent,
          hotkey: settingController.woxSetting.value.selectionHotkey,
          tr: tr,
          onHotkeyChanged: (hotkey) {
            // Step extraction: the component reports the new value, while the
            // parent keeps the setting key and controller dependency here.
            settingController.updateConfig('SelectionHotkey', hotkey);
          },
        );
      case 'glance':
        // Bug fix: this step is built from a LayoutBuilder callback, which is
        // outside the outer Obx dependency tracking. Keep the reactive boundary
        // beside the setting reads so switch/dropdown changes repaint the active
        // Glance step without requiring users to navigate away and back.
        return Obx(() {
          final items = _buildGlanceDropdownItems();
          final currentRef = settingController.woxSetting.value.primaryGlance;
          final currentValue = items.any((item) => item.value == currentRef.key) ? currentRef.key : (items.isEmpty ? currentRef.key : items.first.value);
          return WoxGlanceOnboarding(
            accent: accent,
            tr: tr,
            enabled: settingController.woxSetting.value.enableGlance,
            isLoading: isGlanceLoading,
            isLoadFailed: isGlanceLoadFailed,
            items: items,
            currentValue: currentValue,
            label: _currentGlanceLabel(),
            value: _currentGlanceValue(),
            icon: _currentGlanceIcon(),
            onEnableChanged: (value) {
              // Step extraction: Glance setup is optional and persisted through
              // the same setting update path as the full settings page.
              settingController.updateConfig('EnableGlance', value.toString());
            },
            onPrimaryGlanceChanged: (encodedRef) {
              settingController.updateConfig('EnableGlance', 'true');
              settingController.updateConfig('PrimaryGlance', encodedRef);
            },
          );
        });
      case 'queryHotkeys':
        return WoxQueryHotkeysOnboarding(accent: accent, tr: tr);
      case 'trayQueries':
        return WoxTrayQueriesOnboarding(accent: accent, tr: tr);
      case 'wpmInstall':
        return WoxPluginStoreOnboarding(accent: accent, tr: tr);
      case 'themeInstall':
        return WoxThemeInstallOnboarding(accent: accent, tr: tr);
      case 'finish':
        return WoxFinishOnboarding(
          accent: accent,
          tr: tr,
          glanceEnabled: settingController.woxSetting.value.enableGlance,
          glanceLabel: _currentGlanceLabel(),
          glanceValue: _currentGlanceValue(),
          glanceIcon: _currentGlanceIcon(),
        );
      default:
        return WoxWelcomeOnboarding(
          accent: accent,
          tr: tr,
          languagesFuture: availableLanguagesFuture,
          currentLangCode: settingController.woxSetting.value.langCode,
          onLangChanged: settingController.updateLang,
        );
    }
  }

  List<WoxDropdownItem<String>> _buildGlanceDropdownItems() {
    final items = <WoxDropdownItem<String>>[];
    for (final plugin in settingController.installedPlugins) {
      for (final glance in plugin.glances) {
        final key = GlanceRef(pluginId: plugin.id, glanceId: glance.id).key;
        final preview = settingController.settingGlancePreviewItems[key];
        final icon = _resolveGlanceIcon(glance, preview);
        items.add(
          WoxDropdownItem(
            value: key,
            label: glance.name,
            subtitle: plugin.name,
            leading: icon == null ? null : WoxImageView(woxImage: icon, width: 18, height: 18, svgColor: getThemeTextColor()),
            trailing: Text(preview?.text ?? '', maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          ),
        );
      }
    }
    return items;
  }

  String _currentGlanceLabel() {
    final currentRef = settingController.woxSetting.value.primaryGlance;
    for (final plugin in settingController.installedPlugins) {
      for (final glance in plugin.glances) {
        if (GlanceRef(pluginId: plugin.id, glanceId: glance.id).key == currentRef.key) {
          return glance.name;
        }
      }
    }
    return tr('onboarding_glance_sample_time');
  }

  String _currentGlanceValue() {
    final currentRef = settingController.woxSetting.value.primaryGlance;
    final preview = settingController.settingGlancePreviewItems[currentRef.key]?.text;
    if (preview != null && preview.isNotEmpty) {
      return preview;
    }
    return tr('onboarding_glance_sample_value');
  }

  WoxImage? _currentGlanceIcon() {
    final currentRef = settingController.woxSetting.value.primaryGlance;
    final preview = settingController.settingGlancePreviewItems[currentRef.key];
    for (final plugin in settingController.installedPlugins) {
      for (final glance in plugin.glances) {
        if (GlanceRef(pluginId: plugin.id, glanceId: glance.id).key == currentRef.key) {
          return _resolveGlanceIcon(glance, preview);
        }
      }
    }
    return preview != null && preview.icon.imageData.isNotEmpty ? preview.icon : null;
  }

  Widget _buildFooter(Color accent) {
    return Container(
      height: _onboardingFooterHeight,
      padding: const EdgeInsets.symmetric(horizontal: 28),
      decoration: BoxDecoration(color: WoxOnboardingGlassStyle.chromeSurface(0.50), border: Border(top: BorderSide(color: WoxOnboardingGlassStyle.outline(0.10)))),
      child: Row(
        children: [
          WoxButton.text(key: const ValueKey('onboarding-skip-button'), text: tr('onboarding_skip'), onPressed: () => _finish(markFinished: true)),
          // Feature refinement: progress is represented by the step rail only.
          // Removing the footer progress bar keeps the action area focused on
          // navigation and avoids showing two competing progress indicators.
          const Spacer(),
          WoxButton.secondary(
            key: const ValueKey('onboarding-back-button'),
            text: tr('onboarding_back'),
            onPressed: activeStepIndex == 0 ? null : () => _goToStep(activeStepIndex - 1),
          ),
          const SizedBox(width: 12),
          WoxButton.primary(
            key: ValueKey(isLastStep ? 'onboarding-finish-button' : 'onboarding-next-button'),
            text: tr(isLastStep ? 'onboarding_finish' : 'onboarding_next'),
            onPressed: isLastStep ? () => _finish(markFinished: true) : () => _goToStep(activeStepIndex + 1),
          ),
        ],
      ),
    );
  }
}

class _OnboardingBackdrop extends StatelessWidget {
  const _OnboardingBackdrop();

  @override
  Widget build(BuildContext context) {
    // Keep onboarding backdrop intentionally flat so acrylic/window materials
    // do not introduce perceived texture over the walkthrough content.
    return ColoredBox(color: getThemeBackgroundColor());
  }
}
