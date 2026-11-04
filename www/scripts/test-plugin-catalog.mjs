import assert from "node:assert/strict";
import { test } from "node:test";
import { fetchRemotePluginCatalog } from "../docs/.vitepress/theme/components/remotePluginCatalog.mjs";

test("loads the shared remote catalog and preserves statistics", async () => {
  const plugins = [{ Id: "one", Name: "Plugin", StarCount: 3, DownloadCount: 10 }];
  const result = await fetchRemotePluginCatalog(async (url) => {
    assert.equal(url, "https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-plugin.json");
    return { ok: true, json: async () => plugins };
  });
  assert.deepEqual(result, plugins);
});

test("rejects failed and invalid responses for snapshot fallback", async () => {
  await assert.rejects(fetchRemotePluginCatalog(async () => ({ ok: false, status: 503 })), /503/);
  await assert.rejects(fetchRemotePluginCatalog(async () => ({ ok: true, json: async () => ({}) })), /Invalid/);
  await assert.rejects(fetchRemotePluginCatalog(async () => ({ ok: true, json: async () => [{ Id: "one" }] })), /Invalid/);
});
