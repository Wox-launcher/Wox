---
layout: home
title: Wox — Cross-platform launcher for Windows, macOS, Linux
titleTemplate: false
description: "A native, open-source Raycast / Alfred alternative for Windows, macOS, and Linux."
---

<main class="wox-home">
  <section class="wox-hero">
    <div class="wox-hero-copy">
      <h1><span>Your desktop,</span><span>a few keystrokes away.</span></h1>
      <p class="wox-hero-lede">Open apps, find files, and run commands with Wox.<br /> A native, open-source Raycast / Alfred alternative for Windows, macOS, and Linux.</p>
      <div class="wox-hero-actions">
        <a class="wox-button wox-button-primary" href="https://github.com/Wox-launcher/Wox/releases">Download Wox</a>
        <a class="wox-button" href="https://www.reddit.com/r/WoxLauncher/"><span class="wox-button-icon wox-reddit-icon" aria-hidden="true"></span><span>Reddit</span></a>
        <a class="wox-button" href="https://github.com/Wox-launcher/Wox"><svg class="wox-button-icon" aria-hidden="true" viewBox="0 0 16 16"><path fill="currentColor" d="M8 0C3.58 0 0 3.67 0 8.2c0 3.63 2.29 6.7 5.47 7.79.4.08.55-.18.55-.4 0-.19-.01-.84-.01-1.52-2.01.38-2.53-.5-2.69-.96-.09-.24-.48-.96-.82-1.16-.28-.15-.68-.52-.01-.53.63-.01 1.08.6 1.23.84.72 1.24 1.87.89 2.33.68.07-.53.28-.89.51-1.09-1.78-.21-3.64-.91-3.64-4.05 0-.89.31-1.63.82-2.2-.08-.21-.36-1.05.08-2.17 0 0 .67-.22 2.2.84A7.43 7.43 0 0 1 8 3.99c.68 0 1.36.09 2 .28 1.53-1.06 2.2-.84 2.2-.84.44 1.12.16 1.96.08 2.17.51.57.82 1.3.82 2.2 0 3.15-1.87 3.84-3.65 4.05.29.26.54.75.54 1.52 0 1.09-.01 1.98-.01 2.25 0 .22.15.48.55.4A8.14 8.14 0 0 0 16 8.2C16 3.67 12.42 0 8 0Z"/></svg><span>GitHub</span></a>
        <a class="wox-button" href="https://discord.gg/NnahFAwm3"><span class="wox-button-icon wox-discord-icon" aria-hidden="true"></span><span>Discord</span></a>
      </div>
      <HomeHeroNote />
    </div>
    <figure class="wox-hero-poster">
      <img src="/images/hero-glass-dark.png" alt="Wox in its dark glass theme, showing project search and keyboard actions" fetchpriority="high" />
    </figure>
  </section>

  <section class="wox-section wox-section-compact">
    <div class="wox-section-heading">
      <h2>One place for the things you do every day.</h2>
    </div>
    <div class="wox-feature-grid">
      <article class="wox-feature-card">
        <h3>Find and open</h3>
        <p>Type a name to open an app, a file, or a recent project.</p>
      </article>
      <article class="wox-feature-card">
        <h3>Keep your hands on the keyboard</h3>
        <p>Copy a result, reveal a file, or choose another command from the action panel.</p>
      </article>
      <article class="wox-feature-card">
        <h3>Add the tools you need</h3>
        <p>Install community plugins, or write your own with Python and Node.js.</p>
      </article>
    </div>
  </section>

  <SystemPluginCarousel />

  <ThemeShowcase />

  <HomeQuotes />

  <section class="wox-section wox-split-section">
    <div>
      <p class="wox-home-label">For developers</p>
      <h2>Bring your own tools.</h2>
      <p>Write a plugin in Python, Node.js, or a script. Search your projects, call a service, or run a command you use every day.</p>
      <a class="wox-button" href="./development/plugins/overview">Build a plugin</a>
    </div>
    <figure class="wox-feature-shot wox-feature-shot-plugin">
      <img src="/images/plugin_setting.png" alt="Plugin settings in Wox" loading="lazy" />
    </figure>
  </section>

  <section class="wox-section wox-closing">
    <h2>Try Wox on your desktop.</h2>
    <p>Windows · macOS · Linux</p>
    <div class="wox-hero-actions">
      <a class="wox-button wox-button-primary" href="https://github.com/Wox-launcher/Wox/releases">Download Wox</a>
      <a class="wox-button" href="./guide/installation">Installation guide</a>
    </div>
    <figure class="wox-feature-shot wox-closing-shot">
      <img src="/images/confetti.png" alt="Wox is ready: setup complete with celebratory confetti" width="4186" height="2450" loading="lazy" />
    </figure>
  </section>
</main>
