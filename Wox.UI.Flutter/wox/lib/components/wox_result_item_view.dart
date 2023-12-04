import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_theme.dart';

class WoxResultItemView extends StatelessWidget {
  final bool isActive;
  final WoxImage icon;
  final String title;
  final String subTitle;
  final WoxTheme woxTheme;

  const WoxResultItemView({
    super.key,
    required this.woxTheme,
    required this.icon,
    required this.title,
    required this.subTitle,
    required this.isActive,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: isActive ? fromCssColor(woxTheme.resultItemActiveBackgroundColor) : Colors.transparent,
        borderRadius: BorderRadius.circular(woxTheme.resultItemBorderRadius.toDouble()),
        border: Border(
            left: BorderSide(
          color: isActive ? fromCssColor(woxTheme.resultItemActiveBackgroundColor) : Colors.transparent,
          width: isActive ? double.parse(woxTheme.resultItemActiveBorderLeft) : double.parse(woxTheme.resultItemBorderLeft),
        )),
      ),
      padding: EdgeInsets.only(
        top: woxTheme.resultItemPaddingTop.toDouble(),
        right: woxTheme.resultItemPaddingRight.toDouble(),
        bottom: woxTheme.resultItemPaddingBottom.toDouble(),
        left: woxTheme.resultItemPaddingLeft.toDouble(),
      ),
      child: Row(
        children: [
          Padding(
              padding: const EdgeInsets.only(left: 5.0, right: 10.0),
              child: WoxImageView(
                woxImage: icon,
                width: 30,
                height: 30,
              )),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisAlignment: MainAxisAlignment.center, children: [
              Text(
                style: TextStyle(
                  color: fromCssColor(woxTheme.resultItemTitleColor),
                ),
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                strutStyle: const StrutStyle(
                  forceStrutHeight: true,
                ),
              ),
              if (subTitle.isNotEmpty)
                Text(
                  style: TextStyle(
                    color: fromCssColor(woxTheme.resultItemSubTitleColor),
                  ),
                  subTitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  strutStyle: const StrutStyle(
                    forceStrutHeight: true,
                  ),
                ),
            ]),
          ),
        ],
      ),
    );
  }
}
