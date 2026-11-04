import assert from "node:assert/strict";
import { test } from "node:test";
import { sortStorePlugins } from "../docs/.vitepress/theme/components/pluginSort.mjs";

test("sorts by localized name and newest update, leaving missing dates last", () => {
  const plugins = [
    { LocalizedName: "Zebra", DateUpdated: "2025-01-01 12:00:00" },
    { LocalizedName: "Alpha", DateUpdated: "invalid" },
    { LocalizedName: "Beta", DateUpdated: "2026-01-01 12:00:00" },
  ];
  assert.deepEqual(sortStorePlugins(plugins, "name", "en-US").map((item) => item.LocalizedName), ["Alpha", "Beta", "Zebra"]);
  assert.deepEqual(sortStorePlugins(plugins, "updated", "en-US").map((item) => item.LocalizedName), ["Beta", "Zebra", "Alpha"]);
  assert.deepEqual(plugins.map((item) => item.LocalizedName), ["Zebra", "Alpha", "Beta"]);
});

test("keeps known zero counts ahead of unknown counts", () => {
  const plugins = [{ LocalizedName: "unknown" }, { LocalizedName: "zero", StarCount: 0 }, { LocalizedName: "high", StarCount: 2 }];
  assert.deepEqual(sortStorePlugins(plugins, "stars", "en").map((item) => item.LocalizedName), ["high", "zero", "unknown"]);
});
