<script setup>
import { onMounted, computed } from 'vue'
import { store, bootstrap } from './store'
import GenerateView from './views/GenerateView.vue'
import GalleryView from './views/GalleryView.vue'
import SettingsView from './views/SettingsView.vue'
import Lightbox from './components/Lightbox.vue'

const navs = [
  { key: 'generate', label: '生成工作台' },
  { key: 'gallery', label: '图库' },
  { key: 'settings', label: '设置' },
]

const engine = computed(() => {
  if (!store.system) return { text: '检测中…', cls: '' }
  if (!store.system.exeFound) return { text: '引擎缺失', cls: 'pill-err' }
  const running = store.jobs.find((j) => j.status === 'running')
  if (running) return { text: '生成中', cls: 'pill-ok' }
  return { text: '就绪 · 空闲', cls: 'pill-ok' }
})

const REPO_URL = 'https://github.com/lmb1113/zimage-ncnn-vulkan-webui'
const version = computed(() => (store.system && store.system.version) || '')

onMounted(bootstrap)
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <svg width="26" height="26" viewBox="0 0 24 24" fill="none">
          <rect x="1" y="3" width="22" height="18" rx="5" fill="#14161A" />
          <circle cx="8.8" cy="9.4" r="1.9" fill="#FFFFFF" />
          <path
            d="M3 18.2l5.2-5 3.9 3.8 3.1-2.6 5.1 4"
            stroke="#FFFFFF"
            stroke-width="1.6"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
        <span class="name">Z-Image 工作台</span>
        <span class="tag">本地</span>
      </div>

      <nav class="nav">
        <button
          v-for="n in navs"
          :key="n.key"
          class="nav-item"
          :class="{ active: store.view === n.key }"
          @click="store.view = n.key"
        >
          {{ n.label }}
        </button>
      </nav>

      <div class="right">
        <span class="pill" :class="engine.cls">
          <i class="dot" :class="{ ok: engine.cls === 'pill-ok' }" />{{ engine.text }}
        </span>
        <span v-if="version" class="ver" :title="'当前版本 ' + version">{{ version }}</span>
        <a
          class="gh"
          :href="REPO_URL"
          target="_blank"
          rel="noopener"
          title="GitHub 仓库"
        >
          <svg width="20" height="20" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path
              d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z"
            />
          </svg>
        </a>
      </div>
    </header>

    <main class="main">
      <GenerateView v-if="store.view === 'generate'" />
      <GalleryView v-else-if="store.view === 'gallery'" />
      <SettingsView v-else />
    </main>

    <transition name="fade">
      <div v-if="store.toast" class="toast" :class="store.toast.kind">
        {{ store.toast.message }}
      </div>
    </transition>

    <Lightbox />
  </div>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.topbar {
  height: 56px;
  flex: none;
  background: #fff;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  gap: 16px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}
.brand .name {
  font-size: 15px;
  font-weight: 600;
}
.brand .tag {
  font-size: 11px;
  color: var(--text-3);
  background: var(--tint);
  border-radius: var(--r-sm);
  padding: 2px 6px;
}
.nav {
  display: flex;
  gap: 4px;
  background: var(--tint);
  padding: 4px;
  border-radius: 10px;
}
.nav-item {
  padding: 6px 14px;
  border-radius: 7px;
  font-size: 13px;
  color: var(--text-3);
  transition: background 0.15s, color 0.15s;
}
.nav-item.active {
  background: #fff;
  color: var(--text);
  font-weight: 500;
}
.right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #c9ccd1;
  display: inline-block;
}
.dot.ok {
  background: var(--success);
}
.ver {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-3);
  background: var(--tint);
  border-radius: 999px;
  padding: 3px 10px;
  font-variant-numeric: tabular-nums;
}
.gh {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  color: var(--text-3);
  transition: background 0.15s, color 0.15s;
}
.gh:hover {
  background: var(--tint);
  color: var(--text);
}
.main {
  flex: 1;
  min-height: 0;
}
.toast {
  position: fixed;
  left: 50%;
  bottom: 32px;
  transform: translateX(-50%);
  background: var(--ink);
  color: #fff;
  padding: 10px 18px;
  border-radius: var(--r-md);
  font-size: 13px;
  box-shadow: 0 8px 24px rgba(16, 24, 40, 0.18);
  z-index: 100;
}
.toast.error {
  background: var(--danger);
}
.toast.success {
  background: var(--success);
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
/* 窄窗口时收紧顶栏 */
@media (max-width: 720px) {
  .topbar {
    padding: 0 10px;
    gap: 8px;
  }
  .brand .tag {
    display: none;
  }
  .nav-item {
    padding: 6px 10px;
  }
}
</style>
