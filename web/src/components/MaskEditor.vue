<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'

// MaskEditor：局部重绘的手绘蒙版编辑器。
// 支持滚轮缩放（以光标为中心）、移动工具平移；笔画坐标全部存图片原始像素，
// 缩放/平移/窗口尺寸变化都不会让笔迹漂移。提交时由 exportMask() 按原图分辨率
// 导出黑底白笔蒙版（白=重绘，黑=保留），与引擎 -k 参数约定一致。
const props = defineProps({
  src: { type: String, required: true },
})

const wrap = ref(null)
const baseCanvas = ref(null)
const maskCanvas = ref(null)
const brushSize = ref(30)
const tool = ref('brush') // brush | erase | pan
const zoomPct = ref(100)
const strokes = ref([]) // { points:[{x,y}], size, erase }，坐标与 size 均为图片原始像素
const undone = ref([]) // 被撤销的笔画，供重做

let img = new Image()
let drawing = false
let panning = false
let panStart = null // { x, y, panX, panY }
let current = null
let ro = null
let cursorPos = null // { x, y } 画布内屏幕坐标，用于笔刷预览圈

const MIN_ZOOM = 1
const MAX_ZOOM = 8

// ---------- 视图状态 ----------

let baseW = 0 // 适应宽度（容器宽）
let zoom = 1
let panX = 0
let panY = 0

function clampPan() {
  const w = wrap.value ? wrap.value.clientWidth : 0
  const h = Math.round((w * img.naturalHeight) / img.naturalWidth) || 0
  const vw = w * zoom
  const vh = h * zoom
  panX = vw <= w ? (w - vw) / 2 : Math.min(0, Math.max(w - vw, panX))
  panY = vh <= h ? (h - vh) / 2 : Math.min(0, Math.max(h - vh, panY))
}

function setZoom(z, cx, cy) {
  const old = zoom
  z = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z))
  if (z === old) return
  // 以 (cx, cy) 为焦点缩放
  panX = cx - ((cx - panX) * z) / old
  panY = cy - ((cy - panY) * z) / old
  zoom = z
  clampPan()
  zoomPct.value = Math.round(zoom * 100)
  redrawAll()
}

function resetView() {
  zoom = 1
  panX = 0
  panY = 0
  zoomPct.value = 100
  redrawAll()
}

// 屏幕坐标 → 图片原始像素
function toNatural(clientX, clientY) {
  const r = maskCanvas.value.getBoundingClientRect()
  const sx = clientX - r.left
  const sy = clientY - r.top
  const k = img.naturalWidth / baseW
  return {
    x: ((sx - panX) / zoom) * k,
    y: ((sy - panY) / zoom) * k,
  }
}

// ---------- 绘制 ----------

function redrawBase() {
  const c = baseCanvas.value
  if (!c || !wrap.value || !img.width) return
  baseW = wrap.value.clientWidth
  c.width = baseW
  c.height = Math.round((baseW * img.naturalHeight) / img.naturalWidth)
  const ctx = c.getContext('2d')
  ctx.setTransform(zoom, 0, 0, zoom, panX, panY)
  ctx.drawImage(img, 0, 0, baseW, baseH())
}

function baseH() {
  return Math.round((baseW * img.naturalHeight) / img.naturalWidth)
}

function redrawMask() {
  const c = maskCanvas.value
  const base = baseCanvas.value
  if (!c || !base || !base.width) return
  c.width = base.width
  c.height = base.height
  const ctx = c.getContext('2d')
  // 笔迹存的是原始像素：视图变换 = 缩放 × (适应比例的倒数)
  const view = (zoom * baseW) / img.naturalWidth
  ctx.setTransform(view, 0, 0, view, panX, panY)
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  for (const st of strokes.value) {
    ctx.globalCompositeOperation = st.erase ? 'destination-out' : 'source-over'
    ctx.strokeStyle = 'rgba(255, 86, 86, 0.7)'
    ctx.fillStyle = 'rgba(255, 86, 86, 0.7)'
    ctx.lineWidth = st.size
    ctx.beginPath()
    st.points.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)))
    ctx.stroke()
    if (st.points.length === 1) {
      ctx.beginPath()
      ctx.arc(st.points[0].x, st.points[0].y, st.size / 2, 0, Math.PI * 2)
      ctx.fill()
    }
  }
  ctx.globalCompositeOperation = 'source-over'
  ctx.setTransform(1, 0, 0, 1, 0, 0)

  // 笔刷预览圈：跟随光标显示实际涂抹范围
  if (cursorPos && tool.value !== 'pan') {
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.9)'
    ctx.lineWidth = 1.5
    ctx.beginPath()
    ctx.arc(cursorPos.x, cursorPos.y, Number(brushSize.value) / 2, 0, Math.PI * 2)
    ctx.stroke()
    ctx.strokeStyle = 'rgba(0, 0, 0, 0.5)'
    ctx.beginPath()
    ctx.arc(cursorPos.x, cursorPos.y, Number(brushSize.value) / 2 + 1.5, 0, Math.PI * 2)
    ctx.stroke()
  }
}

function redrawAll() {
  redrawBase()
  redrawMask()
}

