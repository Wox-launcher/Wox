part of 'wox_demo.dart';

class WoxPluginStoreDemo extends StatefulWidget {
  const WoxPluginStoreDemo({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  State<WoxPluginStoreDemo> createState() => _WoxPluginStoreDemoState();
}

class _WoxPluginStoreDemoState extends State<WoxPluginStoreDemo> with SingleTickerProviderStateMixin {
  static const List<_PluginStoreDemoIcon> _storeIcons = [
    _PluginStoreDemoIcon(
      name: 'Quick Links',
      iconUrl: 'https://raw.githubusercontent.com/Bonemind/Wox.Plugin.QuickLinks/main/images/app.png',
      fallbackIcon: Icons.link_outlined,
      color: Color(0xFF4F6EF7),
    ),
    _PluginStoreDemoIcon(
      name: 'RImage',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Rimage/main/images/app.svg',
      fallbackIcon: Icons.image_outlined,
      color: Color(0xFF3B82F6),
      featured: true,
    ),
    _PluginStoreDemoIcon(
      name: 'RSS Reader',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.RSSReader/main/images/app.png',
      fallbackIcon: Icons.rss_feed_outlined,
      color: Color(0xFFF97316),
    ),
    _PluginStoreDemoIcon(
      name: 'Spotify',
      iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.Spotify/main/images/app.png',
      fallbackIcon: Icons.music_note_outlined,
      color: Color(0xFF22C55E),
    ),
    _PluginStoreDemoIcon(
      name: 'Strava',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Strava/refs/heads/main/image/app.png',
      fallbackIcon: Icons.directions_bike_outlined,
      color: Color(0xFFFC4C02),
    ),
    _PluginStoreDemoIcon(
      name: 'Sum Selection Numbers',
      iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.Selection.Sum/main/images/app.png',
      fallbackIcon: Icons.functions_outlined,
      color: Color(0xFFFACC15),
    ),
    _PluginStoreDemoIcon(
      name: 'DeepL translator',
      iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.DeepL/main/images/app.png',
      fallbackIcon: Icons.translate_outlined,
      color: Color(0xFF0EA5E9),
    ),
    _PluginStoreDemoIcon(
      name: 'Iconify',
      iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.Iconify/master/image/app.png',
      fallbackIcon: Icons.interests_outlined,
      color: Color(0xFFA855F7),
    ),
    _PluginStoreDemoIcon(
      name: 'LocalSend',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.LocalSend/refs/heads/main/images/app.png',
      fallbackIcon: Icons.send_to_mobile_outlined,
      color: Color(0xFF06B6D4),
    ),
    _PluginStoreDemoIcon(
      name: 'Memos',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Memos/refs/heads/main/images/app.png',
      fallbackIcon: Icons.sticky_note_2_outlined,
      color: Color(0xFFF59E0B),
    ),
    _PluginStoreDemoIcon(
      name: 'Color Picker',
      iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.ColorPicker/refs/heads/main/images/app.png',
      fallbackIcon: Icons.color_lens_outlined,
      color: Color(0xFFF43F5E),
    ),
    _PluginStoreDemoIcon(
      name: 'Projects',
      iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.Projects/main/images/app.svg',
      fallbackIcon: Icons.folder_special_outlined,
      color: Color(0xFF3B82F6),
    ),
    _PluginStoreDemoIcon(
      name: 'Awake',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Awake/main/images/app.svg',
      fallbackIcon: Icons.coffee_outlined,
      color: Color(0xFFEAB308),
    ),
    _PluginStoreDemoIcon(
      name: 'Everything',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Everything/main/images/app.png',
      fallbackIcon: Icons.search_outlined,
      color: Color(0xFF22C55E),
    ),
    _PluginStoreDemoIcon(
      name: 'Unsplash',
      iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Unsplash/main/images/app.png',
      fallbackIcon: Icons.wallpaper_outlined,
      color: Color(0xFF64748B),
    ),
    _PluginStoreDemoIcon(
      name: 'LuxTranslate',
      iconUrl: 'https://raw.githubusercontent.com/stmluyuer/Wox.Plugin.LuxTranslate/main/images/app.svg',
      fallbackIcon: Icons.language_outlined,
      color: Color(0xFF06B6D4),
    ),
  ];

  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    // Feature change: plugin store owns a separate timeline from theme store
    // because it teaches discovery first, then Wox search/install. A dedicated
    // controller keeps that sequence readable without mode flags in theme code.
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 6200))..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  double _interval(double start, double end, Curve curve) {
    final value = ((_controller.value - start) / (end - start)).clamp(0.0, 1.0).toDouble();
    return curve.transform(value);
  }

