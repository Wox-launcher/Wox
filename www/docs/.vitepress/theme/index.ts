import DefaultTheme from "vitepress/theme";
import PluginGallery from "./components/PluginGallery.vue";
import ThemeGallery from "./components/ThemeGallery.vue";
import "./style.css";

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component("PluginGallery", PluginGallery);
    app.component("ThemeGallery", ThemeGallery);
  },
};
