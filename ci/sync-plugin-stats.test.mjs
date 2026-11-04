import assert from "node:assert/strict";
import { test } from "node:test";
import { fetchPluginStars, githubSource, fetchPluginDownloads, releaseAssetSource, updatePluginStats } from "./sync-plugin-stats.mjs";

test("recognizes repository and gist sources and falls back to the download URL", () => {
  assert.deepEqual(githubSource({ Website: "https://github.com/owner/repo", DownloadUrl: "https://example.com" }), { type: "repository", owner: "owner", name: "repo" });
  assert.deepEqual(githubSource({ Website: "https://gist.githubusercontent.com/owner/abcdef/raw/file" }), { type: "gist", owner: "owner", name: "abcdef" });
  assert.deepEqual(githubSource({ Website: "https://example.com", DownloadUrl: "https://github.com/owner/repo/releases/latest" }), { type: "repository", owner: "owner", name: "repo" });
  assert.equal(githubSource({ Website: "https://github.com.evil.test/owner/repo" }), null);
});

test("preserves zero counts and leaves missing sources unknown", async () => {
  const plugins = [
    { Id: "repo", Website: "https://github.com/owner/repo" },
    { Id: "gist", Website: "https://gist.github.com/owner/abcdef" },
    { Id: "missing", Website: "https://github.com/owner/missing" },
    { Id: "unsupported", Website: "https://example.com" },
  ];
  const stars = await fetchPluginStars(plugins, "test", async (_url, options) => {
    const query = JSON.parse(options.body).query;
    assert.match(query, /repository\(owner: "owner", name: "repo"\)/);
    assert.match(query, /gist\(name: "abcdef"\)/);
    return { ok: true, json: async () => ({ data: { p0: { stargazerCount: 12 }, p1: { gist: { stargazerCount: 0 } }, p2: null }, errors: [{ type: "NOT_FOUND" }] }) };
  });
  assert.deepEqual(stars, { repo: 12, gist: 0 });
});

test("fails on API errors so a failed sync cannot publish empty counts", async () => {
  await assert.rejects(fetchPluginStars([{ Id: "repo", Website: "https://github.com/owner/repo" }], "test", async () => ({ ok: true, json: async () => ({ data: {}, errors: [{ type: "RATE_LIMITED" }] }) })), /RATE_LIMITED/);
});


test("fetches large catalogs in batches", async () => {
  const plugins = Array.from({ length: 26 }, (_, index) => ({ Id: String(index), Website: `https://github.com/owner/repo${index}` }));
  let calls = 0;
  const stars = await fetchPluginStars(plugins, "test", async (_url, options) => {
    const query = JSON.parse(options.body).query;
    const aliases = [...query.matchAll(/p(\d+): repository/g)];
    calls++;
    return { ok: true, json: async () => ({ data: Object.fromEntries(aliases.map((match) => [`p${match[1]}`, { stargazerCount: calls }])) }) };
  });
  assert.equal(calls, 2);
  assert.equal(stars["24"], 1);
  assert.equal(stars["25"], 2);
});


test("recognizes only GitHub release asset download URLs", () => {
  assert.deepEqual(releaseAssetSource({ DownloadUrl: "https://github.com/a/b/releases/latest/download/plugin.wox" }), { owner: "a", repo: "b", name: "plugin.wox" });
  assert.deepEqual(releaseAssetSource({ DownloadUrl: "https://github.com/a/b/releases/download/v1/plugin%20one.wox" }), { owner: "a", repo: "b", name: "plugin one.wox" });
  assert.equal(releaseAssetSource({ DownloadUrl: "https://gist.githubusercontent.com/a/abcd/raw/file.py" }), null);
  assert.equal(releaseAssetSource({ DownloadUrl: "https://github.com.evil.test/a/b/releases/latest/download/plugin.wox" }), null);
});

test("batches repositories and paginates releases without mixing zero and missing counts", async () => {
  const plugins = [
    { Id: "one", DownloadUrl: "https://github.com/a/b/releases/latest/download/plugin.wox" },
    { Id: "zero", DownloadUrl: "https://github.com/a/b/releases/latest/download/zero.wox" },
    { Id: "missing", DownloadUrl: "https://github.com/a/b/releases/latest/download/missing.wox" },
    { Id: "gist", DownloadUrl: "https://gist.github.com/a/abcd" },
  ];
  let calls = 0;
  const fakeFetch = async (_url, options) => {
    const query = JSON.parse(options.body).query;
    calls++;
    const release = (name, count, isDraft = false) => ({ isDraft, releaseAssets: { nodes: name ? [{ name, downloadCount: count }] : [] } });
    const page = (nodes, more, cursor) => ({ releases: { nodes, pageInfo: { hasNextPage: more, endCursor: cursor } } });
    if (calls === 1) {
      assert.match(query, /orderBy: \{field: CREATED_AT, direction: DESC\}/);
      assert.match(query, /releaseAssets\(first: 1, name: "plugin.wox"\)/);
      return { ok: true, json: async () => ({ data: { p0: page([release("plugin.wox", 3), release("plugin.wox", 100, true)], true, "next"), p1: page([release("zero.wox", 0)], false, "z"), p2: page([], false, null) } }) };
    }
    assert.match(query, /after: "next"/);
    return { ok: true, json: async () => ({ data: { p0: page([release("plugin.wox", 5)], true, "end") } }) };
  };
  assert.deepEqual(await fetchPluginDownloads(plugins, "token", fakeFetch), { zero: 0, one: 8 });
  assert.equal(calls, 2);
});

test("does not publish partial totals when the API fails", async () => {
  const plugins = [{ Id: "one", DownloadUrl: "https://github.com/a/b/releases/latest/download/plugin.wox" }];
  await assert.rejects(fetchPluginDownloads(plugins, "token", async () => ({ ok: false, status: 403 })), /HTTP 403/);
});


test("updates only statistics and distinguishes unknown from zero", () => {
  const plugins = [{ Id: "one", Name: "Plugin", StarCount: 5, DownloadCount: 9, Extra: { kept: true } }, { Id: "two", Name: "Other" }];
  updatePluginStats(plugins, { one: 0 }, { two: 12 });
  assert.deepEqual(plugins, [{ Id: "one", Name: "Plugin", StarCount: 0, Extra: { kept: true } }, { Id: "two", Name: "Other", DownloadCount: 12 }]);
});
