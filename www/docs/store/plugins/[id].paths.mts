import plugins from "../../../../store-plugin.json";

export default {
  paths: () => plugins.map((plugin) => ({ params: { id: plugin.Id } })),
};
