<script setup lang="ts">
import { computed, inject, reactive, ref, type Ref } from 'vue'
import { withBase } from 'vitepress'
import { INSTALL, pick, type InstallMethod, type Lang } from '../../../data/downloads'
import { createT } from '../../../data/landing-i18n'
import CodeBlock from './CodeBlock.vue'

const lang = inject<Ref<Lang>>('landingLang')!
const t = (k: string) => createT(lang.value)(k)

const methodId = ref(INSTALL[0].id)
// 每个方法各自记住选中的 edition / OS
const editionId = reactive<Record<string, string>>({})
const osId = reactive<Record<string, string>>({})

const method = computed<InstallMethod>(() => INSTALL.find((m) => m.id === methodId.value)!)

const currentEdition = computed(() => {
  const m = method.value
  if (!m.editions) return undefined
  const id = editionId[m.id] ?? m.editions[0].id
  return m.editions.find((e) => e.id === id) ?? m.editions[0]
})

// 当前可选的 OS 分组（download 类：来自 edition.groups 或 method.groups）
const groups = computed(() => currentEdition.value?.groups ?? method.value.groups ?? [])

const currentGroup = computed(() => {
  const g = groups.value
  if (!g.length) return undefined
  const key = `${methodId.value}:${currentEdition.value?.id ?? '-'}`
  const id = osId[key] ?? g[0].os
  return g.find((x) => x.os === id) ?? g[0]
})

function selectOS(os: string) {
  osId[`${methodId.value}:${currentEdition.value?.id ?? '-'}`] = os
}
</script>

<template>
  <section id="install" class="install" data-reveal>
    <div class="landing-container">
      <p class="section-eyebrow">{{ t('install.eyebrow') }}</p>
      <h2 class="section-title">{{ t('install.title') }}</h2>
      <p class="section-subtitle">{{ t('install.subtitle') }}</p>

      <div class="install-security">⚠️ {{ t('install.security') }}</div>

      <div class="install-card">
        <!-- 一级：安装方式 -->
        <div class="method-tabs" role="tablist">
          <button
            v-for="m in INSTALL"
            :key="m.id"
            class="method-tab"
            :class="{ active: m.id === methodId }"
            type="button"
            @click="methodId = m.id"
          >
            <span class="method-label">{{ pick(m.label, lang) }}</span>
            <span class="method-tagline">{{ pick(m.tagline, lang) }}</span>
          </button>
        </div>

        <div class="method-body">
          <!-- 下载类：可能有 edition + OS 分组 -->
          <template v-if="method.kind === 'download'">
            <div v-if="method.editions" class="pill-row">
              <span class="pill-label">{{ t('install.edition') }}</span>
              <button
                v-for="e in method.editions"
                :key="e.id"
                class="pill"
                :class="{ active: (editionId[method.id] ?? method.editions[0].id) === e.id }"
                type="button"
                @click="editionId[method.id] = e.id"
              >
                {{ pick(e.label, lang) }}
              </button>
              <span v-if="currentEdition" class="edition-desc">{{ pick(currentEdition.desc, lang) }}</span>
            </div>

            <div class="pill-row">
              <span class="pill-label">{{ t('install.platform') }}</span>
              <button
                v-for="g in groups"
                :key="g.os"
                class="pill"
                :class="{ active: currentGroup?.os === g.os }"
                type="button"
                @click="selectOS(g.os)"
              >
                {{ pick(g.osLabel, lang) }}
              </button>
            </div>

            <div v-if="currentGroup" class="asset-grid">
              <a
                v-for="asset in currentGroup.assets"
                :key="asset.file"
                class="asset"
                :href="asset.url"
              >
                <div class="asset-arch">{{ pick(asset.archLabel, lang) }}</div>
                <div class="asset-file">{{ asset.file }}</div>
                <span class="asset-dl">↓ {{ t('install.download') }}</span>
              </a>
            </div>
          </template>

          <!-- 命令类：Docker（按 group 分隔为不同方式） -->
          <template v-else-if="method.kind === 'command'">
            <div class="cmd-list">
              <template v-for="(c, i) in method.commands" :key="i">
                <div
                  v-if="c.group && pick(c.group, lang) !== (method.commands[i - 1] && method.commands[i - 1].group ? pick(method.commands[i - 1].group!, lang) : '')"
                  class="cmd-group"
                  :class="{ 'cmd-group--first': i === 0 }"
                >
                  {{ pick(c.group, lang) }}
                </div>
                <CodeBlock :title="pick(c.title, lang)" :code="c.code" />
              </template>
            </div>
          </template>

          <!-- 外链类：Flutter / Kodi（可附带命令块，如 Flutter 的 Scoop 安装） -->
          <template v-else>
            <div class="ext-row">
              <a
                v-for="(e, i) in method.external"
                :key="i"
                class="btn"
                :class="e.primary ? 'btn-primary' : 'btn-ghost'"
                :href="typeof e.url === 'string' ? e.url : withBase(pick(e.url, lang))"
                :target="typeof e.url === 'string' ? '_blank' : undefined"
                :rel="typeof e.url === 'string' ? 'noreferrer' : undefined"
              >
                {{ pick(e.label, lang) }}<span v-if="typeof e.url === 'string'"> ↗</span>
              </a>
            </div>
            <div v-if="method.commands" class="cmd-list">
              <template v-for="(c, i) in method.commands" :key="i">
                <div
                  v-if="c.group && pick(c.group, lang) !== (method.commands[i - 1] && method.commands[i - 1].group ? pick(method.commands[i - 1].group!, lang) : '')"
                  class="cmd-group"
                  :class="{ 'cmd-group--first': i === 0 }"
                >
                  {{ pick(c.group, lang) }}
                </div>
                <CodeBlock :title="pick(c.title, lang)" :code="c.code" />
              </template>
            </div>
          </template>

          <p v-if="method.note" class="method-note">
            {{ pick(method.note, lang) }}
            <a
              v-if="method.noteLink"
              :href="typeof method.noteLink.url === 'string' ? method.noteLink.url : withBase(pick(method.noteLink.url, lang))"
            >{{ pick(method.noteLink.label, lang) }}</a>
          </p>
        </div>
      </div>

      <p class="install-docs">
        {{ t('install.docsHint') }}
        <a :href="withBase(lang === 'en' ? '/en/quick-start' : '/quick-start')">{{ t('install.docsLink') }}</a>
      </p>
    </div>
  </section>
