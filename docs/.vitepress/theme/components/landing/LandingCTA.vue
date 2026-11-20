<script setup lang="ts">
import { inject, type Ref } from 'vue'
import { withBase } from 'vitepress'
import type { Lang } from '../../../data/downloads'
import { createT } from '../../../data/landing-i18n'

const lang = inject<Ref<Lang>>('landingLang')!
const t = (k: string) => createT(lang.value)(k)
</script>

<template>
  <section class="cta" data-reveal>
    <div class="landing-container cta-inner">
      <h2 class="cta-title">{{ t('cta.title') }}</h2>
      <p class="cta-subtitle">{{ t('cta.subtitle') }}</p>
      <div class="cta-actions">
        <a class="btn btn-primary" :href="withBase(lang === 'en' ? '/en/quick-start' : '/quick-start')">{{ t('cta.primary') }}</a>
        <a class="btn btn-ghost" href="https://github.com/songloft-org/songloft" target="_blank" rel="noreferrer">
          ★ {{ t('cta.github') }}
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.cta {
  padding: 84px 0;
  margin-top: 40px;
}
.cta-inner {
  text-align: center;
  position: relative;
  border-radius: 24px;
  padding: 56px 24px;
  background:
    radial-gradient(70% 120% at 50% 0%, var(--vp-c-brand-soft), transparent 70%),
    var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
}
.cta-title {
  margin: 0;
  font-size: clamp(1.6rem, 3.4vw, 2.4rem);
  font-weight: 800;
  letter-spacing: -0.02em;
  border: none;
  padding: 0;
}
.cta-subtitle {
  margin: 14px 0 0;
  color: var(--vp-c-text-2);
  font-size: 1.08rem;
}
.cta-actions {
  margin-top: 28px;
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}
</style>
