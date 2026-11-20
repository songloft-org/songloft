<script setup lang="ts">
import { inject, type Ref } from 'vue'
import { withBase } from 'vitepress'
import type { Lang } from '../../../data/downloads'
import { createT } from '../../../data/landing-i18n'

const lang = inject<Ref<Lang>>('landingLang')!
const t = (k: string) => createT(lang.value)(k)

const links = [
  { key: 'plugins.cta.list', href: 'https://github.com/songloft-org/songloft/issues/4', external: true, icon: '🧩' },
  { key: 'plugins.cta.dev', href: '/js-plugin-development-guide', external: false, icon: '🛠️' },
  { key: 'plugins.cta.registry', href: '/plugin_registry', external: false, icon: '📦' },
]
</script>

<template>
  <section class="plugins" data-reveal>
    <div class="landing-container plugins-inner">
      <p class="section-eyebrow">{{ t('plugins.eyebrow') }}</p>
      <h2 class="section-title">{{ t('plugins.title') }}</h2>
      <p class="section-subtitle">{{ t('plugins.subtitle') }}</p>

      <div class="plugin-cards">
        <a
          v-for="l in links"
          :key="l.key"
          class="plugin-card"
          :href="l.external ? l.href : withBase(lang === 'en' ? '/en' + l.href : l.href)"
          :target="l.external ? '_blank' : undefined"
          :rel="l.external ? 'noreferrer' : undefined"
        >
          <span class="plugin-icon">{{ l.icon }}</span>
          <span class="plugin-text">{{ t(l.key) }}</span>
          <span class="plugin-arrow">→</span>
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.plugins {
  padding: 72px 0;
  background:
    radial-gradient(60% 60% at 50% 0%, var(--vp-c-brand-soft), transparent 70%);
}
.plugins-inner { text-align: center; }
.plugin-cards {
  margin-top: 34px;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
.plugin-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 20px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 14px;
  background: var(--vp-c-bg);
  text-align: left;
  transition: all 0.18s;
}
.plugin-card:hover {
  border-color: var(--vp-c-brand-1);
  transform: translateY(-3px);
  box-shadow: 0 14px 30px -18px rgba(0, 0, 0, 0.4);
}
.plugin-icon { font-size: 24px; }
.plugin-text {
  flex: 1;
  font-weight: 650;
  color: var(--vp-c-text-1);
}
.plugin-arrow {
  color: var(--vp-c-brand-1);
  font-weight: 700;
  transition: transform 0.18s;
}
.plugin-card:hover .plugin-arrow { transform: translateX(4px); }
@media (max-width: 880px) {
  .plugin-cards { grid-template-columns: 1fr; }
}
</style>
