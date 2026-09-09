<script setup>
import { reactive, ref, watch, computed, onMounted, onUnmounted } from 'vue'
import api from '../api'
import { store, notify, isBusy, activeJob, ensureNotifyPermission } from '../store'
import MaskEditor from './MaskEditor.vue'

const modes = [
  { key: 'txt2img', label: '文生图' },
  { key: 'img2img', label: '图生图' },
  { key: 'inpaint', label: '局部重绘' },
  { key: 'outpaint', label: '画布扩图' },
  { key: 'controlnet', label: '控制生成' },
  { key: 'tile', label: '图片放大' },
]

const presets = [
  { label: '1024×1024', w: 1024, h: 1024 },
  { label: '768×1024', w: 768, h: 1024 },
  { label: '1024×768', w: 1024, h: 768 },
  { label: '2048×2048', w: 2048, h: 2048 },
  { label: '2048×512', w: 2048, h: 512 },
]

const params = reactive({
  prompt: '',
  negative: '',
  width: 1024,
  height: 1024,
  steps: 0,
  seed: -1,
  batch: 1,
  modelPath: '',
  gpuId: -2,
  inputImage: '',
  maskImage: '',
  outpaint: '128,128,128,128',
  controlImage: '',
  controlScale: 1.0,
  tileUpscale: false,
})

const mode = ref('txt2img')
const previews = reactive({})
const showAdvanced = ref(false)
const maskEditor = ref(null)
const inputDims = reactive({ w: 0, h: 0 })

// 环境自检中已就位的模型目录，作为模型路径的可选建议
const foundModels = computed(() =>
  ((store.system && store.system.models) || []).filter((m) => m.found)
)

// 提示词历史：取自图库元数据（去重、最新在前、最多 20 条），正向/反向通用
const historyField = ref('') // '' | 'prompt' | 'negative'
function toggleHistory(field) {
  historyField.value = historyField.value === field ? '' : field
}
// 默认设备/模型跟随设置页：初始化与设置保存后同步
watch(
  () => store.system && store.system.config,
  (c) => {
    if (!c) return
    params.gpuId = Number(c.gpuId)
    if (!params.modelPath) params.modelPath = c.modelPath || 'z-image-turbo'
  },
  { immediate: true }
)

function historyItems(field) {
  const key = field === 'negative' ? 'negative' : 'prompt'
  const seen = new Set()
  const out = []
  for (const it of store.gallery) {
    const p = (it[key] || '').trim()
    if (!p || seen.has(p)) continue
    seen.add(p)
    out.push(p)
    if (out.length >= 20) break
  }
  return out
}

function applyHistory(field, p) {
  params[field] = p
  historyField.value = ''
}

// 扩图后的最终画布：原图尺寸 + 左/上/右/下边距
const expandedSize = computed(() => {
  if (!inputDims.w || mode.value !== 'outpaint') return ''
  const m = String(params.outpaint || '')
    .split(',')
    .map((v) => parseInt(v, 10) || 0)
  const l = m[0] || 0
  const t = m[1] || 0
  const r = m[2] || 0
  const b = m[3] || 0
  return `${inputDims.w + l + r} × ${inputDims.h + t + b}`
})

watch(
  () => store.pendingParams,
  (p) => {
    if (!p) return
    // 支持联动携带目标模式与图片预览（图生图参考图 / 重绘输入图）
    const { mode: m, previews: pv, ...rest } = p
    Object.assign(params, rest)
    if (m) mode.value = m
    if (pv) {
      Object.assign(previews, pv)
      if (pv.inputImage) syncInputSize(pv.inputImage)
    }
    store.pendingParams = null
  },
  // immediate：图库页设置 pendingParams 时本组件尚未挂载，挂载后需立即消费一次
  { immediate: true }
)

function setPreset(p) {
  params.width = p.w
  params.height = p.h
}

function randomSeed() {
  params.seed = Math.floor(Math.random() * 2147483647)
}

