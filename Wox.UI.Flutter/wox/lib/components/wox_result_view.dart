import 'package:flutter/material.dart';
import 'package:wox/entity.dart';

import 'wox_image_view.dart';

class WoxResultView extends StatelessWidget {
  final QueryResult result;
  final bool isActive;

  const WoxResultView({super.key, required this.result, required this.isActive});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: isActive ? Colors.red : Colors.transparent,
      child: Row(
        children: [
          WoxImageView(woxImage: result.icon),
          const SizedBox(width: 8),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(
                result.title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                strutStyle: const StrutStyle(
                  forceStrutHeight: true,
                ),
              ),
              Text(
                result.subTitle,
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