function setup() {
  redrawAll()
}

function load() {
  img = new Image()
  img.onload = () => {
    strokes.value = []
    undone.value = []
    resetView()
  }
  img.src = props.src
}

// ---------- 交互 ----------

function down(e) {
  e.preventDefault()
  if (tool.value === 'pan') {
    panning = true
    panStart = { x: e.clientX, y: e.clientY, panX, panY }
    return
  }
  try {
    maskCanvas.value.setPointerCapture(e.pointerId)
  } catch (err) {
    /* 合成事件或指针已释放时忽略 */
  }
  drawing = true
  const p = toNatural(e.clientX, e.clientY)
  const k = img.naturalWidth / baseW
  current = {
    points: [p],
    size: (Number(brushSize.value) * k) / zoom,
    erase: tool.value === 'erase',
  }
  strokes.value.push(current)
  undone.value = [] // 有新笔画后，之前的重做分支作废
  drawDot(current)
}

function move(e) {
  if (panning) {
    panX = panStart.panX + (e.clientX - panStart.x)
    panY = panStart.panY + (e.clientY - panStart.y)
    clampPan()
    redrawAll()
    return
  }
  const r = maskCanvas.value.getBoundingClientRect()
  cursorPos = { x: e.clientX - r.left, y: e.clientY - r.top }
  if (!drawing) {
    redrawMask()
    return
  }
  e.preventDefault()
  current.points.push(toNatural(e.clientX, e.clientY))
  drawSegment(current)
  // 绘制中不画光标圈：刚落下的笔迹本身就是宽度预览，避免残影
}

// 增量绘制：只画最新一段，长笔画会话不随笔画数变卡
function drawSegment(st) {
  const pts = st.points
  if (pts.length < 2) return
  const ctx = maskCanvas.value.getContext('2d')
  const view = (zoom * baseW) / img.naturalWidth
  ctx.setTransform(view, 0, 0, view, panX, panY)
  ctx.globalCompositeOperation = st.erase ? 'destination-out' : 'source-over'
  ctx.strokeStyle = 'rgba(255, 86, 86, 0.7)'
  ctx.lineWidth = st.size
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.beginPath()
  ctx.moveTo(pts[pts.length - 2].x, pts[pts.length - 2].y)
  ctx.lineTo(pts[pts.length - 1].x, pts[pts.length - 1].y)
  ctx.stroke()
  ctx.globalCompositeOperation = 'source-over'
  ctx.setTransform(1, 0, 0, 1, 0, 0)
}

// 落点圆点（增量）
function drawDot(st) {
  const ctx = maskCanvas.value.getContext('2d')
  const view = (zoom * baseW) / img.naturalWidth
  ctx.setTransform(view, 0, 0, view, panX, panY)
  ctx.globalCompositeOperation = st.erase ? 'destination-out' : 'source-over'
  ctx.fillStyle = 'rgba(255, 86, 86, 0.7)'
  ctx.beginPath()
  ctx.arc(st.points[0].x, st.points[0].y, st.size / 2, 0, Math.PI * 2)
  ctx.fill()
  ctx.globalCompositeOperation = 'source-over'
  ctx.setTransform(1, 0, 0, 1, 0, 0)
}

// 光标笔刷预览圈（屏幕坐标系，独立于笔画重绘）
function drawCursor() {
  if (!cursorPos || tool.value === 'pan') return
  const ctx = maskCanvas.value.getContext('2d')
  ctx.setTransform(1, 0, 0, 1, 0, 0)
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.9)'
  ctx.lineWidth = 1.5
  ctx.beginPath()
  ctx.arc(cursorPos.x, cursorPos.y, Number(brushSize.value) / 2, 0, Math.PI * 2)
  ctx.stroke()
  ctx.strokeStyle = 'rgba(0, 0, 0, 0.5)'
  ctx.beginPath()
  ctx.arc(cursorPos.x, cursorPos.y, Number(brushSize.value) / 2 + 1.5, 0, Math.PI * 2)
  ctx.stroke()
}

function up() {
  drawing = false
  panning = false
  current = null
  redrawMask() // 归一化渲染，消除增量绘制期间的预览圈残留
}

function onWheel(e) {
  if (!maskCanvas.value) return
  e.preventDefault()
  const r = maskCanvas.value.getBoundingClientRect()
  setZoom(
    zoom * (e.deltaY < 0 ? 1.2 : 1 / 1.2),
    e.clientX - r.left,
    e.clientY - r.top
  )
}

function undo() {
  if (!strokes.value.length) return
  undone.value.push(strokes.value.pop())
  redrawMask()
}

function redo() {
  if (!undone.value.length) return
  strokes.value.push(undone.value.pop())
  redrawMask()
}

function clear() {
  strokes.value = []
  undone.value = []
  redrawMask()
}

function hasStrokes() {
  return strokes.value.length > 0
}