</template>

<style scoped>
.install { padding: 72px 0; }
.install-security {
  margin: 22px auto 0;
  max-width: 900px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--vp-c-text-2);
  background: var(--vp-c-warning-soft, rgba(232, 121, 43, 0.1));
  border: 1px solid var(--vp-c-brand-soft);
  border-radius: 10px;
  padding: 12px 16px;
}
.install-card {
  margin: 28px auto 0;
  max-width: 980px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 16px;
  background: var(--vp-c-bg-soft);
  overflow: hidden;
}
.method-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  padding: 8px;
  background: var(--vp-c-bg-alt);
  border-bottom: 1px solid var(--vp-c-divider);
}
.method-tab {
  flex: 1 1 auto;
  min-width: 130px;
  text-align: left;
  padding: 10px 14px;
  border-radius: 10px;
  background: transparent;
  transition: background 0.18s;
}
.method-tab:hover { background: var(--vp-c-bg-soft); }
.method-tab.active { background: var(--vp-c-brand-soft); }
.method-label {
  display: block;
  font-weight: 700;
  font-size: 15px;
  color: var(--vp-c-text-1);
}
.method-tab.active .method-label { color: var(--vp-c-brand-1); }
.method-tagline {
  display: block;
  font-size: 12px;
  color: var(--vp-c-text-3);
  margin-top: 2px;
}
.method-body { padding: 22px 22px 24px; }
.pill-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}
.pill-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--vp-c-text-3);
  margin-right: 4px;
}
.pill {
  font-size: 14px;
  font-weight: 600;
  padding: 6px 14px;
  border-radius: 999px;
  border: 1px solid var(--vp-c-divider);
  background: var(--vp-c-bg);
  color: var(--vp-c-text-2);
  transition: all 0.16s;
}
.pill:hover { border-color: var(--vp-c-brand-1); }
.pill.active {
  background: var(--vp-c-brand-1);
  border-color: var(--vp-c-brand-1);
  color: #fff;
}
.edition-desc {
  font-size: 13px;
  color: var(--vp-c-text-3);
  margin-left: 4px;
}
.asset-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
  margin-top: 6px;
}
.asset {
  display: block;
  padding: 14px 16px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  background: var(--vp-c-bg);
  transition: all 0.16s;
}
.asset:hover {
  border-color: var(--vp-c-brand-1);
  transform: translateY(-2px);
  box-shadow: 0 10px 24px -16px rgba(0, 0, 0, 0.4);
}
.asset-arch { font-weight: 700; color: var(--vp-c-text-1); }
.asset-file {
  font-size: 12px;
  color: var(--vp-c-text-3);
  font-family: var(--vp-font-family-mono);
  margin: 4px 0 10px;
  word-break: break-all;
}
.asset-dl { font-size: 13px; font-weight: 600; color: var(--vp-c-brand-1); }
.cmd-list { display: flex; flex-direction: column; gap: 14px; }
.cmd-group {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 20px;
  font-size: 14px;
  font-weight: 700;
  color: var(--vp-c-brand-1);
}
.cmd-group.cmd-group--first { margin-top: 0; }
.cmd-group::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--vp-c-divider);
}
.ext-row { display: flex; flex-wrap: wrap; gap: 12px; }
.method-note {
  margin-top: 18px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--vp-c-text-3);
  padding: 12px 14px;
  border-radius: 10px;
  background: var(--vp-c-bg-alt);
}
.method-note a { color: var(--vp-c-brand-1); font-weight: 600; }
.install-docs {
  text-align: center;
  margin-top: 22px;
  font-size: 14px;
  color: var(--vp-c-text-2);
}
.install-docs a { color: var(--vp-c-brand-1); font-weight: 600; }
</style>
