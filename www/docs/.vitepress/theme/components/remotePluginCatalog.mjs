// Read the same catalog as Wox; failures leave the pre-rendered snapshot in place.
export async function fetchRemotePluginCatalog(fetchImpl = fetch) {
  const response = await fetchImpl("https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-plugin.json");
  if (!response.ok) throw new Error(`Plugin catalog: HTTP ${response.status}`);
  const data = await response.json();
  if (!Array.isArray(data) || !data.every((item) => item && typeof item.Id === "string" && typeof item.Name === "string")) {
    throw new Error("Invalid plugin catalog");
  }
  return data;
}
