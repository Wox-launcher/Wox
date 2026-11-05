import themes from "../../../../store-theme.json";

export default {
  paths: () => themes.map((theme) => ({ params: { id: theme.Id } })),
};
