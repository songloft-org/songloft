<script setup lang="ts">
import { inject, type Ref } from 'vue'
import { withBase } from 'vitepress'
import type { Lang } from '../../../data/downloads'
import { createT } from '../../../data/landing-i18n'
import BrowserFrame from './BrowserFrame.vue'

const lang = inject<Ref<Lang>>('landingLang')!
const t = (k: string) => createT(lang.value)(k)

const badges = [
  { alt: 'GitHub stars', src: 'https://img.shields.io/github/stars/songloft-org/songloft?style=flat&color=E8792B' },
  { alt: 'Release', src: 'https://img.shields.io/github/v/release/songloft-org/songloft?color=E8792B' },
  { alt: 'Docker pulls', src: 'https://img.shields.io/docker/pulls/songloft/songloft?color=E8792B' },
  { alt: 'License', src: 'https://img.shields.io/github/license/songloft-org/songloft?color=E8792B' },
]
</script>

<template>
  <section class="hero" data-reveal>
    <div class="hero-glow" aria-hidden="true" />
    <div class="landing-container hero-inner">
      <div class="hero-copy">
        <span class="hero-badge">{{ t('hero.badge') }}</span>
        <h1 class="hero-title">{{ t('hero.title') }}</h1>
        <p class="hero-subtitle">{{ t('hero.subtitle') }}</p>
        <div class="hero-actions">
          <a class="btn btn-primary" :href="withBase(lang === 'en' ? '/en/quick-start' : '/quick-start')">{{ t('hero.cta.primary') }}</a>
          <a class="btn btn-ghost" href="#install">{{ t('hero.cta.secondary') }}</a>
          <a class="btn btn-ghost" href="https://github.com/songloft-org/songloft" target="_blank" rel="noreferrer">
            {{ t('hero.cta.github') }}
          </a>
        </div>
        <div class="hero-badges">
          <img v-for="b in badges" :key="b.alt" :src="b.src" :alt="b.alt" loading="lazy" />
        </div>
        <p class="hero-note">{{ t('hero.note') }}</p>
      </div>
      <div class="hero-shot">
        <BrowserFrame src="/screenshots/home-desktop.png" url="songloft · 我的音乐" :eager="true" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero {
  position: relative;
  overflow: hidden;
  padding: 72px 0 40px;
}
.hero-glow {
  position: absolute;
  inset: -30% 0 auto 0;
  height: 120%;
  background:
    radial-gradient(60% 50% at 20% 10%, rgba(232, 121, 43, 0.22), transparent 70%),
    radial-gradient(50% 40% at 85% 0%, rgba(245, 166, 35, 0.18), transparent 70%);
  filter: blur(10px);
  z-index: 0;
  pointer-events: none;
}
.hero-inner {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.05fr);
  gap: 48px;
  align-items: center;
}
.hero-badge {
  display: inline-block;
  font-size: 13px;
  font-weight: 600;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  padding: 6px 14px;
  border-radius: 999px;
}
.hero-title {
  margin: 20px 0 0;
  font-size: clamp(2.4rem, 5vw, 3.6rem);
  line-height: 1.08;
  font-weight: 800;
  letter-spacing: -0.02em;
  background: linear-gradient(120deg, var(--vp-c-brand-1) 20%, #f5a623);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.hero-subtitle {
  margin: 20px 0 0;
  font-size: clamp(1rem, 1.4vw, 1.18rem);
  line-height: 1.7;
  color: var(--vp-c-text-2);
  max-width: 34em;
}
.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 30px;
}
.hero-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 26px;
}
.hero-badges img {
  height: 20px;
}
.hero-note {
  margin-top: 18px;
  font-size: 12.5px;
  color: var(--vp-c-text-3);
  max-width: 40em;
}
.hero-shot {
  position: relative;
}
@media (max-width: 880px) {
  .hero-inner {
    grid-template-columns: 1fr;
    gap: 36px;
  }
  .hero { padding-top: 48px; }
}
</style>