// 按原图分辨率导出蒙版：黑底 + 白色笔迹（橡皮涂黑）
async function exportMask() {
  const c = document.createElement('canvas')
  c.width = img.naturalWidth
  c.height = img.naturalHeight
  const ctx = c.getContext('2d')
  ctx.fillStyle = '#000'
  ctx.fillRect(0, 0, c.width, c.height)
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  for (const st of strokes.value) {
    ctx.strokeStyle = st.erase ? '#000' : '#fff'
    ctx.fillStyle = st.erase ? '#000' : '#fff'
    ctx.lineWidth = st.size
    ctx.beginPath()
    st.points.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)))
    ctx.stroke()
    if (st.points.length === 1) {
      ctx.beginPath()
      ctx.arc(st.points[0].x, st.points[0].y, st.size / 2, 0, Math.PI * 2)
      ctx.fill()
    }
  }
  const blob = await new Promise((r) => c.toBlob(r, 'image/png'))
  return new File([blob], 'mask.png', { type: 'image/png' })
}

function onLeave() {
  cursorPos = null
  redrawMask()
}

watch(() => props.src, load)
watch(brushSize, () => {
  if (cursorPos) redrawMask()
})
onMounted(() => {
  load()
  ro = new ResizeObserver(setup)
  ro.observe(wrap.value)
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  ro && ro.disconnect()
  window.removeEventListener('keydown', onKey)
})

// 快捷键：Ctrl+Z 撤销、[ / ] 调笔刷（输入框聚焦时不拦截）
function onKey(e) {
  const tag = (e.target && e.target.tagName) || ''
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
  if ((e.ctrlKey || e.metaKey) && (e.key === 'z' || e.key === 'Z')) {
    e.preventDefault()
    if (e.shiftKey) redo()
    else undo()
  } else if ((e.ctrlKey || e.metaKey) && e.key === 'y') {
    e.preventDefault()
    redo()
  } else if (e.key === '[') {
    brushSize.value = Math.max(6, Number(brushSize.value) - 4)
  } else if (e.key === ']') {
    brushSize.value = Math.min(120, Number(brushSize.value) + 4)
  }
}

defineExpose({ hasStrokes, exportMask, clear, redo })
</script>

<template>
  <div class="meditor">
    <div class="mtoolbar">
      <div class="tools">
        <button class="tbtn" :class="{ on: tool === 'brush' }" @click="tool = 'brush'">
          画笔
        </button>
        <button class="tbtn" :class="{ on: tool === 'erase' }" @click="tool = 'erase'">
          橡皮
        </button>
        <button class="tbtn" :class="{ on: tool === 'pan' }" title="拖动平移画面" @click="tool = 'pan'">
          移动
        </button>
      </div>
      <div class="size">
        <span class="slabel">笔刷</span>
        <input v-model.number="brushSize" type="range" min="6" max="120" step="2" />
      </div>
      <div class="tools">
        <span class="zoom-label">{{ zoomPct }}%</span>
        <button class="tbtn" :disabled="!strokes.length" title="Ctrl+Z" @click="undo">撤销</button>
        <button class="tbtn" :disabled="!undone.length" title="Ctrl+Shift+Z / Ctrl+Y" @click="redo">重做</button>
        <button class="tbtn" :disabled="!strokes.length" @click="clear">清空</button>
        <button class="tbtn" :disabled="zoomPct === 100" @click="resetView">适应</button>
      </div>
    </div>

    <div ref="wrap" class="cwrap">
      <canvas ref="baseCanvas" class="base" />
      <canvas
        ref="maskCanvas"
        class="mask"
        :class="{ pan: tool === 'pan' }"
        @pointerdown="down"
        @pointermove="move"
        @pointerup="up"
        @pointercancel="up"
        @pointerleave="onLeave"
        @wheel="onWheel"
      />
    </div>
    <p class="mhint">
      涂抹需要重绘的区域（红色），提交时自动生成蒙版。滚轮缩放、移动工具平移；Ctrl+Z 撤销 / Ctrl+Shift+Z 重做、[ / ] 调笔刷
    </p>
  </div>
</template>

<style scoped>
.meditor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.mtoolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}
.tools {
  display: flex;
  gap: 6px;
  align-items: center;
}
.tbtn {
  padding: 6px 10px;
  border-radius: var(--r-md);
  font-size: 12px;
  background: var(--tint);
  color: var(--text-2);
  transition: background 0.15s, color 0.15s;
}
.tbtn.on {
  background: var(--ink);
  color: #fff;
  font-weight: 500;
}
.tbtn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.zoom-label {
  font-size: 11px;
  color: var(--text-3);
  font-variant-numeric: tabular-nums;
  min-width: 34px;
  text-align: center;
}
.size {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 120px;
}
.slabel {
  font-size: 12px;
  color: var(--text-3);
  flex: none;
}
.size input {
  padding: 0;
  border: none;
  height: 18px;
}
.cwrap {
  position: relative;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-md);
  overflow: hidden;
  line-height: 0;
  background: var(--tint);
}
.cwrap canvas {
  display: block;
  width: 100%;
}
.mask {
  position: absolute;
  inset: 0;
  cursor: crosshair;
  touch-action: none;
}
.mask.pan {
  cursor: grab;
}
.mhint {
  margin: 0;
  font-size: 11px;
  color: var(--muted);
  line-height: 1.6;
}
</style>
