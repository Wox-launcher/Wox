---
layout: home
title: Wox
description: "A fast, open, plugin-driven launcher for Windows, macOS, and Linux."
---

<!-- The homepage uses custom sections instead of the default VitePress hero because the default template made Wox look generic; plain HTML keeps the page easy to edit while giving the product a more grounded first impression. -->
<main class="wox-home">
  <section class="wox-hero">
    <div class="wox-hero-copy">
      <p class="wox-home-label"><span class="wox-logo-mark">W</span><span>Open source launcher</span></p>
      <h1><span>Launch. Extend.</span><span class="wox-nowrap">Make it yours.</span></h1>
      <p class="wox-hero-lede">Wox is a fast desktop launcher for opening apps, finding files, running system actions, and building workflows with plugins.</p>
      <div class="wox-hero-actions">
        <a class="wox-button wox-button-primary" href="./guide/introduction"><span class="wox-button-icon" aria-hidden="true">→</span><span>Get Started</span></a>
        <a class="wox-button" href="./store/plugins"><span class="wox-button-icon" aria-hidden="true">⌘</span><span>Plugins</span></a>
        <a class="wox-button wox-button-subtle" href="https://github.com/Wox-launcher/Wox"><svg class="wox-button-icon" aria-hidden="true" viewBox="0 0 16 16"><path fill="currentColor" d="M8 0C3.58 0 0 3.67 0 8.2c0 3.63 2.29 6.7 5.47 7.79.4.08.55-.18.55-.4 0-.19-.01-.84-.01-1.52-2.01.38-2.53-.5-2.69-.96-.09-.24-.48-.96-.82-1.16-.28-.15-.68-.52-.01-.53.63-.01 1.08.6 1.23.84.72 1.24 1.87.89 2.33.68.07-.53.28-.89.51-1.09-1.78-.21-3.64-.91-3.64-4.05 0-.89.31-1.63.82-2.2-.08-.21-.36-1.05.08-2.17 0 0 .67-.22 2.2.84A7.43 7.43 0 0 1 8 3.99c.68 0 1.36.09 2 .28 1.53-1.06 2.2-.84 2.2-.84.44 1.12.16 1.96.08 2.17.51.57.82 1.3.82 2.2 0 3.15-1.87 3.84-3.65 4.05.29.26.54.75.54 1.52 0 1.09-.01 1.98-.01 2.25 0 .22.15.48.55.4A8.14 8.14 0 0 0 16 8.2C16 3.67 12.42 0 8 0Z"/></svg><span>GitHub</span></a>
        <!-- Keep community destinations together so visitors can choose between source code and discussion without scanning elsewhere on the page. -->
        <a class="wox-button wox-button-subtle" href="https://www.reddit.com/r/WoxLauncher/"><span class="wox-button-icon" aria-hidden="true">r/</span><span>Reddit</span></a>
      </div>
    </div>
    <figure class="wox-hero-poster">
      <img src="/images/poster.png" alt="Wox launcher showing project, app, plugin, and action results" />
    </figure>
  </section>

  <section class="wox-section wox-section-compact">
    <div class="wox-section-heading">
      <h2>Built for everyday flow</h2>
      <p>Wox stays close to the keyboard and keeps the important actions one command away.</p>
    </div>
    <div class="wox-feature-grid">
      <article class="wox-feature-card">
        <span class="wox-feature-index">01</span>
        <h3>Find and launch quickly</h3>
        <p>Start apps, open folders, jump to recent projects, and search local files without switching context.</p>
      </article>
      <article class="wox-feature-card">
        <span class="wox-feature-index">02</span>
        <h3>Act on every result</h3>
        <p>Use result actions for opening, copying, revealing, running shortcuts, and sending work to the right tool.</p>
      </article>
      <article class="wox-feature-card">
        <span class="wox-feature-index">03</span>
        <h3>Grow with plugins</h3>
        <p>Install community plugins or build your own with Node.js, Python, script plugins, and the Wox API.</p>
      </article>
    </div>
  </section>

  <SystemPluginCarousel />

  <ThemeShowcase />

  <section class="wox-section wox-split-section">
    <div>
      <p class="wox-home-label">Core experience</p>
      <h2>Search results that stay useful after you find them</h2>
      <p>Wox is not just a text box. Results can carry icons, subtitles, action panels, hotkeys, context, and plugin-provided workflows so common tasks finish inside the launcher.</p>
      <ul class="wox-check-list">
        <li>Application, file, bookmark, web search, clipboard, calculator, and converter workflows.</li>
        <li>Action panel for secondary commands without leaving the keyboard.</li>
        <li>Theme support for matching the launcher with your desktop.</li>
      </ul>
    </div>
    <figure class="wox-feature-shot">
      <img src="/images/search_result_and_action_panel.png" alt="Wox screenshot plugin results with the action panel open" />
      <figcaption>Search results and action panel</figcaption>
    </figure>
  </section>

  <section class="wox-section wox-split-section wox-split-section-reverse">
    <div>
      <p class="wox-home-label">Plugin platform</p>
      <h2>Use the built-ins, then shape the rest around your own workflow</h2>
      <p>Wox keeps the launcher small and lets plugins add the parts that are specific to your work: project shortcuts, system commands, web tools, AI helpers, internal dashboards, or anything reachable from a script.</p>
      <div class="wox-mini-grid">
        <span><span class="wox-mini-icon wox-mini-icon-node" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M12 2 21 7v10l-9 5-9-5V7l9-5Z" /><path d="M8.2 15.4V8.8h1.7l2.5 3.8V8.8h1.7v6.6h-1.7l-2.5-3.8v3.8H8.2Z" /></svg></span>Node.js SDK</span>
        <span><span class="wox-mini-icon wox-mini-icon-python" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M12.1 3.2c-3.7 0-4.4 1.6-4.4 3.3v1.4h4.5v.9H5.9c-1.7 0-3.2 1-3.2 4s1.3 4 3 4h1.2v-1.7c0-1.9 1.6-3.5 3.5-3.5h4.4c1.2 0 2.2-1 2.2-2.2V6.5c0-1.7-1.4-3.3-4.9-3.3Zm-2.5 2a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8Z" /><path d="M11.9 20.8c3.7 0 4.4-1.6 4.4-3.3v-1.4h-4.5v-.9h6.3c1.7 0 3.2-1 3.2-4s-1.3-4-3-4h-1.2v1.7c0 1.9-1.6 3.5-3.5 3.5H9.2c-1.2 0-2.2 1-2.2 2.2v2.9c0 1.7 1.4 3.3 4.9 3.3Zm2.5-2a.9.9 0 1 1 0-1.8.9.9 0 0 1 0 1.8Z" /></svg></span>Python SDK</span>
        <span><span class="wox-mini-icon wox-mini-icon-script" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M4 5.5h16v13H4v-13Z" /><path d="m8 9 2.4 2.5L8 14" /><path d="M12.4 14h4" /></svg></span>Script plugins</span>
        <span><span class="wox-mini-icon wox-mini-icon-store" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M6 9h12l-1 11H7L6 9Z" /><path d="M9 9V7a3 3 0 0 1 6 0v2" /><path d="M9.2 14.2h5.6" /><path d="M12 11.4v5.6" /></svg></span>Plugin Store</span>
      </div>
    </div>
    <figure class="wox-feature-shot">
      <img src="/images/plugin_setting.png" alt="Wox plugin settings showing an installed plugin detail page" />
      <figcaption>Plugin settings and store-ready details</figcaption>
    </figure>
  </section>

  <section class="wox-section wox-closing">
    <h2>Ready to make Wox your command center?</h2>
    <p>Install Wox, connect the plugins you need, and keep your launcher tuned to the way you work.</p>
    <div class="wox-hero-actions">
      <a class="wox-button wox-button-primary" href="./guide/installation"><span class="wox-button-icon" aria-hidden="true">→</span><span>Install Wox</span></a>
      <a class="wox-button" href="./store/plugins"><span>Find a Plugin</span></a>
    </div>
  </section>
</main>
