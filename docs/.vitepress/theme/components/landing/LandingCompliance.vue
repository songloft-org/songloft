<script setup lang="ts">
import { inject, type Ref } from 'vue'
import { withBase } from 'vitepress'
import type { Lang } from '../../../data/downloads'
import { createT } from '../../../data/landing-i18n'

const lang = inject<Ref<Lang>>('landingLang')!
const t = (k: string) => createT(lang.value)(k)

const items = ['compliance.i1', 'compliance.i2', 'compliance.i3', 'compliance.i4']
</script>

<template>
  <section class="compliance" data-reveal>
    <div class="landing-container">
      <div class="compliance-card">
        <h2 class="compliance-title">⚖️ {{ t('compliance.title') }}</h2>
        <p class="compliance-intro">{{ t('compliance.intro') }}</p>
        <ul class="compliance-list">
          <li v-for="k in items" :key="k">{{ t(k) }}</li>
        </ul>
        <p class="compliance-more">
          {{ t('compliance.more') }}
          <a :href="withBase(lang === 'en' ? '/en/NOTICE' : '/NOTICE')">{{ t('compliance.notice') }}</a>
          ·
          <a :href="withBase(lang === 'en' ? '/en/PRIVACY' : '/PRIVACY')">{{ t('compliance.privacy') }}</a>
        </p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.compliance { padding: 20px 0 8px; }
.compliance-card {
  border: 1px solid var(--vp-c-divider);
  border-left: 4px solid var(--vp-c-brand-1);
  border-radius: 14px;
  background: var(--vp-c-bg-soft);
  padding: 26px 28px;
}
.compliance-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 750;
  border: none;
  padding: 0;
}
.compliance-intro {
  margin: 12px 0 0;
  color: var(--vp-c-text-2);
  line-height: 1.7;
}
.compliance-list {
  margin: 14px 0 0;
  padding-left: 20px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 24px;
  color: var(--vp-c-text-2);
  line-height: 1.6;
}
.compliance-more {
  margin: 18px 0 0;
  font-size: 14px;
  color: var(--vp-c-text-3);
}
.compliance-more a { color: var(--vp-c-brand-1); font-weight: 600; }
@media (max-width: 720px) {
  .compliance-list { grid-template-columns: 1fr; }
}
</style>
