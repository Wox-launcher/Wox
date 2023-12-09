import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';

typedef PointerScrollEventListener = void Function(PointerScrollEvent event);
typedef ListViewItemGlobalKeyGetter = GlobalKey Function(int index);
typedef ListViewItemIsActiveGetter = bool Function(int index);
typedef ListViewItemParamsGetter = WoxListViewItemParams Function(int index);

class WoxListView extends StatelessWidget {
  final WoxTheme woxTheme;
  final ScrollController scrollController;
  final int itemCount;
  final double itemExtent;
  final ListViewItemParamsGetter listViewItemParamsGetter;
  final WoxListViewType listViewType;
  final ListViewItemGlobalKeyGetter listViewItemGlobalKeyGetter;
  final ListViewItemIsActiveGetter listViewItemIsActiveGetter;
  final PointerScrollEventListener? onMouseWheelScroll;

  const WoxListView(
      {super.key,
      required this.woxTheme,
      required this.scrollController,
      required this.itemCount,
      required this.itemExtent,
      required this.listViewType,
      required this.listViewItemParamsGetter,
      required this.listViewItemGlobalKeyGetter,
      required this.listViewItemIsActiveGetter,
      this.onMouseWheelScroll});

  @override
  Widget build(BuildContext context) {
    return Scrollbar(
        controller: scrollController,
        child: Listener(
            onPointerSignal: (event) {
              if (event is PointerScrollEvent) {
                onMouseWheelScroll?.call(event);
              }
            },
            child: ListView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              controller: scrollController,
              itemCount: itemCount,
              itemExtent: itemExtent,
              itemBuilder: (context, index) {
                return WoxListItemView(
                    key: listViewItemGlobalKeyGetter.call(index),
                    woxTheme: woxTheme,
                    icon: listViewItemParamsGetter.call(index).icon,
                    title: listViewItemParamsGetter.call(index).title,
                    subTitle: listViewItemParamsGetter.call(index).subTitle,
                    isActive: listViewItemIsActiveGetter.call(index),
                    listViewType: listViewType);
              },
            )));
  }
}
