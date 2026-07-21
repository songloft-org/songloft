<script setup lang="ts">
import { inject, type Ref } from 'vue'
import type { Lang } from '../../../data/downloads'
import { createT } from '../../../data/landing-i18n'

const lang = inject<Ref<Lang>>('landingLang')!
const t = (k: string) => createT(lang.value)(k)

const items = ['notice.i1', 'notice.i2', 'notice.i3', 'notice.i4']
</script>

<template>
  <section class="notice" data-reveal>
    <div class="landing-container">
      <div class="notice-card">
        <p class="notice-badge">{{ t('notice.badge') }}</p>
        <h2 class="notice-title">{{ t('notice.title') }}</h2>
        <p class="notice-intro">{{ t('notice.intro') }}</p>
        <ul class="notice-list">
          <li v-for="k in items" :key="k">{{ t(k) }}</li>
        </ul>
        <p class="notice-channel">{{ t('notice.channel') }}</p>
        <p class="notice-links">
          <a href="https://github.com/songloft-org/songloft" target="_blank" rel="noopener">{{ t('notice.github') }}</a>
          ·
          <a href="https://songloft.hanxi.cc" target="_blank" rel="noopener">{{ t('notice.docs') }}</a>
          ·
          <a href="https://github.com/songloft-org/songloft/issues/2" target="_blank" rel="noopener">{{ t('notice.wechat') }}</a>
        </p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.notice { padding: 32px 0 12px; }
.notice-card {
  border: 1px solid var(--vp-c-divider);
  border-left: 4px solid var(--vp-c-green-1, #10b981);
  border-radius: 14px;
  background: var(--vp-c-bg-soft);
  padding: 28px 30px;
}
.notice-badge {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--vp-c-green-1, #10b981);
}
.notice-title {
  margin: 8px 0 0;
  font-size: 1.4rem;
  font-weight: 800;
  border: none;
  padding: 0;
}
.notice-intro {
  margin: 12px 0 0;
  color: var(--vp-c-text-2);
  line-height: 1.7;
}
.notice-list {
  margin: 16px 0 0;
  padding-left: 20px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 24px;
  color: var(--vp-c-text-1);
  line-height: 1.6;
}
.notice-channel {
  margin: 18px 0 0;
  color: var(--vp-c-text-2);
  line-height: 1.7;
  font-weight: 500;
}
.notice-links {
  margin: 12px 0 0;
  font-size: 14px;
  color: var(--vp-c-text-3);
}
.notice-links a { color: var(--vp-c-brand-1); font-weight: 600; }
@media (max-width: 720px) {
  .notice-list { grid-template-columns: 1fr; }
}
</style>
