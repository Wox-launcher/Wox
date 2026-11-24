<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from "vue";
import { useData } from "vitepress";

const installUrl = "https://raw.githubusercontent.com/Wox-launcher/Wox/master/scripts/install";
const releasesUrl = "https://github.com/Wox-launcher/Wox/releases";
const platforms = {
  windows: { label: "Windows", shell: "PowerShell", command: `irm ${installUrl}/windows.ps1 | iex` },
  macos: { label: "macOS", shell: "Terminal", command: `curl -q -fsSL --proto-redir '=https' ${installUrl}/macos.sh | bash` },
  linux: { label: "Linux", shell: "Terminal", command: `curl -q -fsSL --proto-redir '=https' ${installUrl}/linux.sh | bash` },
};
type Platform = keyof typeof platforms;

const { lang } = useData();
const isZh = computed(() => lang.value.toLowerCase().startsWith("zh"));
const copyLabel = computed(() => isZh.value ? "复制安装命令" : "Copy install command");
const panelId = `${useId()}-install-options`;
const expanded = ref(false);
const platform = ref<Platform>("windows");
const selected = computed(() => platforms[platform.value]);
const copyState = ref<"idle" | "copied" | "failed">("idle");
const commandText = ref<HTMLElement>();

const help = computed(() => {
  const shell = isZh.value && selected.value.shell === "Terminal" ? "终端" : selected.value.shell;
  if (copyState.value === "copied") {
    return isZh.value ? `已复制，粘贴到 ${shell} 运行。` : `Copied. Paste into ${shell} to run.`;
  }
  if (copyState.value === "failed") {
    return isZh.value ? "请手动复制命令。" : "Copy the command manually.";
  }
  return isZh.value ? `在 ${shell} 中运行。` : `Run in ${shell}.`;
});

onMounted(() => {
  const browser = navigator as Navigator & { userAgentData?: { platform?: string } };
  const { userAgent } = browser;
  if (/Android|CrOS|iPhone|iPad|iPod/i.test(userAgent) ||
      (/Macintosh/i.test(userAgent) && browser.maxTouchPoints > 1)) {
    return;
  }

  const system = browser.userAgentData?.platform || userAgent;
  if (/Windows/i.test(system)) platform.value = "windows";
  else if (/Mac/i.test(system)) platform.value = "macos";
  else if (/Linux/i.test(system)) platform.value = "linux";
});

watch([platform, expanded], () => {
  copyState.value = "idle";
});

async function copyCommand() {
  const { command } = selected.value;
  try {
    await navigator.clipboard.writeText(command);
    if (expanded.value && command === selected.value.command) copyState.value = "copied";
  } catch {
    if (!expanded.value || command !== selected.value.command || !commandText.value) return;
    copyState.value = "failed";
    commandText.value.focus();
    const range = document.createRange();
    range.selectNodeContents(commandText.value);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
  }
}
</script>

<template>
  <div class="wox-hero-actions">
    <button
      type="button"
      class="wox-button wox-button-primary install-toggle"
      :aria-controls="panelId"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      {{ isZh ? "安装 Wox" : "Install Wox" }}
    </button>
    <slot />
  </div>
  <section
    v-show="expanded"
    :id="panelId"
    class="wox-home-install"
    :aria-label="isZh ? '安装方式' : 'Installation options'"
  >
    <fieldset class="platforms" :aria-label="isZh ? '操作系统' : 'Operating system'">
      <label v-for="(option, id) in platforms" :key="id" class="platform">
        <input v-model="platform" type="radio" :name="panelId" :value="id" />
        <span>{{ option.label }}</span>
      </label>
    </fieldset>
    <div class="command">
      <code ref="commandText" tabindex="-1">{{ selected.command }}</code>
      <button
        type="button"
        class="copy-command"
        :class="{ copied: copyState === 'copied' }"
        :aria-label="copyLabel"
        :title="copyLabel"
        @click="copyCommand"
      ></button>
    </div>
    <div class="install-footer">
      <p class="help" role="status">{{ help }}</p>
      <a class="manual-download" :href="releasesUrl">
        {{ isZh ? "从 GitHub 手动下载" : "Manual download on GitHub" }}
      </a>
    </div>
  </section>
</template>

<style scoped>
.install-toggle {
  cursor: pointer;
}

.install-toggle::after {
  content: "";
  width: 7px;
  height: 7px;
  margin: -3px 0 0 3px;
  border: solid currentColor;
  border-width: 0 1.5px 1.5px 0;
  transform: rotate(45deg);
}

.install-toggle[aria-expanded="true"]::after {
  margin-top: 3px;
  transform: rotate(225deg);
}

.wox-home-install {
  max-width: 680px;
  margin: 12px auto 0;
  padding: 12px;
  border: 1px solid var(--wox-home-border);
  border-radius: 10px;
  background: var(--wox-home-panel);
  text-align: left;
}

.platforms {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  min-inline-size: 0;
  margin: 0;
  padding: 0;
  border: 0;
  border-bottom: 1px solid var(--wox-home-border);
}

.platform {
  position: relative;
}

.platform input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}

.platform span {
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  padding: 0 5px;
  border-bottom: 2px solid transparent;
  color: var(--wox-home-text-soft);
  font-size: 13px;
  font-weight: 600;
}

.platform input:checked + span {
  border-color: var(--wox-home-accent);
  color: var(--vp-c-text-1);
}

.platform input:focus-visible + span,
.command code:focus-visible {
  outline: 2px solid var(--vp-c-text-1);
  outline-offset: 3px;
}

.command {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
  gap: 8px;
  margin-top: 8px;
  padding: 6px 6px 6px 12px;
  border: 1px solid var(--wox-home-border);
  border-radius: 8px;
  background: var(--wox-home-panel-soft);
  text-align: left;
}

.command code {
  padding: 6px 0;
  background: transparent;
  color: var(--vp-c-text-1);
  font: 13px/1.5 var(--vp-font-family-mono);
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.copy-command {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border-radius: 6px;
  color: var(--wox-home-text-soft);
  cursor: pointer;
}

.copy-command::before {
  content: "";
  width: 16px;
  height: 16px;
  background: currentColor;
  mask: var(--vp-icon-copy) center / contain no-repeat;
}

.copy-command.copied::before {
  mask-image: var(--vp-icon-copied);
}

.copy-command:hover {
  background: var(--wox-home-panel);
  color: var(--vp-c-text-1);
}

.platform:hover input:not(:checked) + span {
  color: var(--vp-c-text-1);
}

.install-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 2px 16px;
  margin-top: 4px;
  font-size: 13px;
  line-height: 1.5;
}

.wox-home-install p.help {
  margin: 0;
  color: var(--wox-home-text-soft);
}

a.manual-download {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  color: var(--wox-home-text-soft);
  font-weight: 500;
  text-decoration: none;
}

a.manual-download:hover,
a.manual-download:focus-visible {
  color: var(--vp-c-text-1);
  text-decoration: underline;
  text-underline-offset: 3px;
}

@media (pointer: coarse) {
  .platform span,
  a.manual-download {
    min-height: 44px;
  }

  .copy-command {
    width: 44px;
    height: 44px;
  }
}
</style>
