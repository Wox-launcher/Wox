import 'dart:convert';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/colors.dart';
import 'package:get/get.dart';

class EmojiGroupData {
  final String labelKey;
  final IconData icon;
  final List<String> emojis;

  const EmojiGroupData({required this.labelKey, required this.icon, required this.emojis});
}

class WoxImageSelector extends StatelessWidget {
  final WoxImage value;
  final ValueChanged<WoxImage> onChanged;
  final double previewSize;

  const WoxImageSelector({super.key, required this.value, required this.onChanged, this.previewSize = 80});

  static const List<EmojiGroupData> emojiGroups = [
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_recommended",
      icon: Icons.auto_awesome,
      emojis: [
        "🤖",
        "💡",
        "🔍",
        "📊",
        "📈",
        "📝",
        "🛠",
        "⚙️",
        "🧠",
        "✅",
        "🚀",
        "🎯",
        "🔥",
        "⭐",
        "🌟",
        "💻",
        "📱",
        "🌐",
        "🔒",
        "🧩",
        "📌",
        "📎",
        "🗂️",
        "📦",
        "📁",
        "📂",
        "⌨️",
        "🖥️",
        "⚡",
        "🏷️",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_faces",
      icon: Icons.emoji_emotions_outlined,
      emojis: [
        "😀",
        "😃",
        "😄",
        "😁",
        "😆",
        "😅",
        "😂",
        "🤣",
        "😊",
        "🙂",
        "😉",
        "😍",
        "😘",
        "😜",
        "🤩",
        "🤔",
        "🤨",
        "😐",
        "😑",
        "🙄",
        "😏",
        "😶",
        "😮",
        "😲",
        "😴",
        "😌",
        "🤤",
        "😷",
        "🤒",
        "🤕",
        "🥳",
        "😎",
        "🤯",
        "😇",
        "🥲",
        "😭",
        "😡",
        "🤬",
        "🥶",
        "🥵",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_people",
      icon: Icons.people_alt_outlined,
      emojis: [
        "👤",
        "👥",
        "🧑",
        "👨",
        "👩",
        "🧔",
        "👱",
        "👶",
        "🧒",
        "👦",
        "👧",
        "🧑‍💻",
        "👨‍💻",
        "👩‍💻",
        "🧑‍🔬",
        "🧑‍🏫",
        "🧑‍🎨",
        "🧑‍🚀",
        "🧑‍🚒",
        "🧑‍⚕️",
        "🕵️",
        "💂",
        "👷",
        "🧙",
        "🧛",
        "🧟",
        "🙋",
        "🙌",
        "👏",
        "👍",
        "👎",
        "👊",
        "✌️",
        "🤝",
        "🙏",
        "💪",
        "👀",
        "🫶",
        "👋",
        "🤟",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_animals",
      icon: Icons.pets_outlined,
      emojis: [
        "🐶",
        "🐱",
        "🐭",
        "🐹",
        "🐰",
        "🦊",
        "🐻",
        "🐼",
        "🐨",
        "🐯",
        "🦁",
        "🐮",
        "🐷",
        "🐸",
        "🐵",
        "🦄",
        "🐔",
        "🐧",
        "🐦",
        "🐤",
        "🦆",
        "🦅",
        "🦉",
        "🦇",
        "🐺",
        "🐗",
        "🐴",
        "🦋",
        "🐝",
        "🐞",
        "🐢",
        "🐍",
        "🦖",
        "🦕",
        "🐙",
        "🦑",
        "🐬",
        "🐳",
        "🦈",
        "🌿",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_food",
      icon: Icons.restaurant_menu_outlined,
      emojis: [
        "🍎",
        "🍊",
        "🍋",
        "🍌",
        "🍉",
        "🍇",
        "🍓",
        "🫐",
        "🍒",
        "🍍",
        "🥭",
        "🥝",
        "🍅",
        "🥑",
        "🥦",
        "🥕",
        "🌽",
        "🍞",
        "🥐",
        "🧀",
        "🍗",
        "🍖",
        "🍔",
        "🍟",
        "🍕",
        "🌭",
        "🌮",
        "🌯",
        "🍜",
        "🍝",
        "🍣",
        "🍤",
        "🍰",
        "🧁",
        "🍩",
        "🍪",
        "🍫",
        "☕",
        "🍵",
        "🥤",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_activities",
      icon: Icons.sports_esports_outlined,
      emojis: [
        "⚽",
        "🏀",
        "🏈",
        "⚾",
        "🎾",
        "🏐",
        "🏉",
        "🥏",
        "🎱",
        "🏓",
        "🏸",
        "🏒",
        "🏑",
        "🥍",
        "🏏",
        "⛳",
        "🏹",
        "🎣",
        "🥊",
        "🥋",
        "🎮",
        "🕹️",
        "🎲",
        "🧩",
        "♟️",
        "🎯",
        "🎳",
        "🎼",
        "🎵",
        "🎶",
        "🎸",
        "🎹",
        "🥁",
        "🎻",
        "🎺",
        "🎨",
        "🎭",
        "🎬",
        "🎪",
        "🏆",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_travel",
      icon: Icons.travel_explore_outlined,
      emojis: [
        "🚗",
        "🚕",
        "🚙",
        "🚌",
        "🚎",
        "🏎️",
        "🚓",
        "🚑",
        "🚒",
        "🚚",
        "🚜",
        "🚲",
        "🛴",
        "🏍️",
        "🚨",
        "🚦",
        "🚥",
        "🛣️",
        "🛤️",
        "⛽",
        "🚉",
        "✈️",
        "🛫",
        "🛬",
        "🚁",
        "🚀",
        "🛰️",
        "⛵",
        "🚤",
        "🛳️",
        "🗺️",
        "🧭",
        "🏔️",
        "🏕️",
        "🏖️",
        "🏝️",
        "🏜️",
        "🌋",
        "🌅",
        "🌃",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_objects",
      icon: Icons.category_outlined,
      emojis: [
        "📱",
        "☎️",
        "💻",
        "⌨️",
        "🖥️",
        "🖨️",
        "🖱️",
        "💾",
        "💿",
        "📷",
        "📹",
        "🎥",
        "📞",
        "📟",
        "📠",
        "📺",
        "📻",
        "🧭",
        "⏰",
        "⏱️",
        "💡",
        "🔦",
        "🕯️",
        "🧯",
        "🧰",
        "🔧",
        "🔨",
        "⚒️",
        "🪛",
        "🔩",
        "⚙️",
        "🧲",
        "🧪",
        "🧫",
        "🧬",
        "💊",
        "📦",
        "📚",
        "🗂️",
        "🗃️",
      ],
    ),
    EmojiGroupData(
      labelKey: "ui_select_emoji_group_symbols",
      icon: Icons.tag_outlined,
      emojis: [
        "❤️",
        "🧡",
        "💛",
        "💚",
        "💙",
        "💜",
        "🖤",
        "🤍",
        "🤎",
        "💔",
        "❣️",
        "💕",
        "💞",
        "💯",
        "✅",
        "☑️",
        "✔️",
        "❌",
        "⚠️",
        "❗",
        "❓",
        "💤",
        "♻️",
        "⚜️",
        "🔱",
        "♾️",
        "™️",
        "©️",
        "®️",
        "➕",
        "➖",
        "✖️",
        "➗",
        "🟢",
        "🔴",
        "🟡",
        "🔵",
        "🟣",
        "⚪",
        "⚫",
      ],
    ),
  ];

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  Future<String?> showEmojiPicker(BuildContext context) async {
    final initialEmoji = value.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code ? value.imageData : null;

    return showDialog<String>(
      context: context,
      builder: (context) {
        return EmojiPickerDialog(initialEmoji: initialEmoji, tr: tr);
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Container(
          width: previewSize,
          height: previewSize,
          decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withAlpha(76)), borderRadius: BorderRadius.circular(8)),
          child: ClipRRect(borderRadius: BorderRadius.circular(8), child: Center(child: WoxImageView(woxImage: value, width: previewSize, height: previewSize))),
        ),
        const SizedBox(width: 16),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            WoxButton.secondary(
              text: tr("ui_image_editor_emoji"),
              icon: Icon(Icons.emoji_emotions_outlined, size: 14, color: getThemeTextColor()),
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
              onPressed: () async {
                final emojiResult = await showEmojiPicker(context);
                if (emojiResult != null && emojiResult.isNotEmpty) {
                  onChanged(WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: emojiResult));
                }
              },
            ),
            WoxButton.secondary(
              text: tr("ui_image_editor_upload_image"),
              icon: Icon(Icons.file_upload_outlined, size: 14, color: getThemeTextColor()),
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
              onPressed: () async {
                final result = await FilePicker.platform.pickFiles(type: FileType.image, allowMultiple: false);

                if (result != null && result.files.isNotEmpty && result.files.first.path != null) {
                  final filePath = result.files.first.path!;
                  final file = File(filePath);
                  if (await file.exists()) {
                    final bytes = await file.readAsBytes();
                    final base64Image = base64Encode(bytes);
                    onChanged(WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: "data:image/png;base64,$base64Image"));
                  }
                }
              },
            ),
          ],
        ),
      ],
    );
  }
}

