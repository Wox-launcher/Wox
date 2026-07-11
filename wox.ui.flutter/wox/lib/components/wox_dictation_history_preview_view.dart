import 'package:flutter/material.dart';
import 'package:wox/components/file_preview/audio_file_preview_renderer.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/entity/wox_preview_dictation_history.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxDictationHistoryPreviewView extends StatelessWidget {
  final WoxPreviewDictationHistory data;
  final WoxTheme woxTheme;

  const WoxDictationHistoryPreviewView({super.key, required this.data, required this.woxTheme});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = safeFromCssColor(woxTheme.previewFontColor);
    final mutedColor = safeFromCssColor(woxTheme.previewPropertyContentColor, defaultColor: textColor);
    final accentColor = safeFromCssColor(woxTheme.queryBoxCursorColor, defaultColor: textColor);
    final dividerColor = safeFromCssColor(woxTheme.previewSplitLineColor).withValues(alpha: 0.32);

    return Padding(
      padding: EdgeInsets.fromLTRB(metrics.scaledSpacing(26), metrics.scaledSpacing(26), metrics.scaledSpacing(26), metrics.scaledSpacing(32)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _SectionHeader(
            label: data.refinedLabel,
            icon: data.hasOriginalTranscript ? Icons.auto_awesome_rounded : Icons.graphic_eq_rounded,
            iconColor: accentColor,
            mutedColor: mutedColor,
            statusLabel: data.statusLabel,
            isChanged: data.isChanged,
          ),
          SizedBox(height: metrics.scaledSpacing(16)),
          IntrinsicHeight(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Container(width: 2, decoration: BoxDecoration(color: accentColor.withValues(alpha: 0.72), borderRadius: BorderRadius.circular(999))),
                SizedBox(width: metrics.scaledSpacing(14)),
                Expanded(
                  child: WoxSelectableText(
                    data.refinedText,
                    style: TextStyle(
                      color: textColor.withValues(alpha: 0.94),
                      fontSize: metrics.resultTitleFontSize + 3,
                      height: 1.55,
                      fontWeight: FontWeight.w500,
                      letterSpacing: 0,
                    ),
                  ),
                ),
              ],
            ),
          ),
          if (data.hasOriginalTranscript) ...[
            Padding(padding: EdgeInsets.symmetric(vertical: metrics.scaledSpacing(24)), child: Divider(height: 1, thickness: 1, color: dividerColor)),
            _SectionHeader(label: data.originalLabel, icon: Icons.graphic_eq_rounded, iconColor: mutedColor.withValues(alpha: 0.7), mutedColor: mutedColor),
            SizedBox(height: metrics.scaledSpacing(12)),
            WoxSelectableText(
              data.originalText,
              style: TextStyle(
                color: textColor.withValues(alpha: data.isChanged ? 0.7 : 0.58),
                fontSize: metrics.resultSubtitleFontSize + 2,
                height: 1.55,
                fontWeight: FontWeight.w400,
                letterSpacing: 0,
              ),
            ),
          ],
          if (data.hasDiagnosticAudio) ...[
            Padding(padding: EdgeInsets.symmetric(vertical: metrics.scaledSpacing(24)), child: Divider(height: 1, thickness: 1, color: dividerColor)),
            _SectionHeader(label: data.audioLabel, icon: Icons.multitrack_audio_rounded, iconColor: accentColor.withValues(alpha: 0.78), mutedColor: mutedColor),
            SizedBox(height: metrics.scaledSpacing(14)),
            _AudioTrack(label: data.rawAudioLabel, filePath: data.rawAudioPath, textColor: textColor, mutedColor: mutedColor, borderColor: dividerColor),
            SizedBox(height: metrics.scaledSpacing(12)),
            _AudioTrack(label: data.processedAudioLabel, filePath: data.processedAudioPath, textColor: textColor, mutedColor: mutedColor, borderColor: dividerColor),
          ],
        ],
      ),
    );
  }
}

class _AudioTrack extends StatelessWidget {
  final String label;
  final String filePath;
  final Color textColor;
  final Color mutedColor;
  final Color borderColor;

  const _AudioTrack({required this.label, required this.filePath, required this.textColor, required this.mutedColor, required this.borderColor});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return Container(
      padding: EdgeInsets.fromLTRB(metrics.scaledSpacing(14), metrics.scaledSpacing(12), metrics.scaledSpacing(14), metrics.scaledSpacing(8)),
      decoration: BoxDecoration(color: textColor.withValues(alpha: 0.025), border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(metrics.scaledSpacing(12))),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Icon(Icons.play_circle_outline_rounded, color: mutedColor.withValues(alpha: 0.62), size: metrics.scaledSpacing(15)),
              SizedBox(width: metrics.scaledSpacing(7)),
              Text(label, style: TextStyle(color: mutedColor.withValues(alpha: 0.78), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w600, height: 1.1)),
            ],
          ),
          SizedBox(height: metrics.scaledSpacing(4)),
          WoxAudioFilePlayer(filePath: filePath, height: metrics.scaledSpacing(62)),
        ],
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color iconColor;
  final Color mutedColor;
  final String statusLabel;
  final bool isChanged;

  const _SectionHeader({required this.label, required this.icon, required this.iconColor, required this.mutedColor, this.statusLabel = "", this.isChanged = false});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;

    return Row(
      children: [
        Icon(icon, color: iconColor, size: metrics.scaledSpacing(16)),
        SizedBox(width: metrics.scaledSpacing(8)),
        Expanded(
          child: Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(color: mutedColor.withValues(alpha: 0.82), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700, height: 1.1, letterSpacing: 0.2),
          ),
        ),
        if (statusLabel.isNotEmpty) ...[
          SizedBox(width: metrics.scaledSpacing(10)),
          Icon(isChanged ? Icons.auto_awesome_rounded : Icons.check_rounded, color: mutedColor.withValues(alpha: 0.58), size: metrics.scaledSpacing(13)),
          SizedBox(width: metrics.scaledSpacing(4)),
          Text(statusLabel, style: TextStyle(color: mutedColor.withValues(alpha: 0.62), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w500, height: 1)),
        ],
      ],
    );
  }
}