async function uploadFile(file, key) {
  try {
    const r = await api.upload(file)
    params[key] = r.path
    previews[key] = r.url
    // 输入图 / 参考图：输出尺寸自动对齐原图，避免引擎尺寸冲突
    // Tile 放大除外——-s 是放大目标尺寸，必须由用户指定
    if (key === 'inputImage' || (key === 'controlImage' && mode.value !== 'tile')) {
      syncInputSize(r.url)
    }
    notify(`已上传 ${file.name}`, 'success')
  } catch (e) {
    notify(e.message, 'error')
  }
}

function syncInputSize(url) {
  const im = new Image()
  im.onload = () => {
    if (im.naturalWidth > 0) {
      inputDims.w = im.naturalWidth
      inputDims.h = im.naturalHeight
      params.width = im.naturalWidth
      params.height = im.naturalHeight
    }
  }
  im.src = url
}

function pickFile(key) {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/*'
  input.onchange = async () => {
    const f = input.files && input.files[0]
    if (f) await uploadFile(f, key)
  }
  input.click()
}

function onDrop(e, key) {
  const f = e.dataTransfer.files && e.dataTransfer.files[0]
  if (f && f.type.startsWith('image/')) uploadFile(f, key)
}

function onKey(e) {
  // 中文输入法组词中的 Enter（isComposing / keyCode 229）不触发提交
  if (e.isComposing || e.keyCode === 229) return
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter' && canSubmit.value) submit()
}

// Ctrl+V 粘贴剪贴板图片：按当前模式自动落到对应输入位
function onPaste(e) {
  const files = (e.clipboardData && e.clipboardData.files) || []
  const img = Array.from(files).find((f) => f.type.startsWith('image/'))
  if (!img) return
  let key = ''
  if (mode.value === 'inpaint') key = previews.inputImage ? '' : 'inputImage'
  else if (mode.value === 'outpaint') key = 'inputImage'
  else if (mode.value === 'img2img' || mode.value === 'controlnet' || mode.value === 'tile') key = 'controlImage'
  if (!key) {
    if (mode.value !== 'txt2img') notify('该输入位已有图片', 'info')
    return
  }
  e.preventDefault()
  uploadFile(img, key)
}
onMounted(() => {
  window.addEventListener('keydown', onKey)
  window.addEventListener('paste', onPaste)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('paste', onPaste)
})

const canSubmit = computed(() => !isBusy.value && params.prompt.trim() !== '')

async function submit() {
  if (!canSubmit.value) return
  // 局部重绘：先把画布涂抹导出为蒙版并上传，再提交任务
  if (mode.value === 'inpaint' && previews.inputImage && maskEditor.value) {
    if (maskEditor.value.hasStrokes()) {
      try {
        const f = await maskEditor.value.exportMask()
        const r = await api.upload(f)
        params.maskImage = r.path
      } catch (e) {
        notify('蒙版生成失败：' + e.message, 'error')
        return
      }
    } else {
      params.maskImage = ''
    }
  }
  const payload = { ...params }
  if (mode.value === 'tile') payload.tileUpscale = true
  try {
    ensureNotifyPermission() // 首次提交时征询通知权限，长任务结束后可在后台收到提醒
    // 图生图 = ControlNet 路线：引擎无原生 img2img，用 -c 参考图实现
    await api.generate(mode.value === 'img2img' ? 'controlnet' : mode.value, payload)
    notify('任务已提交')
  } catch (e) {
    notify(e.message, 'error')
  }
}

function cancel() {
  if (activeJob.value) {
    api.cancelJob(activeJob.value.id)
  }
}

function reset() {
  Object.assign(params, {
    prompt: '',
    negative: '',
    width: 1024,
    height: 1024,
    steps: 0,
    seed: -1,
    batch: 1,
    inputImage: '',
    maskImage: '',
    outpaint: '128,128,128,128',
    controlImage: '',
    controlScale: 1.0,
  })
  for (const k of Object.keys(previews)) delete previews[k]
  if (maskEditor.value) maskEditor.value.clear()
}
</script>

<template>
  <aside class="panel">
    <div class="tabs">
      <button
        v-for="m in modes"
        :key="m.key"
        class="tab"
        :class="{ active: mode === m.key }"
        @click="mode = m.key"
      >
        {{ m.label }}
      </button>
    </div>

    <div class="group">
      <div class="row-between">
        <span class="field-label">提示词</span>
        <div class="prompt-tools">
          <span class="counter">{{ params.prompt.length }} 字</span>
          <button
            v-if="historyItems('prompt').length"
            class="hist-btn"
            @click="toggleHistory('prompt')"
          >
            历史 {{ historyItems('prompt').length }}
          </button>
        </div>
      </div>
      <div v-if="historyField === 'prompt'" class="hist-panel">
        <button
          v-for="(p, i) in historyItems('prompt')"
          :key="i"
          class="hist-item"
          :title="p"
          @click="applyHistory('prompt', p)"
        >
          {{ p }}
        </button>
      </div>
      <textarea
        v-model="params.prompt"
        rows="4"
        placeholder="描述你想生成的画面，支持中英文"
      />
    </div>

    <div class="group">
      <div class="row-between">
        <span class="field-label">反向提示词</span>
        <button
          v-if="historyItems('negative').length"
          class="hist-btn"
          @click="toggleHistory('negative')"
        >
          历史 {{ historyItems('negative').length }}
        </button>
      </div>
      <div v-if="historyField === 'negative'" class="hist-panel">
        <button
          v-for="(p, i) in historyItems('negative')"
          :key="i"
          class="hist-item"
          :title="p"
          @click="applyHistory('negative', p)"
        >
          {{ p }}
        </button>
      </div>
      <textarea
        v-model="params.negative"
        rows="2"
        placeholder="低清晰度，畸变的手指，水印，文字"
      />
    </div>

    <!-- 模式相关输入 -->
    <div v-if="mode === 'inpaint'" class="group uploads">
      <button
        class="uploader"
        title="点击选择或拖入图片"
        @click="pickFile('inputImage')"
        @dragover.prevent
        @drop.prevent="onDrop($event, 'inputImage')"
      >
        <img v-if="previews.inputImage" :src="previews.inputImage" alt="" />
        <span v-else>上传输入图像（可拖入 / Ctrl+V 粘贴）</span>
      </button>
      <!-- 已有原图：内嵌手绘蒙版编辑器，替代上传遮罩 -->
      <MaskEditor
        v-if="previews.inputImage"
        ref="maskEditor"
        :src="previews.inputImage"
      />
      <button
        v-else
        class="uploader"
        title="点击选择或拖入遮罩"
        @click="pickFile('maskImage')"
        @dragover.prevent
        @drop.prevent="onDrop($event, 'maskImage')"
      >
        <span>上传遮罩图 · 白=重绘 黑=保留</span>
      </button>
    </div>

    <div v-else-if="mode === 'outpaint'" class="group">
      <button
        class="uploader"
        title="点击选择或拖入图片"
        @click="pickFile('inputImage')"
        @dragover.prevent
        @drop.prevent="onDrop($event, 'inputImage')"
      >
        <img v-if="previews.inputImage" :src="previews.inputImage" alt="" />
        <span v-else>上传输入图像（可拖入）</span>
      </button>
      <div class="row-between">
        <span class="field-label">扩展 左,上,右,下</span>
        <span v-if="expandedSize" class="value">扩展后 {{ expandedSize }}</span>
      </div>
      <input v-model="params.outpaint" placeholder="128,128,128,128" />
    </div>

    <div v-else-if="mode === 'controlnet' || mode === 'tile' || mode === 'img2img'" class="group">
      <button
        class="uploader"
        title="点击选择或拖入图片"
        @click="pickFile('controlImage')"
        @dragover.prevent
        @drop.prevent="onDrop($event, 'controlImage')"
      >
        <img v-if="previews.controlImage" :src="previews.controlImage" alt="" />
        <span v-else>{{
          mode === 'tile'
            ? '上传低分辨率图（放大到目标尺寸）'
            : mode === 'img2img'
              ? '上传参考图（可拖入 / Ctrl+V）'
              : '上传控制图（姿态/线稿/灰度）'
        }}</span>
      </button>
      <div class="row-between">
        <span class="field-label">
          {{ mode === 'img2img' ? '相似强度' : '控制强度' }}
        </span>
        <span class="value">{{ params.controlScale.toFixed(2) }}</span>
      </div>
      <input
        v-model.number="params.controlScale"
        type="range"
        min="0"
        max="2"
        step="0.05"
      />
      <p v-if="mode === 'img2img'" class="i2i-hint">
        通过 ControlNet 实现图生图：结果跟随参考图的结构与构图。相似强度越高越接近原图，越低 AI 发挥越大；配合提示词描述想要的画面。
      </p>
    </div>

    <div class="group params">
      <span class="section-title">生成参数</span>

      <div class="row-between">
        <span class="field-label">输出尺寸</span>
        <div class="inline">
          <input class="num" v-model.number="params.width" type="number" />
          <span class="x">×</span>
          <input class="num" v-model.number="params.height" type="number" />
        </div>
      </div>
      <div class="presets">
        <button
          v-for="p in presets"
          :key="p.label"
          class="preset"
          :class="{ on: params.width === p.w && params.height === p.h }"
          @click="setPreset(p)"
        >
          {{ p.label }}
        </button>
      </div>

      <div class="row-between">
        <span class="field-label">采样步数</span>
        <input
          class="num"
          v-model.number="params.steps"
          type="number"
          min="0"
          title="0 = 自动（由引擎决定）"
        />
      </div>

      <div class="row-between">
        <span class="field-label">随机种子</span>
        <div class="inline">
          <input
            class="num wide"
            v-model.number="params.seed"
            type="number"
            title="≥0 固定种子复现结果，-1 = 随机"
          />
          <button class="icon-btn" title="随机" @click="randomSeed">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
              <rect
                x="3.5"
                y="3.5"
                width="17"
                height="17"
                rx="4.5"
                stroke="#4B5563"
                stroke-width="1.6"
              />
              <circle cx="8.4" cy="8.4" r="1.3" fill="#4B5563" />
              <circle cx="15.6" cy="15.6" r="1.3" fill="#4B5563" />
              <circle cx="12" cy="12" r="1.3" fill="#4B5563" />
            </svg>
          </button>
        </div>
      </div>

      <div class="row-between">
        <span class="field-label">批量张数</span>
        <div class="stepper">
          <button @click="params.batch = Math.max(1, params.batch - 1)">−</button>
          <span>{{ params.batch }}</span>
          <button @click="params.batch = Math.min(16, params.batch + 1)">+</button>
        </div>
      </div>

      <button class="adv-toggle" @click="showAdvanced = !showAdvanced">
        {{ showAdvanced ? '收起高级设置' : '展开高级设置' }}
      </button>

      <template v-if="showAdvanced">
        <div class="row-between">
          <span class="field-label">模型路径</span>
          <input
            class="num wide"
            v-model="params.modelPath"
            list="dl-models-param"
            placeholder="z-image-turbo"
          />
          <datalist id="dl-models-param">
            <option v-for="m in foundModels" :key="m.path" :value="m.name" />
          </datalist>
        </div>
        <div class="row-between">
          <span class="field-label">计算设备</span>
          <select class="num wide" v-model.number="params.gpuId">
            <option :value="-2">自动</option>
            <option :value="-1">CPU</option>
            <option :value="0">GPU 0</option>
            <option :value="1">GPU 1</option>
            <option :value="2">GPU 2</option>
          </select>
        </div>
      </template>
    </div>

    <div class="spacer" />

    <button v-if="!isBusy" class="btn btn-primary block" :disabled="!canSubmit" @click="submit">
      开始生成 <span class="kbd">Ctrl + ↵</span>
    </button>
    <button v-else class="btn btn-danger block" @click="cancel">取消当前任务</button>
    <button class="btn btn-ghost block" @click="reset">重置参数</button>
  </aside>
</template>

<style scoped>
.panel {
  width: 400px;
  flex: none;
  background: #fff;
  border-right: 1px solid var(--border);
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}
/* 面板子项不随窗口变矮被压缩：空间不足时改为滚动，保持控件原始高度 */
.panel > * {
  flex-shrink: 0;
}
@media (max-width: 920px) {
  .panel {
    width: 100%;
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}
.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.tab {
  padding: 7px 12px;
  border-radius: var(--r-md);
  font-size: 13px;
  background: var(--tint);
  color: var(--text-2);
  transition: background 0.15s, color 0.15s;
}
.tab.active {
  background: var(--ink);
  color: #fff;
  font-weight: 500;
}
.group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.params {
  gap: 14px;
}
.section-title {
  font-size: 13px;
  font-weight: 500;
}
.row-between {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.counter {
  font-size: 11px;
  color: var(--muted);
}
.prompt-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}
.hist-btn {
  font-size: 11px;
  color: var(--text-2);
  background: var(--tint);
  border-radius: var(--r-sm);
  padding: 2px 8px;
}
.hist-btn:hover {
  color: var(--text);
}
.hist-panel {
  border: 1px solid var(--border-strong);
  border-radius: var(--r-md);
  background: #fff;
  max-height: 180px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}
.hist-item {
  text-align: left;
  font-size: 12px;
  color: var(--text-2);
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.hist-item:last-child {
  border-bottom: none;
}
.hist-item:hover {
  background: var(--tint-2);
  color: var(--text);
}
.inline {
  display: flex;
  align-items: center;
  gap: 8px;
}
.num {
  width: 76px;
  text-align: center;
  padding: 7px 4px;
}
.num.wide {
  width: 140px;
  text-align: left;
}
.x {
  color: var(--muted);
}
.presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.preset {
  font-size: 11px;
  padding: 5px 9px;
  border-radius: var(--r-sm);
  background: var(--tint);
  color: var(--text-2);
}
.preset.on {
  background: var(--ink);
  color: #fff;
}
.stepper {
  display: flex;
  align-items: center;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-md);
  height: 34px;
  width: 108px;
  justify-content: space-between;
  padding: 0 4px;
}
.stepper button {
  width: 26px;
  height: 26px;
  border-radius: 6px;
  color: var(--text-2);
  font-size: 15px;
  line-height: 1;
}
.stepper button:hover {
  background: var(--tint);
}
.icon-btn {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-md);
  background: #fff;
}
.icon-btn:hover {
  background: var(--tint-2);
}
.adv-toggle {
  font-size: 12px;
  color: var(--text-3);
  text-align: left;
  padding: 0;
}
.adv-toggle:hover {
  color: var(--text);
}
.uploads {
  gap: 10px;
}
.uploader {
  height: 84px;
  border: 1px dashed var(--border-strong);
  border-radius: var(--r-md);
  background: var(--tint-2);
  color: var(--text-3);
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 0;
}
.uploader:hover {
  border-color: var(--ink);
  color: var(--text);
}
.uploader img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.value {
  font-size: 12px;
  color: var(--text-2);
  font-variant-numeric: tabular-nums;
}
.i2i-hint {
  margin: 0;
  font-size: 11px;
  color: var(--muted);
  line-height: 1.7;
}
.spacer {
  flex: 1 0 auto;
  min-height: 16px;
}
.block {
  width: 100%;
}
.kbd {
  font-size: 11px;
  opacity: 0.55;
  margin-left: 4px;
}
</style>
