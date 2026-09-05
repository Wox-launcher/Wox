---
layout: home
title: Wox
description: "适用于 Windows、macOS 和 Linux 的原生开源启动器。"
---

<main class="wox-home wox-home-zh">
  <section class="wox-hero">
    <div class="wox-hero-copy">
      <h1><span>你的桌面，</span><span>几次按键就能直达。</span></h1>
      <p class="wox-hero-lede">打开应用、查找文件、执行命令。<br />适用于 Windows、macOS 和 Linux 的原生开源启动器。</p>
      <div class="wox-hero-actions">
        <a class="wox-button wox-button-primary" href="https://github.com/Wox-launcher/Wox/releases">下载 Wox</a>
        <a class="wox-button" href="https://www.reddit.com/r/WoxLauncher/"><span class="wox-button-icon wox-reddit-icon" aria-hidden="true"></span><span>Reddit</span></a>
        <a class="wox-button" href="https://github.com/Wox-launcher/Wox"><svg class="wox-button-icon" aria-hidden="true" viewBox="0 0 16 16"><path fill="currentColor" d="M8 0C3.58 0 0 3.67 0 8.2c0 3.63 2.29 6.7 5.47 7.79.4.08.55-.18.55-.4 0-.19-.01-.84-.01-1.52-2.01.38-2.53-.5-2.69-.96-.09-.24-.48-.96-.82-1.16-.28-.15-.68-.52-.01-.53.63-.01 1.08.6 1.23.84.72 1.24 1.87.89 2.33.68.07-.53.28-.89.51-1.09-1.78-.21-3.64-.91-3.64-4.05 0-.89.31-1.63.82-2.2-.08-.21-.36-1.05.08-2.17 0 0 .67-.22 2.2.84A7.43 7.43 0 0 1 8 3.99c.68 0 1.36.09 2 .28 1.53-1.06 2.2-.84 2.2-.84.44 1.12.16 1.96.08 2.17.51.57.82 1.3.82 2.2 0 3.15-1.87 3.84-3.65 4.05.29.26.54.75.54 1.52 0 1.09-.01 1.98-.01 2.25 0 .22.15.48.55.4A8.14 8.14 0 0 0 16 8.2C16 3.67 12.42 0 8 0Z"/></svg><span>GitHub</span></a>
        <a class="wox-button" href="https://discord.gg/NnahFAwm3"><svg class="wox-button-icon" aria-hidden="true" viewBox="0 0 16 16"><path fill="currentColor" d="M13.545 2.907a13.2 13.2 0 0 0-3.257-1.011.05.05 0 0 0-.052.025c-.141.25-.297.577-.406.833a12.2 12.2 0 0 0-3.658 0 8 8 0 0 0-.412-.833.05.05 0 0 0-.052-.025c-1.125.194-2.22.534-3.257 1.011a.04.04 0 0 0-.021.018C.356 6.024-.213 9.047.066 12.032q.003.022.021.037a13.3 13.3 0 0 0 3.995 2.02.05.05 0 0 0 .056-.019q.463-.63.818-1.329a.05.05 0 0 0-.01-.059q-.325-.247-.625-.532a.03.03 0 0 1 0-.041l.208-.165a.05.05 0 0 1 .052-.007 10 10 0 0 0 8.63 0 .05.05 0 0 1 .053.007q.104.09.207.165a.03.03 0 0 1 .001.041q-.3.285-.624.532a.05.05 0 0 0-.01.059q.36.698.819 1.329a.05.05 0 0 0 .056.019 13.2 13.2 0 0 0 4.001-2.02.05.05 0 0 0 .021-.037c.334-3.451-.559-6.449-2.366-9.106a.03.03 0 0 0-.02-.019m-8.198 7.307c-.789 0-1.438-.724-1.438-1.612s.637-1.613 1.438-1.613c.807 0 1.45.73 1.438 1.613 0 .888-.637 1.612-1.438 1.612m5.316 0c-.788 0-1.438-.724-1.438-1.612s.637-1.613 1.438-1.613c.807 0 1.45.73 1.438 1.613 0 .888-.631 1.612-1.438 1.612"/></svg><span>Discord</span></a>
      </div>
      <p class="wox-hero-note">免费，开源。</p>
    </div>
    <figure class="wox-hero-poster">
      <img src="/images/hero-glass-dark.png" alt="Wox 深色玻璃主题，展示项目搜索和键盘操作" fetchpriority="high" />
    </figure>
  </section>

  <section class="wox-section wox-section-compact">
    <div class="wox-section-heading">
      <h2>日常操作，一个入口。</h2>
    </div>
    <div class="wox-feature-grid">
      <article class="wox-feature-card">
        <h3>快速找到并打开</h3>
        <p>输入名称，打开应用、文件或最近的项目。</p>
      </article>
      <article class="wox-feature-card">
        <h3>用键盘完成下一步</h3>
        <p>复制结果、打开所在文件夹，或从操作面板执行更多命令。</p>
      </article>
      <article class="wox-feature-card">
        <h3>安装插件，扩展功能</h3>
        <p>使用社区插件，或用 Python 和 Node.js 编写自己的插件。</p>
      </article>
    </div>
  </section>

  <SystemPluginCarousel />

  <ThemeShowcase />

  <section class="wox-section wox-split-section">
    <div>
      <p class="wox-home-label">插件开发</p>
      <h2>把自己的工具接入 Wox。</h2>
      <p>用 Python、Node.js 或脚本添加命令，让 Wox 搜索项目、调用服务，或执行常用操作。</p>
      <a class="wox-button" href="./development/plugins/overview">开发插件</a>
    </div>
    <figure class="wox-feature-shot wox-feature-shot-plugin">
      <img src="/images/plugin_setting.png" alt="Wox 插件设置" loading="lazy" />
    </figure>
  </section>

  <section class="wox-section wox-closing">
    <h2>开始使用 Wox。</h2>
    <p>Windows · macOS · Linux</p>
    <div class="wox-hero-actions">
      <a class="wox-button wox-button-primary" href="https://github.com/Wox-launcher/Wox/releases">下载 Wox</a>
      <a class="wox-button" href="./guide/installation">安装指南</a>
    </div>
    <figure class="wox-feature-shot wox-closing-shot">
      <img src="/images/confetti.png" alt="Wox 已准备就绪：完成设置后的彩纸庆祝画面" width="4186" height="2450" loading="lazy" />
    </figure>
  </section>
</main>
