<script setup>
import { computed, ref, watch, onMounted } from 'vue'
import api from '../api'
import { store, refreshGallery, selectImage, removeImage, notify, openLightbox, reuseImage } from '../store'

const keyword = ref('')
const filter = ref('all')

// 分页渲染：网格每次只挂 60 张，避免大图库 DOM 过重
const PAGE = 60
const visibleCount = ref(PAGE)
watch([keyword, filter], () => {
  visibleCount.value = PAGE
})

const modeLabels = {
  txt2img: '文生图',
  inpaint: '局部重绘',
  outpaint: '画布扩图',
  controlnet: '图生图 / 控制生成',
  tile: '图片放大',
}

const counts = computed(() => {
  const c = { all: store.gallery.length }
  for (const k of Object.keys(modeLabels)) c[k] = 0
  for (const it of store.gallery) {
    if (it.mode && c[it.mode] !== undefined) c[it.mode]++
  }
  return c
})

const filters = computed(() => [
  { key: 'all', label: '全部', n: counts.value.all },
  ...Object.keys(modeLabels).map((k) => ({
    key: k,
    label: modeLabels[k],
    n: counts.value[k] || 0,
  })),
])

const list = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return store.gallery.filter((it) => {
    if (filter.value !== 'all' && it.mode !== filter.value) return false
    if (!kw) return true
    return (
      (it.prompt || '').toLowerCase().includes(kw) ||
      String(it.seed || '').includes(kw) ||
      it.name.toLowerCase().includes(kw)
    )
  })
})

const visibleList = computed(() => list.value.slice(0, visibleCount.value))

// 网格用服务端缩略图（宽 >480 的图），点开详情/灯箱仍是原图
function thumbUrl(it) {
  return it.width > 480 ? '/media/thumb/' + encodeURIComponent(it.name) : it.url
}

