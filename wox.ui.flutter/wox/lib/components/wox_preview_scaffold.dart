import 'package:flutter/material.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxPreviewScaffold extends StatelessWidget {
  final WoxTheme woxTheme;
  final Widget child;
  final List<WoxPreviewTag> tags;
  final ScrollController scrollController;
  final bool contentHandlesScrolling;

  const WoxPreviewScaffold({super.key, required this.woxTheme, required this.child, required this.tags, required this.scrollController, this.contentHandlesScrolling = false});

  @override
  Widget build(BuildContext context) {
    // The preview shell keeps every plugin preview content-first. It now owns the
    // framed scroll surface as well as the metadata row because leaving the frame
    // inside individual preview renderers put scrollbars outside the background
    // and made each preview type drift visually. The top area is always the
    // preview body, while optional metadata stays in lightweight pills below it.
    return Container(
      padding: EdgeInsets.only(
        top: WoxInterfaceSizeUtil.instance.current.scaledSpacing(12),
        bottom: WoxInterfaceSizeUtil.instance.current.scaledSpacing(10),
        left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(14),
        right: WoxInterfaceSizeUtil.instance.current.scaledSpacing(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(child: _buildContentSurface(context)),
          if (tags.isNotEmpty) ...[
            // The framed preview body already separates content from metadata,
            // so an extra divider above the pills would add visual noise without
            // improving scanability.
            Padding(padding: EdgeInsets.only(top: WoxInterfaceSizeUtil.instance.current.scaledSpacing(10)), child: WoxPreviewTagPills(woxTheme: woxTheme, tags: tags)),
          ],
        ],
      ),
    );
  }

  Widget _buildContentSurface(BuildContext context) {
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);
    final splitLineColor = safeFromCssColor(woxTheme.previewSplitLineColor);

    // The frame lives outside the scroller so the scrollbar track is clipped
    // inside the same rounded background instead of floating on the launcher
    // panel. Content-specific renderers only provide layout and typography.
    return Container(
      decoration: BoxDecoration(
        color: fontColor.withValues(alpha: 0.035),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: splitLineColor.withValues(alpha: 0.45)),
      ),
      child: ClipRRect(borderRadius: BorderRadius.circular(8), child: _buildContent(context)),
    );
  }

  Widget _buildContent(BuildContext context) {
    final selectionTheme = TextSelectionThemeData(selectionColor: safeFromCssColor(woxTheme.previewTextSelectionColor));
    final themedContent = Theme(data: Theme.of(context).copyWith(textSelectionTheme: selectionTheme), child: child);

    if (contentHandlesScrolling) {
      return LayoutBuilder(builder: (context, constraints) => SizedBox(width: constraints.maxWidth, height: constraints.maxHeight, child: themedContent));
    }

    // The shell owns scrolling for simple preview types so text, markdown, and
    // unsupported-file messages share the same scrollbar placement inside the
    // framed preview background.
    // The viewport height is passed as a minimum child height so short preview
    // renderers can opt into vertical centering without reading an unbounded
    // height from SingleChildScrollView.
    return LayoutBuilder(
      builder:
          (context, viewportConstraints) => Scrollbar(
            thumbVisibility: true,
            controller: scrollController,
            child: SingleChildScrollView(
              controller: scrollController,
              child: ConstrainedBox(
                constraints: BoxConstraints(minWidth: viewportConstraints.maxWidth, maxWidth: viewportConstraints.maxWidth, minHeight: viewportConstraints.maxHeight),
                child: themedContent,
              ),
            ),
          ),
    );
  }
}

class WoxPreviewTagPills extends StatelessWidget {
  final WoxTheme woxTheme;
  final List<WoxPreviewTag> tags;

  const WoxPreviewTagPills({super.key, required this.woxTheme, required this.tags});

  @override
  Widget build(BuildContext context) {
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);
    final contentColor = safeFromCssColor(woxTheme.previewPropertyContentColor);
    final borderColor = safeFromCssColor(woxTheme.previewPropertyTitleColor, defaultColor: safeFromCssColor(woxTheme.previewSplitLineColor));

    // Pills are horizontally scrollable instead of wrapping so metadata never
    // steals height from the preview body on compact launcher windows. They are
    // styled as quiet metadata, not controls, because actions already live in
    // the launcher toolbar.
    // Tags show only their label by default to keep the metadata strip compact;
    // the tooltip carries explanatory text without expanding the footer.
    return SizedBox(
      // Preview metadata belongs to the launcher surface, so pill height and
      // text follow density while borders and radii remain theme-owned.
      height: WoxInterfaceSizeUtil.instance.current.scaledSpacing(26),
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: tags.length,
        separatorBuilder: (context, index) => SizedBox(width: WoxInterfaceSizeUtil.instance.current.scaledSpacing(8)),
        itemBuilder: (context, index) {
          final tag = tags[index];
          final visibleText = tag.label.trim().isEmpty ? tag.tooltip : tag.label;
          final tooltip = tag.tooltip.trim().isEmpty ? visibleText : tag.tooltip;

          return WoxTooltip(
            message: tooltip,
            child: Container(
              constraints: BoxConstraints(maxWidth: WoxInterfaceSizeUtil.instance.current.scaledSpacing(220)),
              padding: EdgeInsets.symmetric(horizontal: WoxInterfaceSizeUtil.instance.current.scaledSpacing(9), vertical: WoxInterfaceSizeUtil.instance.current.scaledSpacing(4)),
              decoration: BoxDecoration(
                color: fontColor.withValues(alpha: 0.035),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: borderColor.withValues(alpha: 0.48)),
              ),
              child: Text(
                visibleText,
                overflow: TextOverflow.ellipsis,
                maxLines: 1,
                style: TextStyle(
                  color: contentColor.withValues(alpha: 0.9),
                  fontSize: WoxInterfaceSizeUtil.instance.current.smallLabelFontSize,
                  height: 1.2,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
