<script setup lang="ts">
import { onMounted } from "vue";
import { useData, withBase } from "vitepress";

const { frontmatter } = useData();

// Meta refresh covers direct visits without JavaScript; this also handles
// VitePress client navigation to a legacy URL without adding a history entry.
onMounted(() => window.location.replace(withBase(frontmatter.value.redirect)));
</script>

<template>
  <a :href="withBase(frontmatter.redirect)"><slot /></a>
</template>
