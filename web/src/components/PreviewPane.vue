<script setup>
import { computed, ref, watch } from 'vue'
import api from '../api'
import { store, activeJob, openLightbox, reuseImage } from '../store'

const idx = ref(0)

const job = computed(
  () => activeJob.value || store.jobs.find((j) => j.outputs && j.outputs.length) || null
)

const outputs = computed(() => {
  const j = job.value
  if (!j || !j.outputs) return []
  // 图库刷新后过滤掉已被删除的输出，避免预览破图
  const names = store.gallery.length ? new Set(store.gallery.map((g) => g.name)) : null
  return j.outputs
    .filter((n) => !names || names.has(n))
    .map((n) => ({ name: n, url: api.mediaUrl(n) }))
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

// 任务成功后，从图库条目取到服务端路径，支持一键联动
const galleryItem = computed(() => {
  if (!current.value || !job.value || job.value.status !== 'success') return null
  return store.gallery.find((g) => g.name === current.value.name) || null
})

// 对比模式：重绘/图生图/控制生成任务可拖动分割线对比原图与结果
const compareWith = computed(() => {
  const j = job.value
  if (!j || j.status !== 'success' || !current.value) return ''
  const p = j.params || {}
  return p.inputImage || p.controlImage || ''
})
const showCompare = ref(false)
const cmpPct = ref(50)
const cmpEl = ref(null)
let cmpDown = false

// 服务端路径 → 可访问 URL（uploads 与 out 分别由 /uploads、/media 提供）
function pathToUrl(p) {
  if (!p) return ''
  const name = p.split(/[\\/]/).pop()
  return /[\\/]out[\\/]/.test(p) ? '/media/' + encodeURIComponent(name) : '/uploads/' + encodeURIComponent(name)
}

function cmpMove(e) {
  if (!cmpDown || !cmpEl.value) return
  const r = cmpEl.value.getBoundingClientRect()
  let pct = ((e.clientX - r.left) / r.width) * 100
  cmpPct.value = Math.max(0, Math.min(100, pct))
}

function toggleCompare() {
  showCompare.value = !showCompare.value
  cmpPct.value = 50
}

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
      <div
        v-if="showCompare && compareWith"
        ref="cmpEl"
        class="cmp clickable"
        title="拖动分割线对比"
        @pointerdown="cmpDown = true"
        @pointermove="cmpMove"
        @pointerup="cmpDown = false"
        @pointerleave="cmpDown = false"
      >
        <img class="cmp-img" :src="pathToUrl(compareWith)" alt="原图" />
        <img
          class="cmp-img cmp-top"
          :src="current.url"
          :alt="current.name"
          :style="{ clipPath: 'inset(0 0 0 ' + cmpPct + '%)' }"
        />
        <div class="cmp-line" :style="{ left: cmpPct + '%' }" />
        <span class="cmp-tag left">原图</span>
        <span class="cmp-tag right">结果</span>
      </div>
      <img
        v-else
        class="main-img clickable"
        :src="current.url"
        :alt="current.name"
        title="点击放大"
        @click="openLightbox(current.url, outputs.map((o) => o.url))"
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
      <div class="foot-left">
        <span class="status">{{ statusText }}</span>
        <span class="meta">{{ metaText || (current ? current.name : '') }}</span>
      </div>
      <div class="foot-right">
        <button v-if="galleryItem" class="qa" title="以这张图为参考生成新画面" @click="reuseImage(galleryItem, 'img2img')">图生图</button>
        <button v-if="galleryItem" class="qa" title="在这张图上局部重绘" @click="reuseImage(galleryItem, 'inpaint')">重绘</button>
        <button v-if="compareWith" class="qa" :class="{ on: showCompare }" @click="toggleCompare">对比</button>
        <a v-if="current" class="dl" :href="current.url" download>保存图片</a>
      </div>
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
.foot-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.foot-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}
.qa {
  font-size: 12px;
  color: var(--text-2);
  padding: 5px 10px;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-sm);
  background: #fff;
  white-space: nowrap;
}
.qa:hover {
  border-color: var(--ink);
  color: var(--text);
}
.qa.on {
  background: var(--ink);
  border-color: var(--ink);
  color: #fff;
}
.cmp {
  position: relative;
  max-width: 100%;
  max-height: 100%;
  border-radius: 10px;
  overflow: hidden;
  background: var(--tint);
  touch-action: none;
  cursor: ew-resize;
  line-height: 0;
}
.cmp-img {
  display: block;
  width: 100%;
  height: auto;
  max-height: 100%;
  object-fit: contain;
}
.cmp-top {
  position: absolute;
  inset: 0;
  height: 100%;
  object-fit: contain;
}
.cmp-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  background: #fff;
  box-shadow: 0 0 6px rgba(0, 0, 0, 0.5);
}
.cmp-tag {
  position: absolute;
  bottom: 10px;
  font-size: 11px;
  color: #fff;
  background: rgba(0, 0, 0, 0.45);
  border-radius: 4px;
  padding: 2px 8px;
  line-height: 1.4;
}
.cmp-tag.left {
  left: 10px;
}
.cmp-tag.right {
  right: 10px;
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
