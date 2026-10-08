"""Host-provided client for long-lived JSON-RPC plugins.

Plugins subclass FlowLauncher. The host speaks newline-delimited JSON-RPC 2.0
on stdin and stdout. Prints from plugin code are not part of the protocol;
use debug() for diagnostic text.
"""

import json
import sys
import traceback


class FlowLauncher(object):
    def __init__(self):
        self.settings = {}
        self.rpc_request = {"settings": self.settings}
        self._run()

    def _run(self):
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue
            try:
                message = json.loads(line)
            except Exception:
                continue
            if not isinstance(message, dict) or "method" not in message:
                continue
            response_id = message.get("id", None)
            try:
                result = self._handle(message)
                error = None
            except Exception:
                result = None
                error = traceback.format_exc()
                print(error, file=sys.stderr)
            if response_id is None:
                continue
            payload = {"jsonrpc": "2.0", "id": response_id}
            if error:
                payload["error"] = {"code": -32000, "message": error}
            else:
                payload["result"] = result
            self._write(payload)

    def _handle(self, message):
        method = message.get("method") or ""
        params = message.get("params") or []
        if not isinstance(params, list):
            params = [params]
        if method == "query":
            raw = params[0] if params else ""
            if len(params) > 1 and isinstance(params[1], dict):
                self.settings = params[1]
                self.rpc_request["settings"] = self.settings
            text = ""
            if isinstance(raw, dict):
                text = raw.get("Search") or raw.get("search") or ""
            elif raw is not None:
                text = str(raw)
            results = self.query(text)
            return {"result": self._normalize_results(results)}
        if method == "context_menu":
            data = params[0] if params else None
            results = self.context_menu(data)
            return {"result": self._normalize_results(results)}
        target = getattr(self, method, None)
        if target is None or not callable(target):
            raise AttributeError("plugin has no method %s" % method)
        try:
            target(*params)
        except TypeError:
            target()
        return None

    def _normalize_results(self, results):
        if results is None:
            return []
        if isinstance(results, dict):
            return [results]
        if isinstance(results, list):
            return results
        return list(results)

    def query(self, query):
        return []

    def context_menu(self, data):
        return []

    def change_query(self, query, requery=False):
        self._notify("Flow.Launcher.ChangeQuery", [query, bool(requery)])

    def shell_run(self, cmd):
        self._notify("Flow.Launcher.ShellRun", [cmd])

    def copy_to_clipboard(self, text):
        self._notify("Flow.Launcher.CopyToClipboard", [text])

    def open_url(self, url):
        self._notify("Flow.Launcher.OpenUrl", [url])

    def open_directory(self, directory, file_name=""):
        self._notify("Flow.Launcher.OpenDirectory", [directory, file_name])

    def open_app_uri(self, uri):
        self._notify("Flow.Launcher.OpenAppUri", [uri])

    def show_msg(self, title, sub_title="", ico_path=""):
        self._notify("Flow.Launcher.ShowMsg", [title, sub_title, ico_path])

    def hide_app(self):
        self._notify("Flow.Launcher.HideApp", [])

    def show_app(self):
        self._notify("Flow.Launcher.ShowApp", [])

    def debug(self, msg):
        self._notify("Flow.Launcher.Log", [str(msg)])

    def _notify(self, method, params):
        # Notifications have no id, so query() can call the host without waiting
        # for a reply while the host is still waiting for this query result.
        self._write({"jsonrpc": "2.0", "method": method, "params": params})

    def _write(self, payload):
        sys.stdout.write(json.dumps(payload) + "\n")
        sys.stdout.flush()
