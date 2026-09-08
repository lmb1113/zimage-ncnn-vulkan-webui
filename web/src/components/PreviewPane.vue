<script setup>
import { computed, ref, watch } from 'vue'
import api from '../api'
import { store, activeJob, openLightbox } from '../store'

const idx = ref(0)

const job = computed(
  () => activeJob.value || store.jobs.find((j) => j.outputs && j.outputs.length) || null
)

const outputs = computed(() => {
  const j = job.value
  if (!j || !j.outputs) return []
  return j.outputs.map((n) => ({ name: n, url: api.mediaUrl(n) }))
})

const current = computed(() => outputs.value[Math.min(idx.value, outputs.value.length - 1)] || null)

watch(outputs, (v) => {
  if (idx.value >= v.length) idx.value = 0
})

const statusText = computed(() => {
  const j = job.value
  if (!j) return '等待任务'
  const map = {
    queued: '排队中',
    running: '正在生成',
    success: '生成完成',
    failed: '生成失败',
    canceled: '已取消',
  }
  const label = map[j.status] || j.status
  if (j.status === 'running' || j.status === 'success') {
    return j.total > 1 ? `${label} · ${j.total} 张` : label
  }
  return label
})

const progress = computed(() => (job.value ? job.value.progress : 0))

const metaText = computed(() => {
  const j = job.value
  if (!j) return ''
  const p = j.params || {}
  const bits = []
  bits.push(`${p.width || 1024}×${p.height || 1024}`)
  if (p.seed >= 0) bits.push(`seed ${p.seed}`)
  if (j.elapsedMs) bits.push(`${(j.elapsedMs / 1000).toFixed(1)}s`)
  return bits.join(' · ')
})
</script>

<template>
  <section class="preview card">
    <div v-if="job && (job.status === 'running' || job.status === 'queued')" class="bar-wrap">
      <div class="bar">
        <div class="fill" :style="{ width: progress + '%' }" />
      </div>
    </div>

    <div v-if="current" class="stage">
      <img
        class="main-img clickable"
        :src="current.url"
        :alt="current.name"
        title="点击放大"
        @click="openLightbox(current.url)"
      />
      <div v-if="outputs.length > 1" class="strip">
        <button
          v-for="(o, i) in outputs"
          :key="o.name"
          class="thumb"
          :class="{ on: i === idx }"
          @click="idx = i"
        >
          <img :src="o.url" :alt="o.name" />
        </button>
      </div>
    </div>

    <div v-else class="empty">
      <svg width="34" height="34" viewBox="0 0 24 24" fill="none">
        <rect x="2" y="4" width="20" height="16" rx="3" stroke="#C9CCD1" stroke-width="1.6" />
        <circle cx="8.5" cy="10" r="1.6" fill="#C9CCD1" />
        <path
          d="M4 18.5l5-4.6 3.8 3.6 3-2.6 4.2 3.6"
          stroke="#C9CCD1"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
      <span>{{ statusText }} · 生成结果将显示在这里</span>
      <span v-if="job && job.error" class="err">{{ job.error }}</span>
    </div>

    <div class="foot">
      <span class="status">{{ statusText }}</span>
      <span class="meta">{{ metaText || (current ? current.name : '') }}</span>
      <a v-if="current" class="dl" :href="current.url" download>保存图片</a>
    </div>
  </section>
</template>

<style scoped>
.preview {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 20px;
  gap: 16px;
}
.bar-wrap {
  flex: none;
}
.bar {
  height: 6px;
  background: #eef0f3;
  border-radius: 999px;
  overflow: hidden;
}
.fill {
  height: 100%;
  background: var(--ink);
  border-radius: 999px;
  transition: width 0.4s ease;
}
.stage {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
}
.main-img {
  max-width: 100%;
  max-height: 100%;
  border-radius: 10px;
  object-fit: contain;
  background: var(--tint);
}
.main-img.clickable {
  cursor: zoom-in;
}
.dl {
  font-size: 12px;
  color: var(--text-2);
  text-decoration: none;
  padding: 5px 10px;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-sm);
  white-space: nowrap;
}
.dl:hover {
  background: var(--tint-2);
}
.strip {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.thumb {
  width: 88px;
  height: 88px;
  border-radius: 8px;
  overflow: hidden;
  border: 2px solid transparent;
  padding: 0;
  background: var(--tint);
}
.thumb.on {
  border-color: var(--ink);
}
.thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.foot {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}
.status {
  color: var(--text-2);
  font-weight: 500;
}
.err {
  color: var(--danger);
  max-width: 420px;
  word-break: break-all;
}
</style>
