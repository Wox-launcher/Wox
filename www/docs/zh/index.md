---
layout: home
title: Wox
description: "适用于 Windows、macOS 和 Linux 的快速、开放、插件驱动启动器。"
---

<!-- The homepage uses custom sections instead of the default VitePress hero because the default template made Wox look generic; plain HTML keeps the page easy to edit while giving the product a more grounded first impression. -->
<main class="wox-home wox-home-zh">
  <section class="wox-hero">
    <div class="wox-hero-copy">
      <p class="wox-home-label"><span class="wox-logo-mark">W</span><span>开源快速启动器</span></p>
      <h1><span>启动和扩展。</span><span>按你的方式。</span></h1>
      <p class="wox-hero-lede">Wox 用来打开应用、查找文件、执行系统动作，并通过插件把常用流程集中到一个键盘入口里。</p>
      <div class="wox-hero-actions">
        <a class="wox-button wox-button-primary" href="./guide/introduction"><span class="wox-button-icon" aria-hidden="true">→</span><span>快速开始</span></a>
        <a class="wox-button" href="./store/plugins"><span class="wox-button-icon" aria-hidden="true">⌘</span><span>插件</span></a>
        <a class="wox-button wox-button-subtle" href="https://github.com/Wox-launcher/Wox"><svg class="wox-button-icon" aria-hidden="true" viewBox="0 0 16 16"><path fill="currentColor" d="M8 0C3.58 0 0 3.67 0 8.2c0 3.63 2.29 6.7 5.47 7.79.4.08.55-.18.55-.4 0-.19-.01-.84-.01-1.52-2.01.38-2.53-.5-2.69-.96-.09-.24-.48-.96-.82-1.16-.28-.15-.68-.52-.01-.53.63-.01 1.08.6 1.23.84.72 1.24 1.87.89 2.33.68.07-.53.28-.89.51-1.09-1.78-.21-3.64-.91-3.64-4.05 0-.89.31-1.63.82-2.2-.08-.21-.36-1.05.08-2.17 0 0 .67-.22 2.2.84A7.43 7.43 0 0 1 8 3.99c.68 0 1.36.09 2 .28 1.53-1.06 2.2-.84 2.2-.84.44 1.12.16 1.96.08 2.17.51.57.82 1.3.82 2.2 0 3.15-1.87 3.84-3.65 4.05.29.26.54.75.54 1.52 0 1.09-.01 1.98-.01 2.25 0 .22.15.48.55.4A8.14 8.14 0 0 0 16 8.2C16 3.67 12.42 0 8 0Z"/></svg><span>GitHub</span></a>
        <!-- Keep community destinations together so visitors can choose between source code and discussion without scanning elsewhere on the page. -->
        <a class="wox-button wox-button-subtle" href="https://www.reddit.com/r/WoxLauncher/"><span class="wox-button-icon" aria-hidden="true">r/</span><span>Reddit</span></a>
      </div>
    </div>
    <figure class="wox-hero-poster">
      <img src="/images/poster.png" alt="Wox 启动器展示项目、应用、插件和操作结果" />
    </figure>
  </section>

  <section class="wox-section wox-section-compact">
    <div class="wox-section-heading">
      <h2>为日常工作流而做</h2>
      <p>Wox 靠近键盘，把常用入口和下一步动作放在一个命令窗口里。</p>
    </div>
    <div class="wox-feature-grid">
      <article class="wox-feature-card">
        <span class="wox-feature-index">01</span>
        <h3>快速找到并打开</h3>
        <p>启动应用、打开文件夹、跳转最近项目、搜索本地文件，不需要在窗口之间来回切换。</p>
      </article>
      <article class="wox-feature-card">
        <span class="wox-feature-index">02</span>
        <h3>对结果继续操作</h3>
        <p>通过结果动作完成打开、复制、显示位置、执行快捷命令，把任务留在启动器里结束。</p>
      </article>
      <article class="wox-feature-card">
        <span class="wox-feature-index">03</span>
        <h3>通过插件继续扩展</h3>
        <p>安装社区插件，或用 Node.js、Python、脚本插件和 Wox API 做自己的工作流。</p>
      </article>
    </div>
  </section>

  <SystemPluginCarousel />

  <ThemeShowcase />

  <section class="wox-section wox-split-section">
    <div>
      <p class="wox-home-label">核心体验</p>
      <h2>搜索结果不止能打开，还能继续做事</h2>
      <p>Wox 不是一个简单输入框。结果可以带图标、副标题、操作面板、快捷键、上下文和插件提供的流程，让常用任务在启动器内完成。</p>
      <ul class="wox-check-list">
        <li>应用、文件、书签、网页搜索、剪贴板、计算器、单位转换等内置工作流。</li>
        <li>通过操作面板继续执行二级命令，尽量不离开键盘。</li>
        <li>主题支持可以让启动器和你的桌面风格保持一致。</li>
      </ul>
    </div>
    <figure class="wox-feature-shot">
      <img src="/images/search_result_and_action_panel.png" alt="Wox 截图插件结果和已打开的操作面板" />
      <figcaption>搜索结果和操作面板</figcaption>
    </figure>
  </section>

  <section class="wox-section wox-split-section wox-split-section-reverse">
    <div>
      <p class="wox-home-label">插件平台</p>
      <h2>先用内置能力，再按自己的流程继续扩展</h2>
      <p>Wox 保持启动器本体简洁，把具体工作交给插件：项目入口、系统命令、网页工具、AI 辅助、内部面板，或者任何脚本可以触达的事情。</p>
      <div class="wox-mini-grid">
        <span><span class="wox-mini-icon wox-mini-icon-node" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M12 2 21 7v10l-9 5-9-5V7l9-5Z" /><path d="M8.2 15.4V8.8h1.7l2.5 3.8V8.8h1.7v6.6h-1.7l-2.5-3.8v3.8H8.2Z" /></svg></span>Node.js SDK</span>
        <span><span class="wox-mini-icon wox-mini-icon-python" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M12.1 3.2c-3.7 0-4.4 1.6-4.4 3.3v1.4h4.5v.9H5.9c-1.7 0-3.2 1-3.2 4s1.3 4 3 4h1.2v-1.7c0-1.9 1.6-3.5 3.5-3.5h4.4c1.2 0 2.2-1 2.2-2.2V6.5c0-1.7-1.4-3.3-4.9-3.3Zm-2.5 2a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8Z" /><path d="M11.9 20.8c3.7 0 4.4-1.6 4.4-3.3v-1.4h-4.5v-.9h6.3c1.7 0 3.2-1 3.2-4s-1.3-4-3-4h-1.2v1.7c0 1.9-1.6 3.5-3.5 3.5H9.2c-1.2 0-2.2 1-2.2 2.2v2.9c0 1.7 1.4 3.3 4.9 3.3Zm2.5-2a.9.9 0 1 1 0-1.8.9.9 0 0 1 0 1.8Z" /></svg></span>Python SDK</span>
        <span><span class="wox-mini-icon wox-mini-icon-script" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M4 5.5h16v13H4v-13Z" /><path d="m8 9 2.4 2.5L8 14" /><path d="M12.4 14h4" /></svg></span>脚本插件</span>
        <span><span class="wox-mini-icon wox-mini-icon-store" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M6 9h12l-1 11H7L6 9Z" /><path d="M9 9V7a3 3 0 0 1 6 0v2" /><path d="M9.2 14.2h5.6" /><path d="M12 11.4v5.6" /></svg></span>插件商店</span>
      </div>
    </div>
    <figure class="wox-feature-shot">
      <img src="/images/plugin_setting.png" alt="Wox 插件设置里的已安装插件详情页" />
      <figcaption>插件设置和商店详情</figcaption>
    </figure>
  </section>

  <section class="wox-section wox-closing">
    <h2>把 Wox 调成你的命令中心</h2>
    <p>安装 Wox，接上你需要的插件，把启动器调整到适合自己的工作方式。</p>
    <div class="wox-hero-actions">
      <a class="wox-button wox-button-primary" href="./guide/installation"><span class="wox-button-icon" aria-hidden="true">→</span><span>安装 Wox</span></a>
      <a class="wox-button" href="./store/plugins"><span>查找插件</span></a>
    </div>
  </section>
</main>
