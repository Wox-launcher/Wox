import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_svg/svg.dart';
import 'package:lottie/lottie.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_theme_icon_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_http_util.dart';

typedef WoxLazyImageLoaded = void Function(String traceId, WoxImage image);

class WoxImageView extends StatelessWidget {
  final WoxImage woxImage;
  final double? width;
  final double? height;
  final Color? svgColor;
  final WoxLazyImageLoaded? onLazyImageLoaded;
  // Feature change: callers such as the aspect-ratio grid need cover-style
  // thumbnails, while existing icon surfaces must keep the previous contain
  // behavior. Owning the fit here avoids duplicating image-type branches in
  // every caller that renders a WoxImage.
  final BoxFit fit;

  const WoxImageView({super.key, required this.woxImage, this.width, this.height, this.svgColor, this.onLazyImageLoaded, this.fit = BoxFit.contain});

  ColorFilter? get _svgColorFilter => svgColor == null ? null : ColorFilter.mode(svgColor!, BlendMode.srcIn);

  bool _isSvgUrl(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null) {
      return false;
    }

    return uri.path.toLowerCase().endsWith('.svg');
  }

  bool _isSvgFile(String path) {
    return path.toLowerCase().endsWith('.svg');
  }

  Widget _buildLoadingPlaceholder() {
    final indicatorSize = ((width ?? height ?? 24) * 0.65).clamp(14.0, 28.0);

    return SizedBox(width: width, height: height, child: Center(child: WoxLoadingIndicator(size: indicatorSize)));
  }

  Widget _buildErrorPlaceholder(Object error, StackTrace? stackTrace) {
    var traceId = const UuidV4().generate();
    Logger.instance.error(traceId, "Failed to load wox url image: $error");
    Logger.instance.error(traceId, "Image URL: ${woxImage.imageData}");
    Logger.instance.error(traceId, "Stack trace: $stackTrace");
    return SizedBox(width: width, height: height);
  }

  @override
  Widget build(BuildContext context) {
    late final Widget content;

    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LAZY_LOAD_IMAGE.code) {
      final payload = woxImage.lazyLoadPayload();
      content =
          payload == null
              ? SizedBox(width: width, height: height)
              : _WoxLazyLoadImageView(payload: payload, width: width, height: height, svgColor: svgColor, fit: fit, onLoaded: onLazyImageLoaded);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code) {
      if (_isSvgUrl(woxImage.imageData)) {
        content = _WoxSvgNetworkImageView(
          url: woxImage.imageData,
          width: width,
          height: height,
          fit: fit,
          colorFilter: _svgColorFilter,
          loadingBuilder: _buildLoadingPlaceholder,
          errorBuilder: _buildErrorPlaceholder,
        );
      } else {
        content = Image.network(
          woxImage.imageData,
          width: width,
          height: height,
          fit: fit,
          gaplessPlayback: true,
          errorBuilder: (context, error, stackTrace) => _buildErrorPlaceholder(error, stackTrace),
          loadingBuilder: (context, child, loadingProgress) {
            if (loadingProgress == null) return child;
            return _buildLoadingPlaceholder();
          },
        );
      }
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code) {
      if (woxImage.cachedFile == null) {
        content = const SizedBox(width: 24, height: 24);
      } else if (_isSvgFile(woxImage.imageData)) {
        content = SvgPicture.file(
          woxImage.cachedFile!,
          width: width,
          height: height,
          fit: fit,
          colorFilter: _svgColorFilter,
          placeholderBuilder: (context) => _buildLoadingPlaceholder(),
        );
      } else {
        content = Image.file(
          woxImage.cachedFile!,
          width: width,
          height: height,
          fit: fit,
          gaplessPlayback: true,
          errorBuilder: (context, error, stackTrace) => SizedBox(width: width, height: height),
        );
      }
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      content = SizedBox(width: width, height: height, child: SvgPicture.string(woxImage.imageData, colorFilter: _svgColorFilter));
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code) {
      // Use FittedBox to uniformly scale the emoji to fit the container,
      // which works correctly across all platforms and display sizes.
      content = SizedBox(width: width, height: height, child: FittedBox(fit: BoxFit.contain, child: Text(woxImage.imageData, style: const TextStyle(fontSize: 100, height: 1.0))));
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LOTTIE.code) {
      final bytes = utf8.encode(woxImage.imageData);
      content = Lottie.memory(bytes, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_THEME.code) {
      content = WoxThemeIconView(theme: WoxTheme.fromJson(jsonDecode(woxImage.imageData)), width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      if (!woxImage.imageData.contains(";base64,")) {
        content = Text("Invalid image data: ${woxImage.imageData}", style: const TextStyle(color: Colors.red));
      } else {
        final imageData = woxImage.imageData.split(";base64,")[1];
        content = Image.memory(base64Decode(imageData), width: width, height: height, fit: fit, gaplessPlayback: true);
      }
    } else {
      content = const SizedBox(width: 24, height: 24);
    }

    return content;
  }
}

