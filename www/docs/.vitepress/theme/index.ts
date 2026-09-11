import DefaultTheme from "vitepress/theme";
import PluginDetailPage from "./components/PluginDetailPage.vue";
import PluginGallery from "./components/PluginGallery.vue";
import SystemPluginCarousel from "./components/SystemPluginCarousel.vue";
import ThemeShowcase from "./components/ThemeShowcase.vue";
import ThemeGallery from "./components/ThemeGallery.vue";
import HomeHeroNote from "./components/HomeHeroNote.vue";
import HomeQuotes from "./components/HomeQuotes.vue";
import ReleaseStamp from "./components/ReleaseStamp.vue";
import "./style.css";

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component("PluginDetailPage", PluginDetailPage);
    app.component("PluginGallery", PluginGallery);
    app.component("SystemPluginCarousel", SystemPluginCarousel);
    app.component("ThemeShowcase", ThemeShowcase);
    app.component("ThemeGallery", ThemeGallery);
    app.component("HomeHeroNote", HomeHeroNote);
    app.component("HomeQuotes", HomeQuotes);
    app.component("ReleaseStamp", ReleaseStamp);
  },
};