  double _iconProgress() => _interval(0.08, 0.40, Curves.easeInOutCubic);

  double _iconOpacity() {
    if (_controller.value < 0.08) {
      return _interval(0, 0.08, Curves.easeOutCubic);
    }
    if (_controller.value < 0.42) {
      return 1;
    }
    return 1 - _interval(0.42, 0.55, Curves.easeInCubic);
  }

  double _windowRevealProgress() => _interval(0.34, 0.50, Curves.easeOutCubic);

  String _queryText() {
    // Feature change: the plugin-store demo now mirrors the real install page:
    // "wpm install" opens the store browser, then the selected plugin detail
    // appears beside the list instead of reducing the flow to a plain result row.
    const target = 'wpm install';
    // Keep typing speed aligned with the theme-install section. The previous
    // 0.50-0.75 window made plugin typing noticeably slower per character.
    final t = _interval(0.50, 0.63, Curves.linear);
    return target.substring(0, (t * target.length).floor().clamp(0, target.length));
  }

  String _primaryTail() {
    if (_controller.value >= 0.82 && _controller.value < 0.90) {
      return widget.tr('plugin_wpm_installing');
    }
    if (_controller.value >= 0.90 && _controller.value < 0.97) {
      return widget.tr('plugin_wpm_start_using');
    }
    return widget.tr('plugin_wpm_install');
  }

