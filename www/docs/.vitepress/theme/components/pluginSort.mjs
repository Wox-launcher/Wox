// Sort a filtered catalog without changing its source order.
export function sortStorePlugins(plugins, order, lang) {
  const result = [...plugins];
  if (order === "name") {
    result.sort((left, right) => left.LocalizedName.localeCompare(right.LocalizedName, lang || "en"));
  } else if (order === "updated") {
    const updatedAt = (plugin) => {
      const date = plugin.DateUpdated;
      if (!date || !/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(date)) return -Infinity;
      const value = Date.parse(`${date.replace(" ", "T")}Z`);
      return Number.isNaN(value) ? -Infinity : value;
    };
    result.sort((left, right) => updatedAt(right) - updatedAt(left));
  } else if (order === "stars" || order === "downloads") {
    const key = order === "stars" ? "StarCount" : "DownloadCount";
    result.sort((left, right) => (right[key] ?? -1) - (left[key] ?? -1));
  }
  return result;
}
