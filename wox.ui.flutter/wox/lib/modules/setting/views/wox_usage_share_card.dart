import 'package:flutter/material.dart';
import 'package:wox/entity/wox_usage_stats.dart';

class WoxUsageShareCard extends StatelessWidget {
  const WoxUsageShareCard({super.key, required this.stats, required this.tr});

  final WoxUsageStats stats;
  // The share card is rendered off-screen, so it receives the settings translator explicitly to
  // keep labels aligned with the user's current Wox language during image generation.
  final String Function(String key) tr;

  static const double width = 540;
  static const double height = 610;

  @override
  Widget build(BuildContext context) {
    final highlights = _buildHighlights(stats, tr);

    return SizedBox(
      width: width,
      height: height,
      child: DecoratedBox(
        // The exported image uses square outer corners because X can render pasted transparent
        // rounded corners inconsistently; a full rectangle avoids edge artifacts.
        decoration: BoxDecoration(color: const Color(0xFF020607), border: Border.all(color: Colors.white.withValues(alpha: 0.16))),
        child: Stack(
          children: [
            const _ShareCardBackground(),
            Padding(
              padding: const EdgeInsets.all(28),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _brandRow(),
                  const SizedBox(height: 18),
                  _primaryMetricCard(),
                  const SizedBox(height: 13),
                  Row(
                    children: [
                      Expanded(child: _metricCard(value: _formatNumber(stats.totalActions), label: tr('ui_usage_share_card_actions'))),
                      const SizedBox(width: 12),
                      Expanded(child: _metricCard(value: _formatNumber(stats.totalAppsUsed), label: tr('ui_usage_share_card_apps'))),
                      const SizedBox(width: 12),
                      Expanded(child: _metricCard(value: _formatNumber(stats.usageDays), label: tr('ui_usage_share_card_usage_days'))),
                    ],
                  ),
                  const SizedBox(height: 13),
                  if (highlights.isNotEmpty) _highlightsCard(highlights),
                  const SizedBox(height: 18),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _brandRow() {
    return Row(
      children: [
        Container(
          width: 34,
          height: 34,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(10),
            color: const Color(0xFF2FE6BD),
            boxShadow: [BoxShadow(color: const Color(0xFF2FE6BD).withValues(alpha: 0.55), blurRadius: 24)],
          ),
          alignment: Alignment.center,
          child: const Text('W', style: TextStyle(color: Color(0xFF031614), fontSize: 18, fontWeight: FontWeight.w900)),
        ),
        const SizedBox(width: 11),
        const Expanded(child: Text('Wox Launcher', style: TextStyle(color: Colors.white, fontSize: 19, fontWeight: FontWeight.w800, letterSpacing: 0))),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(999),
            border: Border.all(color: Colors.white.withValues(alpha: 0.14)),
            color: Colors.black.withValues(alpha: 0.24),
          ),
          child: Text(DateTime.now().year.toString(), style: TextStyle(color: Colors.white.withValues(alpha: 0.66), fontSize: 12, fontWeight: FontWeight.w800)),
        ),
      ],
    );
  }

  Widget _primaryMetricCard() {
    final peak = stats.mostActiveHour < 0 ? '-' : '${stats.mostActiveHour.toString().padLeft(2, '0')}:00';

    return Container(
      height: 132,
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: const Color(0xFF2FE6BD).withValues(alpha: 0.36)),
        gradient: LinearGradient(colors: [const Color(0xFF2FE6BD).withValues(alpha: 0.20), Colors.black.withValues(alpha: 0.42)]),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.18), blurRadius: 28)],
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.end,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _formatNumber(stats.totalOpened),
                  style: const TextStyle(color: Colors.white, fontSize: 64, height: 1, fontWeight: FontWeight.w900, letterSpacing: 0, fontFeatures: [FontFeature.tabularFigures()]),
                ),
                const SizedBox(height: 8),
                _metricLabel(tr('ui_usage_share_card_wox_opens')),
              ],
            ),
          ),
          Column(
            mainAxisAlignment: MainAxisAlignment.end,
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              _metricLabel(tr('ui_usage_share_card_peak')),
              const SizedBox(height: 8),
              Text(
                peak,
                style: const TextStyle(color: Colors.white, fontSize: 30, height: 1, fontWeight: FontWeight.w900, letterSpacing: 0, fontFeatures: [FontFeature.tabularFigures()]),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _metricCard({required String value, required String label}) {
    return Container(
      // The compact metric card is captured as a Flutter debug image, so its height needs enough
      // room for real font metrics instead of only the nominal text sizes; otherwise overflow stripes
      // become part of the generated share image.
      height: 94,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: Colors.white.withValues(alpha: 0.12)),
        color: Colors.black.withValues(alpha: 0.34),
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.end,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          FittedBox(
            fit: BoxFit.scaleDown,
            alignment: Alignment.centerLeft,
            child: Text(
              value,
              style: const TextStyle(color: Colors.white, fontSize: 30, height: 1, fontWeight: FontWeight.w900, letterSpacing: 0, fontFeatures: [FontFeature.tabularFigures()]),
              maxLines: 1,
            ),
          ),
          const SizedBox(height: 8),
          _metricLabel(label),
        ],
      ),
    );
  }

  Widget _highlightsCard(List<_UsageShareHighlight> highlights) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: Colors.white.withValues(alpha: 0.12)),
        color: const Color(0xFF02080A).withValues(alpha: 0.72),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.28), blurRadius: 36)],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(tr('ui_usage_share_card_highlights'), style: TextStyle(color: Colors.white.withValues(alpha: 0.66), fontSize: 12, fontWeight: FontWeight.w800, letterSpacing: 1.0)),
          const SizedBox(height: 7),
          ...highlights.map(_highlightRow),
        ],
      ),
    );
  }

  Widget _highlightRow(_UsageShareHighlight highlight) {
    return Container(
      height: 48,
      decoration: BoxDecoration(border: Border(top: BorderSide(color: Colors.white.withValues(alpha: 0.08)))),
      child: Row(
        children: [
          Container(
            width: 30,
            height: 30,
            decoration: BoxDecoration(borderRadius: BorderRadius.circular(9), color: const Color(0xFF2FE6BD).withValues(alpha: 0.14)),
            child: Icon(highlight.icon, size: 16, color: const Color(0xFF2FE6BD)),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(highlight.name, style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w800, letterSpacing: 0), overflow: TextOverflow.ellipsis),
                const SizedBox(height: 2),
                Text(highlight.label, style: TextStyle(color: Colors.white.withValues(alpha: 0.50), fontSize: 11, fontWeight: FontWeight.w500), overflow: TextOverflow.ellipsis),
              ],
            ),
          ),
          const SizedBox(width: 10),
          Text(
            _formatNumber(highlight.count),
            style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w900, fontFeatures: [FontFeature.tabularFigures()]),
          ),
        ],
      ),
    );
  }

  Widget _metricLabel(String label) {
    return Text(label.toUpperCase(), style: TextStyle(color: Colors.white.withValues(alpha: 0.60), fontSize: 12, fontWeight: FontWeight.w800, letterSpacing: 0.25));
  }

  static List<_UsageShareHighlight> _buildHighlights(WoxUsageStats stats, String Function(String key) tr) {
    final highlights = <_UsageShareHighlight>[];
    final usedPluginIds = <String>{};

    // Share highlights must reflect real usage rows only. Missing data is omitted instead of
    // filling the card with examples, which keeps public shares honest and privacy predictable.
    final topPlugin = _firstUsableItem(stats.topPlugins);
    if (topPlugin != null) {
      usedPluginIds.add(_itemKey(topPlugin));
      highlights.add(_UsageShareHighlight(name: _itemName(topPlugin), label: tr('ui_usage_share_card_top_plugin'), count: topPlugin.count, icon: Icons.keyboard_command_key));
    }

    final aiPlugin = _firstWhereOrNull(stats.topPlugins, (item) => !usedPluginIds.contains(_itemKey(item)) && _isUsableItem(item) && _looksLikeAI(item));
    final fallbackPlugin = _firstWhereOrNull(stats.topPlugins, (item) => !usedPluginIds.contains(_itemKey(item)) && _isUsableItem(item));
    final secondPlugin = aiPlugin ?? fallbackPlugin;
    if (secondPlugin != null) {
      usedPluginIds.add(_itemKey(secondPlugin));
      highlights.add(
        _UsageShareHighlight(
          name: _itemName(secondPlugin),
          label: aiPlugin == null ? tr('ui_usage_share_card_plugin') : tr('ui_usage_share_card_ai'),
          count: secondPlugin.count,
          icon: aiPlugin == null ? Icons.extension_outlined : Icons.auto_awesome,
        ),
      );
    }

    final topApp = _firstUsableItem(stats.topApps);
    if (topApp != null) {
      highlights.add(_UsageShareHighlight(name: _itemName(topApp), label: tr('ui_usage_share_card_top_app'), count: topApp.count, icon: Icons.search));
    }

    return highlights.take(3).toList();
  }

  static WoxUsageStatsItem? _firstUsableItem(List<WoxUsageStatsItem> items) {
    return _firstWhereOrNull(items, _isUsableItem);
  }

  static WoxUsageStatsItem? _firstWhereOrNull(List<WoxUsageStatsItem> items, bool Function(WoxUsageStatsItem item) test) {
    for (final item in items) {
      if (test(item)) {
        return item;
      }
    }
    return null;
  }

  static bool _isUsableItem(WoxUsageStatsItem item) {
    return _itemName(item).trim().isNotEmpty;
  }

  static String _itemName(WoxUsageStatsItem item) {
    return item.name.trim().isNotEmpty ? item.name.trim() : item.id.trim();
  }

  static String _itemKey(WoxUsageStatsItem item) {
    return '${item.id.trim()}|${_itemName(item)}';
  }

  static bool _looksLikeAI(WoxUsageStatsItem item) {
    final value = '${item.name} ${item.id}'.toLowerCase();
    return value.contains('ai') || value.contains('gpt') || value.contains('openai') || value.contains('claude') || value.contains('gemini');
  }

  static String _formatNumber(int value) {
    final chars = value.toString().split('');
    final parts = <String>[];
    for (var i = chars.length; i > 0; i -= 3) {
      parts.insert(0, chars.sublist(i - 3 < 0 ? 0 : i - 3, i).join());
    }
    return parts.join(',');
  }
}

class _ShareCardBackground extends StatelessWidget {
  const _ShareCardBackground();

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        Positioned.fill(
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [const Color(0xFF2FE6BD).withValues(alpha: 0.30), const Color(0xFF0A1118), const Color(0xFF06221D)],
                stops: const [0, 0.55, 1],
              ),
            ),
          ),
        ),
        Positioned(
          left: -80,
          top: -90,
          width: 260,
          height: 260,
          child: DecoratedBox(decoration: BoxDecoration(shape: BoxShape.circle, color: const Color(0xFF2FE6BD).withValues(alpha: 0.16))),
        ),
        Positioned(
          left: 30,
          right: 30,
          bottom: 18,
          height: 120,
          child: DecoratedBox(
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(999),
              gradient: RadialGradient(colors: [const Color(0xFF2FE6BD).withValues(alpha: 0.18), Colors.transparent]),
            ),
          ),
        ),
      ],
    );
  }
}

class _UsageShareHighlight {
  const _UsageShareHighlight({required this.name, required this.label, required this.count, required this.icon});

  final String name;
  final String label;
  final int count;
  final IconData icon;
}