  Widget _buildDemoWindow() {
    return _PluginStoreInstallWindow(accent: widget.accent, query: _queryText(), installLabel: _primaryTail(), tr: widget.tr);
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      key: const ValueKey('onboarding-wpm-install-demo'),
      animation: _controller,
      builder: (context, child) {
        final windowReveal = _windowRevealProgress();

        return ClipRRect(
          borderRadius: BorderRadius.circular(8),
          child: Stack(
            children: [
              Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: Platform.isMacOS, showDefaultIcons: false)),
              Positioned.fill(
                child: Padding(
                  padding: _demoDesktopHintContentPadding(),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      WoxDemoHintCard(
                        accent: widget.accent,
                        icon: Icons.extension_outlined,
                        title: widget.tr('onboarding_wpm_install_title'),
                        from: 'wpm install',
                        to: widget.tr('onboarding_wpm_install_hint_target'),
                      ),
                      const SizedBox(height: 12),
                      Expanded(
                        child: Stack(
                          children: [
                            Positioned.fill(
                              child: IgnorePointer(
                                // Feature change: the plugin store starts as a
                                // real icon grid so users see breadth first.
                                // The grid then converges toward Wox, making the
                                // later search feel like it is pulling from the
                                // store rather than appearing from nowhere.
                                child: Opacity(opacity: _iconOpacity(), child: _PluginStoreIconGrid(accent: widget.accent, icons: _storeIcons, progress: _iconProgress())),
                              ),
                            ),
                            Positioned.fill(
                              child: Opacity(
                                opacity: windowReveal,
                                child: Transform.translate(
                                  offset: Offset(0, 18 * (1 - windowReveal)),
                                  child: Transform.scale(scale: 0.96 + (0.04 * windowReveal), child: _buildDemoWindow()),
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _PluginStoreDemoIcon {
  const _PluginStoreDemoIcon({required this.name, this.iconUrl = '', required this.fallbackIcon, required this.color, this.featured = false});

  final String name;
  final String iconUrl;
  final IconData fallbackIcon;
  final Color color;
  final bool featured;
}

class _PluginStoreIconGrid extends StatelessWidget {
  const _PluginStoreIconGrid({required this.accent, required this.icons, required this.progress});

  final Color accent;
  final List<_PluginStoreDemoIcon> icons;
  final double progress;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth.isFinite ? constraints.maxWidth : 720.0;
        final height = constraints.maxHeight.isFinite ? constraints.maxHeight : 360.0;
        const columns = 4;
        const rows = 4;
        final visibleIcons = icons.take(columns * rows).toList(growable: false);
        final gapX = (width / (columns + 1)).clamp(60.0, 92.0).toDouble();
        final gapY = (height / (rows + 1)).clamp(52.0, 76.0).toDouble();
        final gridWidth = (columns - 1) * gapX;
        final gridHeight = (rows - 1) * gapY;
        final target = Offset(width / 2, height / 2 - 4);
        final children = <Widget>[];

        for (var index = 0; index < visibleIcons.length; index++) {
          final icon = visibleIcons[index];
          final row = index ~/ columns;
          final column = index % columns;
          final initial = Offset((width / 2) - (gridWidth / 2) + (column * gapX), (height / 2) - (gridHeight / 2) + (row * gapY));
          final stagger = ((row * 0.025) + (column * 0.012)).clamp(0.0, 0.18).toDouble();
          final localProgress = ((progress - stagger) / (1 - stagger)).clamp(0.0, 1.0).toDouble();
          final easedProgress = Curves.easeInOutCubic.transform(localProgress);
          final position = Offset.lerp(initial, target, easedProgress)!;
          final baseSize = icon.featured ? 58.0 : 50.0;
          final gatherScale = icon.featured ? 0.50 : 0.28;
          final scale = 1 - ((1 - gatherScale) * easedProgress);
          final rotation = math.sin(index * 1.7) * 0.22 * easedProgress;
          final tileOpacity = (0.38 + (0.62 * (1 - easedProgress))).clamp(0.0, 1.0).toDouble();

          children.add(
            Positioned(
              left: position.dx - (baseSize / 2),
              top: position.dy - (baseSize / 2),
              width: baseSize,
              height: baseSize,
              child: Opacity(
                opacity: tileOpacity,
                child: Transform.rotate(angle: rotation, child: Transform.scale(scale: scale, child: _PluginStoreIconTile(icon: icon, accent: accent))),
              ),
            ),
          );
        }

        return Stack(
          children: [
            Positioned(
              left: target.dx - 24,
              top: target.dy - 24,
              width: 48,
              height: 48,
              child: Opacity(
                opacity: _centerMarkOpacity(progress),
                child: Transform.scale(
                  scale: 0.82 + (0.18 * progress),
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      color: accent.withValues(alpha: 0.16),
                      border: Border.all(color: accent.withValues(alpha: 0.42)),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: const Center(child: WoxDemoLogoMark()),
                  ),
                ),
              ),
            ),
            ...children,
          ],
        );
      },
    );
  }

  double _centerMarkOpacity(double value) {
    if (value < 0.35) return 0;
    if (value < 0.62) {
      return Curves.easeOutCubic.transform(((value - 0.35) / 0.27).clamp(0.0, 1.0).toDouble());
    }
    return 1 - Curves.easeInCubic.transform(((value - 0.62) / 0.38).clamp(0.0, 1.0).toDouble());
  }
}

class _PluginStoreIconTile extends StatelessWidget {
  const _PluginStoreIconTile({required this.icon, required this.accent});

  final _PluginStoreDemoIcon icon;
  final Color accent;

  @override
  Widget build(BuildContext context) {
    final borderColor = icon.featured ? accent.withValues(alpha: 0.68) : getThemeTextColor().withValues(alpha: 0.12);
    return Semantics(
      label: icon.name,
      image: true,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: getThemeBackgroundColor().withValues(alpha: 0.88),
          border: Border.all(color: borderColor),
          borderRadius: BorderRadius.circular(12),
          boxShadow: [BoxShadow(color: icon.color.withValues(alpha: icon.featured ? 0.34 : 0.18), blurRadius: icon.featured ? 24 : 14, offset: const Offset(0, 8))],
        ),
        child: Padding(padding: const EdgeInsets.all(8), child: _PluginStoreIconBadge(iconUrl: icon.iconUrl, fallbackIcon: icon.fallbackIcon, fallbackColor: icon.color)),
      ),
    );
  }
}

class _PluginStoreIconBadge extends StatelessWidget {
  const _PluginStoreIconBadge({required this.iconUrl, required this.fallbackIcon, required this.fallbackColor, this.size});

  final String iconUrl;
  final IconData fallbackIcon;
  final Color fallbackColor;
  final double? size;

  @override
  Widget build(BuildContext context) {
    final resolvedSize = size ?? 28.0;
    return SizedBox(
      width: resolvedSize,
      height: resolvedSize,
      child: Stack(
        fit: StackFit.expand,
        children: [
          DecoratedBox(
            // Feature resilience: live store icon URLs can be slow or offline
            // during onboarding. The fallback underneath keeps every tile and
            // result row visually complete instead of rendering an empty slot.
            decoration: BoxDecoration(color: fallbackColor.withValues(alpha: 0.16), borderRadius: BorderRadius.circular(8)),
            child: Icon(fallbackIcon, color: fallbackColor, size: resolvedSize * 0.72),
          ),
          if (iconUrl.isNotEmpty)
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: WoxImageView(woxImage: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code, imageData: iconUrl), width: resolvedSize, height: resolvedSize),
            ),
        ],
      ),
    );
  }
}

class _PluginStoreInstallWindow extends StatelessWidget {
  const _PluginStoreInstallWindow({required this.accent, required this.query, required this.installLabel, required this.tr});

  final Color accent;
  final String query;
  final String installLabel;
  final String Function(String key) tr;

  static const _selectedPlugin = _PluginStoreDemoIcon(
    name: 'RImage',
    iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Rimage/main/images/app.svg',
    fallbackIcon: Icons.image_outlined,
    color: Color(0xFF3B82F6),
    featured: true,
  );

  static const _visibleResults = [
    _PluginStoreListItem(
      icon: _PluginStoreDemoIcon(
        name: 'Quick Links',
        iconUrl: 'https://raw.githubusercontent.com/Bonemind/Wox.Plugin.QuickLinks/main/images/app.png',
        fallbackIcon: Icons.link_outlined,
        color: Color(0xFF4F6EF7),
      ),
      title: 'Quick Links',
      subtitle: 'Quickly open named URLs in your browser',
      installed: false,
    ),
    _PluginStoreListItem(icon: _selectedPlugin, title: 'RImage', subtitle: '使用 rimage 压缩选中的图片', selected: true, installed: false),
    _PluginStoreListItem(
      icon: _PluginStoreDemoIcon(
        name: 'RSS Reader',
        iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.RSSReader/main/images/app.png',
        fallbackIcon: Icons.rss_feed_outlined,
        color: Color(0xFFF97316),
      ),
      title: 'RSS Reader',
      subtitle: 'Read RSS feeds',
    ),
    _PluginStoreListItem(
      icon: _PluginStoreDemoIcon(name: 'Recent Files', fallbackIcon: Icons.schedule_outlined, color: Color(0xFF94A3B8)),
      title: 'Recent Files',
      subtitle: 'List recently used files',
    ),
    _PluginStoreListItem(
      icon: _PluginStoreDemoIcon(
        name: 'Spotify',
        iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.Spotify/main/images/app.png',
        fallbackIcon: Icons.music_note_outlined,
        color: Color(0xFF22C55E),
      ),
      title: 'Spotify',
      subtitle: 'Spotify integration',
    ),
    _PluginStoreListItem(
      icon: _PluginStoreDemoIcon(
        name: 'Strava',
        iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Strava/refs/heads/main/image/app.png',
        fallbackIcon: Icons.directions_bike_outlined,
        color: Color(0xFFFC4C02),
      ),
      title: 'Strava',
      subtitle: '一个与 Strava 运动交互的插件',
    ),
    _PluginStoreListItem(
      icon: _PluginStoreDemoIcon(
        name: 'Sum Selection Numbers',
        iconUrl: 'https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.Selection.Sum/main/images/app.png',
        fallbackIcon: Icons.functions_outlined,
        color: Color(0xFFFACC15),
      ),
      title: 'Sum Selection Nu...',
      subtitle: 'Sum Selection Numbers',
    ),
    _PluginStoreListItem(
      icon: _PluginStoreDemoIcon(name: 'UUID Generator', fallbackIcon: Icons.tag_outlined, color: Color(0xFFA855F7)),
      title: 'UUID Generator',
      subtitle: 'Generate various UUIDs',
    ),
  ];

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final backgroundColor = getThemeBackgroundColor();

    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        // Feature fidelity: the real plugin store uses a taller split-view panel.
        // The old compact launcher-demo height hid too much of the result list,
        // so this demo allows more vertical room while still respecting the
        // onboarding media card's available constraints.
        final height = constraints.maxHeight.clamp(0.0, 600.0).toDouble();
        final listWidth = (width * 0.38).clamp(250.0, 340.0).toDouble();
        // Feature refinement: this is a scaled onboarding preview, not a full
        // launcher window. Cap the footer height so the store content keeps the
        // vertical room needed to show the result list and detail screenshot.
        final footerHeight = WoxThemeUtil.instance.getToolbarHeight().clamp(48.0, 56.0).toDouble();

        return Center(
          child: SizedBox(
            width: width,
            height: height,
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: backgroundColor.withValues(alpha: 0.94),
                border: Border.all(color: textColor.withValues(alpha: 0.12)),
                borderRadius: BorderRadius.circular(8),
                boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.28), blurRadius: 42, offset: const Offset(0, 18))],
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: Stack(
                  children: [
                    Positioned.fill(
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          gradient: RadialGradient(center: const Alignment(0.62, -0.68), radius: 1.12, colors: [accent.withValues(alpha: 0.10), Colors.transparent]),
                        ),
                      ),
                    ),
                    Positioned(left: 12, right: 12, top: 12, child: _PluginStoreSearchBar(query: query)),
                    Positioned(left: 0, top: 80, bottom: footerHeight, width: listWidth, child: _PluginStoreResultList(accent: accent, items: _visibleResults)),
                    Positioned(
                      left: listWidth + 14,
                      right: 18,
                      top: 84,
                      bottom: footerHeight + 12,
                      child: _PluginStoreDetailPanel(textColor: textColor, subTextColor: subTextColor),
                    ),
                    Positioned(left: 0, right: 0, bottom: 0, height: footerHeight, child: _PluginStoreFooter(installLabel: installLabel, tr: tr)),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _PluginStoreListItem {
  const _PluginStoreListItem({required this.icon, required this.title, required this.subtitle, this.selected = false, this.installed = true});

  final _PluginStoreDemoIcon icon;
  final String title;
  final String subtitle;
  final bool selected;
  final bool installed;
}

class _PluginStoreSearchBar extends StatelessWidget {
  const _PluginStoreSearchBar({required this.query});

  final String query;

  @override
  Widget build(BuildContext context) {
    final theme = WoxThemeUtil.instance.currentTheme.value;
    return Container(
      // Feature refinement: the preview uses compact typography so the store UI
      // can communicate the whole plugin-install flow inside the onboarding card.
      height: 52,
      padding: const EdgeInsets.symmetric(horizontal: 14),
      decoration: BoxDecoration(color: theme.queryBoxBackgroundColorParsed, borderRadius: BorderRadius.circular(theme.queryBoxBorderRadius.toDouble())),
      child: Row(
        children: [
          Expanded(
            child: Text(query, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: theme.queryBoxFontColorParsed, fontSize: 22, fontWeight: FontWeight.w500)),
          ),
          const _PluginManagerGlyph(),
        ],
      ),
    );
  }
}

class _PluginManagerGlyph extends StatelessWidget {
  const _PluginManagerGlyph();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 30,
      height: 30,
      decoration: BoxDecoration(color: const Color(0xFF9333EA), borderRadius: BorderRadius.circular(6)),
      child: const Icon(Icons.extension_rounded, color: Colors.white, size: 19),
    );
  }
}

class _PluginStoreResultList extends StatelessWidget {
  const _PluginStoreResultList({required this.accent, required this.items});