function fmtSize(b) {
  if (!b) return '—'
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(0)} KB`
  return `${(b / 1024 / 1024).toFixed(1)} MB`
}

function fmtElapsed(ms) {
  if (!ms) return ''
  return `${(ms / 1000).toFixed(1)}s`
}

// 打包下载当前筛选结果（ZIP）
function downloadZip() {
  const names = list.value.map((i) => i.name).join(',')
  const url = '/api/gallery/download' + (names ? '?names=' + encodeURIComponent(names) : '')
  const a = document.createElement('a')
  a.href = url
  a.download = ''
  document.body.appendChild(a)
  a.click()
  a.remove()
}

async function openFolder() {
  try {
    await api.openPath('')
  } catch (e) {
    notify(e.message, 'error')
  }
}

function reuse() {
  const it = store.selected
  if (!it) return
  store.pendingParams = {
    prompt: it.prompt || '',
    negative: it.negative || '',
    width: it.width || 1024,
    height: it.height || 1024,
    steps: it.steps || 0,
    seed: it.seed >= 0 ? it.seed : -1,
    batch: 1,
  }
  store.view = 'generate'
  notify('参数已带回到生成工作台')
}

// 一键联动：共用 store.reuseImage（图库 / 预览 / 灯箱同源）
function useAsImg2img() {
  reuseImage(store.selected, 'img2img')
}

function useAsInpaint() {
  reuseImage(store.selected, 'inpaint')
}

onMounted(() => {
  if (!store.selected && store.gallery.length) selectImage(store.gallery[0].name)
})
</script>

<template>
  <div class="view">
    <div class="main">
      <div class="toolbar">
        <div class="filters">
          <button
            v-for="f in filters"
            :key="f.key"
            class="chip"
            :class="{ on: filter === f.key }"
            @click="filter = f.key"
          >
            {{ f.label }} {{ f.n }}
          </button>
        </div>
        <div class="tools">
          <input v-model="keyword" class="search" placeholder="搜索提示词或种子" />
          <button class="btn btn-sm btn-ghost" @click="refreshGallery">刷新</button>
          <button class="btn btn-sm btn-ghost" @click="openFolder">打开目录</button>
          <button class="btn btn-sm btn-ghost" title="把当前筛选结果打包为 ZIP" @click="downloadZip">下载 ZIP</button>
        </div>
      </div>

      <div v-if="list.length" class="grid">
        <button
          v-for="it in visibleList"
          :key="it.name"
          class="card item"
          :class="{ on: store.selected && store.selected.name === it.name }"
          @click="selectImage(it.name)"
        >
          <img :src="thumbUrl(it)" :alt="it.name" loading="lazy" />
          <div class="meta">
            <span class="t">{{ it.prompt || it.name }}</span>
            <span class="s">
              {{ it.width }}×{{ it.height }} · {{ fmtElapsed(it.elapsedMs) || '—' }} ·
              {{ fmtSize(it.size) }}
            </span>
          </div>
        </button>
      </div>

      <div v-if="list.length > visibleList.length" class="more-wrap">
        <button class="btn btn-ghost" @click="visibleCount += PAGE">
          加载更多（还有 {{ list.length - visibleList.length }} 张）
        </button>
      </div>

      <div v-if="!list.length" class="empty">
        <span>还没有生成记录</span>
        <span class="sub">回到「生成工作台」跑第一张图吧</span>
      </div>
    </div>

    <aside class="detail">
      <template v-if="store.selected">
        <span class="label">图片详情</span>
        <img
          class="thumb"
          :src="thumbUrl(store.selected)"
          :alt="store.selected.name"
          title="点击放大"
          @click="openLightbox(store.selected.url, list.map((i) => i.url))"
        />
        <span class="name">{{ store.selected.prompt || store.selected.name }}</span>
        <span class="sub">
          {{ store.selected.modTime }} · {{ store.selected.width }}×{{
            store.selected.height
          }}
          · {{ fmtSize(store.selected.size) }}
        </span>

        <div class="kv">
          <div><span>生成模式</span><b>{{ modeLabels[store.selected.mode] || '未知' }}</b></div>
          <div>
            <span>输出尺寸</span><b>{{ store.selected.width }} × {{ store.selected.height }}</b>
          </div>
          <div><span>随机种子</span><b>{{ store.selected.seed >= 0 ? store.selected.seed : '随机' }}</b></div>
          <div><span>生成耗时</span><b>{{ fmtElapsed(store.selected.elapsedMs) || '—' }}</b></div>
        </div>

        <div class="prompt-box">{{ store.selected.prompt || '（无提示词记录）' }}</div>

        <button class="btn btn-primary block" @click="openFolder">打开所在文件夹</button>
        <button class="btn btn-ghost block" @click="reuse">复用参数重新生成</button>
        <div class="link-row">
          <button class="btn btn-ghost" @click="useAsImg2img">作为图生图参考</button>
          <button class="btn btn-ghost" @click="useAsInpaint">作为重绘输入</button>
        </div>
        <button class="btn btn-danger block" @click="removeImage(store.selected.name)">
          删除这张图片
        </button>
      </template>
      <div v-else class="empty">
        <span>未选择图片</span>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  height: 100%;
  min-height: 0;
}
.main {
  flex: 1;
  min-width: 0;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.filters {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.chip {
  padding: 8px 12px;
  border-radius: var(--r-md);
  font-size: 13px;
  background: var(--tint);
  color: var(--text-2);
}
.chip.on {
  background: var(--ink);
  color: #fff;
  font-weight: 500;
}
.tools {
  display: flex;
  gap: 8px;
}
.search {
  width: 240px;
  height: 34px;
}
.grid {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 16px;
  align-content: start;
  padding-right: 4px;
}
.item {
  padding: 0;
  overflow: hidden;
  text-align: left;
  transition: border-color 0.15s, transform 0.15s;
}
.item:hover {
  transform: translateY(-2px);
}
.item.on {
  border-color: var(--ink);
  box-shadow: 0 0 0 1px var(--ink);
}
.item img {
  width: 100%;
  aspect-ratio: 4 / 3;
  object-fit: cover;
  display: block;
  background: var(--tint);
}
.meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px 12px;
}
.meta .t {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta .s {
  font-size: 11px;
  color: var(--muted);
}
.detail {
  width: 360px;
  flex: none;
  background: #fff;
  border-left: 1px solid var(--border);
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow-y: auto;
}
/* 窄窗口时详情栏移到列表下方 */
@media (max-width: 920px) {
  .view {
    flex-direction: column;
    overflow-y: auto;
  }
  .main {
    padding: 16px;
  }
  .grid {
    overflow: visible;
  }
  .detail {
    width: 100%;
    border-left: none;
    border-top: 1px solid var(--border);
  }
}
.label {
  font-size: 13px;
  color: var(--text-3);
}
.thumb {
  width: 100%;
  border-radius: 10px;
  background: var(--tint);
  cursor: zoom-in;
}
.name {
  font-size: 14px;
  font-weight: 600;
  line-height: 1.5;
}
.sub {
  font-size: 11px;
  color: var(--muted);
}
.kv {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 0;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}
.kv div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}
.kv span {
  color: var(--text-3);
}
.kv b {
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.prompt-box {
  background: var(--tint-2);
  border-radius: var(--r-md);
  padding: 12px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--text-2);
  max-height: 140px;
  overflow-y: auto;
}
.block {
  width: 100%;
}
.sub {
  display: block;
}
.more-wrap {
  flex: none;
  display: flex;
  justify-content: center;
  padding: 4px 0 8px;
}
.link-row {
  display: flex;
  gap: 8px;
}
.link-row .btn {
  flex: 1;
  white-space: nowrap;
}
</style>
