(() => {
  if (window !== window.top) return;
  const token = __WOX_DOCUMENT_TOKEN__;
  const listeners = new Set();
  const send = (type, data) => {
    const value = JSON.stringify({ Token: token, Type: type, Data: data });
    if (new TextEncoder().encode(value).length > 262144) throw new Error("message too large");
    if (window.chrome?.webview) window.chrome.webview.postMessage("wox-message:" + value);
    else window.webkit.messageHandlers.woxMessage.postMessage(value);
  };
  Object.defineProperty(window, "woxWindow", {
    value: Object.freeze({
      postMessage(data) {
        if (JSON.stringify(data) === undefined) throw new Error("message must be JSON");
        send("message", data);
      },
      onMessage(fn) {
        if (typeof fn !== "function") throw new TypeError("listener must be a function");
        listeners.add(fn);
        return () => listeners.delete(fn);
      },
    }),
  });
  Object.defineProperty(window, "__woxWindowReceive", {
    value(id, data) {
      if (id !== token) return;
      for (const fn of listeners) {
        try {
          Promise.resolve(fn(data)).catch((error) => send("error", String(error)));
        } catch (error) {
          send("error", String(error));
        }
      }
    },
  });
  document.addEventListener("DOMContentLoaded", () => send("ready", null), { once: true });
  document.addEventListener(
    "click",
    (event) => {
      const anchor = event.target.closest?.("a[href]");
      if (anchor) {
        event.preventDefault();
        send("link", anchor.href);
      }
    },
    true,
  );
})();