  final Color accent;
  final List<_PluginStoreListItem> items;

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(12, 0, 12, 10),
      physics: const NeverScrollableScrollPhysics(),
      itemCount: items.length,
      itemBuilder: (context, index) => _PluginStoreResultRow(accent: accent, item: items[index]),
    );
  }
}

class _PluginStoreResultRow extends StatelessWidget {
  const _PluginStoreResultRow({required this.accent, required this.item});

  final Color accent;
  final _PluginStoreListItem item;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    return Container(
      height: 48,
      margin: const EdgeInsets.only(bottom: 3),
      decoration: BoxDecoration(
        color: item.selected ? accent.withValues(alpha: 0.88) : Colors.transparent,
        border: item.selected ? Border(left: BorderSide(color: accent.withValues(alpha: 0.94), width: 5)) : null,
        borderRadius: BorderRadius.circular(item.selected ? 4 : 0),
      ),
      child: Row(
        children: [
          const SizedBox(width: 12),
          _PluginStoreIconBadge(iconUrl: item.icon.iconUrl, fallbackIcon: item.icon.fallbackIcon, fallbackColor: item.icon.color, size: 30),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  item.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: item.selected ? Colors.white : textColor, fontSize: 14, fontWeight: FontWeight.w700, height: 1.1),
                ),
                const SizedBox(height: 4),
                Text(
                  item.subtitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: item.selected ? Colors.white.withValues(alpha: 0.88) : subTextColor, fontSize: 10.5, fontWeight: FontWeight.w600),
                ),
              ],
            ),
          ),
          if (item.installed) ...[
            const SizedBox(width: 8),
            Container(
              width: 21,
              height: 21,
              decoration: const BoxDecoration(color: Color(0xFF22C55E), shape: BoxShape.circle),
              child: const Icon(Icons.check_rounded, color: Colors.white, size: 16),
            ),
            const SizedBox(width: 10),
          ],
        ],
      ),
    );
  }
}