class _WoxSvgNetworkImageView extends StatefulWidget {
  final String url;
  final double? width;
  final double? height;
  final BoxFit fit;
  final ColorFilter? colorFilter;
  final Widget Function() loadingBuilder;
  final Widget Function(Object error, StackTrace? stackTrace) errorBuilder;

  const _WoxSvgNetworkImageView({required this.url, this.width, this.height, required this.fit, this.colorFilter, required this.loadingBuilder, required this.errorBuilder});

  @override
  State<_WoxSvgNetworkImageView> createState() => _WoxSvgNetworkImageViewState();
}

class _WoxSvgNetworkImageViewState extends State<_WoxSvgNetworkImageView> {
  late Future<String> _svgFuture;

  @override
  void initState() {
    super.initState();
    _svgFuture = _loadSvg();
  }

  @override
  void didUpdateWidget(covariant _WoxSvgNetworkImageView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.url != widget.url) {
      _svgFuture = _loadSvg();
    }
  }

  Future<String> _loadSvg() async {
    try {
      // Bug fix: SvgPicture.network can surface socket failures as late Flutter
      // test errors even with errorBuilder. Fetch the SVG text here so network
      // failures stay inside this FutureBuilder and degrade to the normal empty
      // image placeholder instead of failing unrelated smoke cases.
      final response = await Dio().get<String>(widget.url, options: Options(responseType: ResponseType.plain)).timeout(const Duration(seconds: 8));
      return response.data ?? "";
    } catch (error, stackTrace) {
      Error.throwWithStackTrace(error, stackTrace);
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<String>(
      future: _svgFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return widget.loadingBuilder();
        }
        if (snapshot.hasError || (snapshot.data ?? "").isEmpty) {
          return widget.errorBuilder(snapshot.error ?? "Empty SVG response", snapshot.stackTrace);
        }

        return SvgPicture.string(
          snapshot.data!,
          width: widget.width,
          height: widget.height,
          fit: widget.fit,
          colorFilter: widget.colorFilter,
          errorBuilder: (context, error, stackTrace) => widget.errorBuilder(error, stackTrace),
        );
      },
    );
  }
}

class _WoxLazyLoadImageView extends StatefulWidget {
  final WoxLazyLoadImagePayload payload;
  final double? width;
  final double? height;
  final Color? svgColor;
  final BoxFit fit;
  final WoxLazyImageLoaded? onLoaded;

  const _WoxLazyLoadImageView({required this.payload, this.width, this.height, this.svgColor, required this.fit, this.onLoaded});

  @override
  State<_WoxLazyLoadImageView> createState() => _WoxLazyLoadImageViewState();
}

class _WoxLazyLoadImageViewState extends State<_WoxLazyLoadImageView> {
  static final Map<String, Future<WoxImage>> _inFlightLoads = {};

  WoxImage? _loadedImage;
  String _activeToken = "";

  @override
  void initState() {
    super.initState();
    _loadVisibleImage();
  }

  @override
  void didUpdateWidget(covariant _WoxLazyLoadImageView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.payload.token != widget.payload.token) {
      _loadedImage = null;
      _loadVisibleImage();
    }
  }

  void _loadVisibleImage() {
    final token = widget.payload.token;
    if (token.isEmpty || _loadedImage != null) {
      return;
    }

    _activeToken = token;
    final traceId = const UuidV4().generate();
    // Lazy result icons deliberately dedupe only in-flight requests. Once core
    // returns the resized image, the launcher result model is updated to the real
    // absolute icon, so a long-lived token cache in Flutter would duplicate the
    // core image cache and risk serving stale query tokens.
    final future = _inFlightLoads.putIfAbsent(token, () => WoxHttpUtil.instance.postData<WoxImage>(traceId, "/image/lazy/load", {"token": token}));
    unawaited(
      future
          .then((image) {
            if (!mounted || _activeToken != token) {
              return;
            }
            setState(() {
              _loadedImage = image;
            });
            widget.onLoaded?.call(traceId, image);
          })
          .catchError((error, stackTrace) {
            Logger.instance.warn(traceId, "Failed to lazy load wox image: $error");
          })
          .whenComplete(() {
            if (identical(_inFlightLoads[token], future)) {
              _inFlightLoads.remove(token);
            }
          }),
    );
  }

  @override
  Widget build(BuildContext context) {
    return WoxImageView(woxImage: _loadedImage ?? widget.payload.placeholder, width: widget.width, height: widget.height, svgColor: widget.svgColor, fit: widget.fit);
  }
}
