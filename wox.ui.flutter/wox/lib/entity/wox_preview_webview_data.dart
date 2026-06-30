import 'dart:convert';

class WoxPreviewWebviewData {
  late String url;
  late String html;
  late String injectCss;
  late bool cacheDisabled;
  late String cacheKey;

  WoxPreviewWebviewData({required this.url, this.html = "", this.injectCss = "", this.cacheDisabled = false, this.cacheKey = ""});

  factory WoxPreviewWebviewData.fromJson(Map<String, dynamic> json) {
    return WoxPreviewWebviewData(
      url: json["url"]?.toString() ?? "",
      html: json["html"]?.toString() ?? "",
      injectCss: json["injectCss"]?.toString() ?? "",
      cacheDisabled: json["cacheDisabled"] == true,
      cacheKey: json["cacheKey"]?.toString() ?? "",
    );
  }

  factory WoxPreviewWebviewData.fromPreviewData(String previewData) {
    try {
      final decoded = jsonDecode(previewData);
      if (decoded is Map) {
        final json = Map<String, dynamic>.from(decoded);
        if (json["url"] is String) {
          return WoxPreviewWebviewData.fromJson(json);
        }
      }
    } catch (_) {
      // Keep backward compatibility with plain URL payloads.
    }

    return WoxPreviewWebviewData(url: previewData);
  }

  Map<String, dynamic> toJson() {
    return {"url": url, "html": html, "injectCss": injectCss, "cacheDisabled": cacheDisabled, "cacheKey": resolvedCacheKey};
  }

  String get resolvedCacheKey {
    if (cacheDisabled) {
      return "";
    }

    final trimmedCacheKey = cacheKey.trim();
    if (trimmedCacheKey.isNotEmpty) {
      return trimmedCacheKey;
    }

    final trimmedUrl = url.trim();
    if (trimmedUrl.isNotEmpty) {
      return trimmedUrl;
    }

    return html.trim();
  }
}