class _PluginStoreDetailPanel extends StatelessWidget {
  const _PluginStoreDetailPanel({required this.textColor, required this.subTextColor});

  final Color textColor;
  final Color subTextColor;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: getThemeTextColor().withValues(alpha: 0.06),
        border: Border.all(color: textColor.withValues(alpha: 0.10)),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Padding(
        // Feature refinement: compact detail spacing keeps the real plugin
        // metadata visible while leaving enough area for the screenshot preview.
        padding: const EdgeInsets.all(18),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                const _PluginStoreIconBadge(
                  iconUrl: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Rimage/main/images/app.svg',
                  fallbackIcon: Icons.image_outlined,
                  fallbackColor: Color(0xFF3B82F6),
                  size: 44,
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('RImage', maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 18, fontWeight: FontWeight.w800, height: 1.1)),
                      const SizedBox(height: 7),
                      Text(
                        '使用 rimage 压缩选中的图片 · qianlifeng',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: subTextColor, fontSize: 12, fontWeight: FontWeight.w600),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 7,
              children: const [
                _PluginStoreChip(label: 'v0.0.1'),
                _PluginStoreChip(label: 'NodeJS', icon: Icons.polyline_outlined, iconColor: Color(0xFF84CC16)),
                _PluginStoreChip(label: 'GitHub ↗', icon: Icons.code_rounded, iconColor: Color(0xFF64748B)),
              ],
            ),
            const SizedBox(height: 14),
            Expanded(
              child: ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    DecoratedBox(decoration: BoxDecoration(color: Colors.black.withValues(alpha: 0.22))),
                    WoxImageView(
                      woxImage: WoxImage(
                        imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code,
                        imageData: 'https://raw.githubusercontent.com/qianlifeng/Wox.Plugin.Rimage/main/screenshot.png',
                      ),
                      fit: BoxFit.contain,
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _PluginStoreChip extends StatelessWidget {
  const _PluginStoreChip({required this.label, this.icon, this.iconColor});

  final String label;
  final IconData? icon;
  final Color? iconColor;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    return Container(
      height: 27,
      padding: const EdgeInsets.symmetric(horizontal: 10),
      decoration: BoxDecoration(color: textColor.withValues(alpha: 0.05), border: Border.all(color: textColor.withValues(alpha: 0.10)), borderRadius: BorderRadius.circular(8)),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[Icon(icon, size: 13, color: iconColor ?? textColor.withValues(alpha: 0.72)), const SizedBox(width: 6)],
          Text(label, style: TextStyle(color: textColor.withValues(alpha: 0.86), fontSize: 11.5, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }
}

class _PluginStoreFooter extends StatelessWidget {
  const _PluginStoreFooter({required this.installLabel, required this.tr});

  final String installLabel;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    return DecoratedBox(
      decoration: BoxDecoration(color: getThemeTextColor().withValues(alpha: 0.04), border: Border(top: BorderSide(color: textColor.withValues(alpha: 0.10)))),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          Text(installLabel, style: TextStyle(color: textColor, fontSize: 12.5, fontWeight: FontWeight.w700)),
          const SizedBox(width: 8),
          _PluginStoreKeycap(label: '↵'),
          const SizedBox(width: 18),
          Text(tr('toolbar_more_actions'), style: TextStyle(color: textColor, fontSize: 12.5, fontWeight: FontWeight.w700)),
          const SizedBox(width: 8),
          _PluginStoreKeycap(label: Platform.isMacOS ? '⌥' : 'Alt'),
          const SizedBox(width: 6),
          const _PluginStoreKeycap(label: 'J'),
          const SizedBox(width: 14),
        ],
      ),
    );
  }
}

class _PluginStoreKeycap extends StatelessWidget {
  const _PluginStoreKeycap({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    return ConstrainedBox(
      // Bug fix: Container does not expose a minWidth parameter. Keep the keycap
      // layout stable with BoxConstraints so short glyphs and longer labels use
      // the same sizing path without relying on invalid widget arguments.
      constraints: const BoxConstraints(minWidth: 28),
      child: Container(
        height: 26,
        alignment: Alignment.center,
        padding: const EdgeInsets.symmetric(horizontal: 8),
        decoration: BoxDecoration(border: Border.all(color: textColor.withValues(alpha: 0.64)), borderRadius: BorderRadius.circular(6)),
        child: Text(label, style: TextStyle(color: textColor, fontSize: 11.5, fontWeight: FontWeight.w700)),
      ),
    );
  }
}