class EmojiPickerDialog extends StatefulWidget {
  final String? initialEmoji;
  final String Function(String) tr;

  const EmojiPickerDialog({super.key, required this.initialEmoji, required this.tr});

  @override
  State<EmojiPickerDialog> createState() => EmojiPickerDialogState();
}

class EmojiPickerDialogState extends State<EmojiPickerDialog> {
  int selectedGroupIndex = 0;

  @override
  void initState() {
    super.initState();
    if (widget.initialEmoji == null || widget.initialEmoji == "") {
      return;
    }

    for (var i = 0; i < WoxImageSelector.emojiGroups.length; i++) {
      if (WoxImageSelector.emojiGroups[i].emojis.contains(widget.initialEmoji)) {
        selectedGroupIndex = i;
        return;
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final themeBackground = getThemeBackgroundColor();
    final isDarkTheme = themeBackground.computeLuminance() < 0.5;
    final baseSurface = themeBackground.withAlpha(255);
    final accentColor = getThemeActiveBackgroundColor();
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final cardColor = (isDarkTheme ? baseSurface.lighter(12) : baseSurface.darker(6)).withAlpha(255);
    final outlineColor = accentColor.withValues(alpha: isDarkTheme ? 0.22 : 0.15);
    final panelBackground = isDarkTheme ? cardColor.lighter(6).withAlpha(255) : cardColor.darker(2).withAlpha(255);
    final chipBorderColor = getThemeDividerColor().withValues(alpha: 0.45);
    final selectedChipColor = accentColor.withValues(alpha: isDarkTheme ? 0.30 : 0.20);
    final currentGroup = WoxImageSelector.emojiGroups[selectedGroupIndex];
    final baseTheme = Theme.of(context);
    final dialogTheme = baseTheme.copyWith(
      colorScheme: ColorScheme.fromSeed(seedColor: accentColor, brightness: isDarkTheme ? Brightness.dark : Brightness.light),
      scaffoldBackgroundColor: Colors.transparent,
      cardColor: cardColor,
      shadowColor: textColor.withAlpha(50),
    );

    return Theme(
      data: dialogTheme,
      child: Focus(
        autofocus: true,
        onKeyEvent: (node, event) {
          if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
            Navigator.pop(context);
            return KeyEventResult.handled;
          }
          return KeyEventResult.ignored;
        },
        child: AlertDialog(
          backgroundColor: cardColor,
          surfaceTintColor: Colors.transparent,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20), side: BorderSide(color: outlineColor)),
          elevation: 18,
          insetPadding: const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
          contentPadding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
          actionsPadding: const EdgeInsets.fromLTRB(24, 12, 24, 24),
          actionsAlignment: MainAxisAlignment.end,
          content: SizedBox(
            width: 760,
            height: 500,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(child: Text(widget.tr("ui_select_emoji"), style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: textColor))),
                    IconButton(onPressed: () => Navigator.pop(context), tooltip: widget.tr("ui_cancel"), icon: Icon(Icons.close_rounded, color: subTextColor)),
                  ],
                ),
                const SizedBox(height: 8),
                SizedBox(
                  height: 46,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    itemCount: WoxImageSelector.emojiGroups.length,
                    separatorBuilder: (context, index) => const SizedBox(width: 8),
                    itemBuilder: (context, index) {
                      final group = WoxImageSelector.emojiGroups[index];
                      final selected = selectedGroupIndex == index;
                      return InkWell(
                        onTap: () {
                          setState(() {
                            selectedGroupIndex = index;
                          });
                        },
                        borderRadius: BorderRadius.circular(20),
                        child: AnimatedContainer(
                          duration: const Duration(milliseconds: 160),
                          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                          decoration: BoxDecoration(
                            color: selected ? selectedChipColor : panelBackground,
                            borderRadius: BorderRadius.circular(20),
                            border: Border.all(color: selected ? accentColor : chipBorderColor),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(group.icon, size: 16, color: selected ? accentColor : subTextColor),
                              const SizedBox(width: 6),
                              Text(
                                widget.tr(group.labelKey),
                                style: TextStyle(color: selected ? textColor : subTextColor, fontSize: 12, fontWeight: selected ? FontWeight.w600 : FontWeight.w500),
                              ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                ),
                const SizedBox(height: 12),
                Expanded(
                  child: Container(
                    decoration: BoxDecoration(color: panelBackground, borderRadius: BorderRadius.circular(14), border: Border.all(color: chipBorderColor)),
                    child: LayoutBuilder(
                      builder: (context, constraints) {
                        final rawCount = (constraints.maxWidth / 58).floor();
                        final crossAxisCount = rawCount.clamp(6, 12).toInt();
                        return GridView.builder(
                          padding: const EdgeInsets.all(14),
                          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(crossAxisCount: crossAxisCount, childAspectRatio: 1.0, crossAxisSpacing: 8, mainAxisSpacing: 8),
                          itemCount: currentGroup.emojis.length,
                          itemBuilder: (context, index) {
                            final emoji = currentGroup.emojis[index];
                            final selected = widget.initialEmoji == emoji;
                            return Material(
                              color: Colors.transparent,
                              child: InkWell(
                                borderRadius: BorderRadius.circular(10),
                                onTap: () {
                                  Navigator.pop(context, emoji);
                                },
                                child: AnimatedContainer(
                                  duration: const Duration(milliseconds: 120),
                                  decoration: BoxDecoration(
                                    color: selected ? selectedChipColor : Colors.transparent,
                                    borderRadius: BorderRadius.circular(10),
                                    border: Border.all(color: selected ? accentColor : Colors.transparent),
                                  ),
                                  child: Center(child: Text(emoji, style: const TextStyle(fontSize: 26, height: 1))),
                                ),
                              ),
                            );
                          },
                        );
                      },
                    ),
                  ),
                ),
              ],
            ),
          ),
          actions: [WoxButton.secondary(text: widget.tr("ui_cancel"), padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12), onPressed: () => Navigator.pop(context))],
        ),
      ),
    );
  }
}
