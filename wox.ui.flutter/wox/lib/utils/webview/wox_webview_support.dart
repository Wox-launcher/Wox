class WoxWebViewSupport {
  static const String mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1";
  static const String unhandledEscapeMessageType = "woxUnhandledEscape";
  static const String startDraggingMessageType = "woxStartDragging";

  static String buildInjectCssScript(String css) {
    final cssLiteral = _encodeJsString(css);
    return """
(() => {
  const css = $cssLiteral;
  if (!css) {
    return;
  }

  const styleId = "wox-webview-preview-style";
  let style = document.getElementById(styleId);
  if (!style) {
    style = document.createElement("style");
    style.id = styleId;
    (document.head || document.documentElement).appendChild(style);
  }
  style.textContent = css;
})();
""";
  }

  static String buildUnhandledEscapeScript({required String postMessageExpression}) {
    final messageTypeLiteral = _encodeJsString(unhandledEscapeMessageType);
    return """
(() => {
  if (window.__woxUnhandledEscapeInstalled__) {
    return;
  }

  window.__woxUnhandledEscapeInstalled__ = true;

  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape' || event.repeat) {
      return;
    }

    setTimeout(() => {
      if (event.defaultPrevented || event.cancelBubble) {
        return;
      }

      $postMessageExpression({ type: $messageTypeLiteral });
    }, 0);
  }, true);
})();
""";
  }

  /// Builds a page script that turns non-interactive WebView pointer starts into native window drag requests.
  static String buildStartDraggingScript({required String postMessageExpression}) {
    final messageTypeLiteral = _encodeJsString(startDraggingMessageType);
    return """
(() => {
  if (window.__woxStartDraggingInstalled__) {
    return;
  }

  window.__woxStartDraggingInstalled__ = true;

  const interactiveSelector = [
    'a[href]',
    'area[href]',
    'button',
    'input',
    'textarea',
    'select',
    'option',
    'summary',
    'label',
    '[contenteditable]',
    '[role="button"]',
    '[role="link"]',
    '[role="textbox"]',
    '[role="checkbox"]',
    '[role="radio"]',
    '[role="switch"]',
    '[role="slider"]',
    '[role="tab"]',
    '[role="menuitem"]',
    '[onclick]',
    '[data-wox-no-drag]',
    '[data-no-drag]',
    '[draggable="true"]',
  ].join(',');

  const isInteractiveElement = (element) => {
    if (!(element instanceof Element)) {
      return false;
    }

    if (element.isContentEditable) {
      return true;
    }

    return element.closest(interactiveSelector) !== null;
  };

  const isInteractiveTarget = (event) => {
    const path = typeof event.composedPath === 'function' ? event.composedPath() : [];
    for (const item of path) {
      if (item === window || item === document) {
        break;
      }
      if (isInteractiveElement(item)) {
        return true;
      }
    }

    return isInteractiveElement(event.target);
  };

  const isScrollbarClick = (event) => {
    const root = document.documentElement;
    if (!root) {
      return false;
    }

    return event.clientX >= root.clientWidth || event.clientY >= root.clientHeight;
  };

  const handlePointerStart = (event) => {
    if (event.defaultPrevented || event.button !== 0 || isScrollbarClick(event) || isInteractiveTarget(event)) {
      return;
    }

    $postMessageExpression({ type: $messageTypeLiteral });
  };

  document.addEventListener(window.PointerEvent ? 'pointerdown' : 'mousedown', handlePointerStart, true);
})();
""";
  }

  static String _encodeJsString(String input) {
    final escaped = input
        .replaceAll(r'\', r'\\')
        .replaceAll("'", r"\'")
        .replaceAll('\r', r'\r')
        .replaceAll('\n', r'\n')
        .replaceAll('\u2028', r'\u2028')
        .replaceAll('\u2029', r'\u2029');
    return "'$escaped'";
  }
}
