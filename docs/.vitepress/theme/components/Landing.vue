<script setup lang="ts">
import { computed, provide } from 'vue'
import { useData } from 'vitepress'
import type { Lang } from '../../data/downloads'
import { createT } from '../../data/landing-i18n'
import { FEATURES } from '../../data/features'
import { useReveal } from '../composables/useReveal'

import LandingHero from './landing/LandingHero.vue'
import LandingTrustBar from './landing/LandingTrustBar.vue'
import LandingFeatureRow from './landing/LandingFeatureRow.vue'
import LandingScreenshots from './landing/LandingScreenshots.vue'
import LandingInstaller from './landing/LandingInstaller.vue'
import LandingPlugins from './landing/LandingPlugins.vue'
import LandingCompliance from './landing/LandingCompliance.vue'
import LandingNotice from './landing/LandingNotice.vue'
import LandingCTA from './landing/LandingCTA.vue'

const { lang } = useData()
const landingLang = computed<Lang>(() => (lang.value?.toLowerCase().startsWith('en') ? 'en' : 'zh'))
provide('landingLang', landingLang)

const t = (k: string) => createT(landingLang.value)(k)

useReveal()
</script>

<template>
  <div class="landing-root">
    <LandingHero />
    <LandingTrustBar />

    <section class="features" data-reveal>
      <div class="landing-container features-head">
        <p class="section-eyebrow">{{ t('features.eyebrow') }}</p>
        <h2 class="section-title">{{ t('features.title') }}</h2>
        <p class="section-subtitle">{{ t('features.subtitle') }}</p>
      </div>
      <div class="landing-container">
        <LandingFeatureRow v-for="f in FEATURES" :key="f.id" :feature="f" />
      </div>
    </section>

    <LandingScreenshots />
    <LandingInstaller />
    <LandingNotice />
    <LandingPlugins />
    <LandingCompliance />
    <LandingCTA />
  </div>
</template>

<style scoped>
.features { padding: 64px 0 24px; }
.features-head { text-align: center; margin-bottom: 8px; }
</style>
