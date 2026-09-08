<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'

// MaskEditor：局部重绘的手绘蒙版编辑器。
// 在原图上用半透明红色画笔涂抹，提交时由 exportMask() 按原图分辨率
// 导出黑底白笔的蒙版 PNG（白=重绘，黑=保留），与引擎 -k 参数约定一致。
const props = defineProps({
  src: { type: String, required: true },
})

const wrap = ref(null)
const baseCanvas = ref(null)
const maskCanvas = ref(null)
const brushSize = ref(30)
const tool = ref('brush') // brush | erase
const strokes = ref([]) // { points: [{x,y}], size, erase }，坐标为显示像素

let img = new Image()
let drawing = false
let current = null
let ro = null

function redrawBase() {
  const c = baseCanvas.value
  if (!c || !wrap.value || !img.width) return
  c.width = wrap.value.clientWidth
  c.height = Math.round((c.width * img.naturalHeight) / img.naturalWidth)
  const ctx = c.getContext('2d')
  ctx.drawImage(img, 0, 0, c.width, c.height)
}

function redrawMask() {
  const c = maskCanvas.value
  const base = baseCanvas.value
  if (!c || !base || !base.width) return
  c.width = base.width
  c.height = base.height
  const ctx = c.getContext('2d')
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  for (const s of strokes.value) {
    // 橡皮：把已涂的红色擦掉（导出时按黑色处理）
    ctx.globalCompositeOperation = s.erase ? 'destination-out' : 'source-over'
    ctx.strokeStyle = 'rgba(255, 86, 86, 0.7)'
    ctx.fillStyle = 'rgba(255, 86, 86, 0.7)'
    ctx.lineWidth = s.size
    ctx.beginPath()
    s.points.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)))
    ctx.stroke()
    if (s.points.length === 1) {
      ctx.beginPath()
      ctx.arc(s.points[0].x, s.points[0].y, s.size / 2, 0, Math.PI * 2)
      ctx.fill()
    }
  }
  ctx.globalCompositeOperation = 'source-over'
}

function setup() {
  redrawBase()
  redrawMask()
}

function load() {
  img = new Image()
  img.onload = () => {
    strokes.value = []
    setup()
  }
  img.src = props.src
}

function pos(e) {
  const r = maskCanvas.value.getBoundingClientRect()
  return { x: e.clientX - r.left, y: e.clientY - r.top }
}

function down(e) {
  e.preventDefault()
  try {
    maskCanvas.value.setPointerCapture(e.pointerId)
  } catch (err) {
    /* 合成事件或指针已释放时忽略 */
  }
  drawing = true
  current = {
    points: [pos(e)],
    size: Number(brushSize.value),
    erase: tool.value === 'erase',
  }
  strokes.value.push(current)
  redrawMask()
}

function move(e) {
  if (!drawing) return
  e.preventDefault()
  current.points.push(pos(e))
  redrawMask()
}

function up() {
  drawing = false
  current = null
}

function undo() {
  strokes.value.pop()
  redrawMask()
}

function clear() {
  strokes.value = []
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
  const k = img.naturalWidth / baseCanvas.value.width
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  for (const s of strokes.value) {
    ctx.strokeStyle = s.erase ? '#000' : '#fff'
    ctx.fillStyle = s.erase ? '#000' : '#fff'
    ctx.lineWidth = s.size * k
    ctx.beginPath()
    s.points.forEach((p, i) =>
      i ? ctx.lineTo(p.x * k, p.y * k) : ctx.moveTo(p.x * k, p.y * k)
    )
    ctx.stroke()
    if (s.points.length === 1) {
      ctx.beginPath()
      ctx.arc(s.points[0].x * k, s.points[0].y * k, (s.size * k) / 2, 0, Math.PI * 2)
      ctx.fill()
    }
  }
  const blob = await new Promise((r) => c.toBlob(r, 'image/png'))
  return new File([blob], 'mask.png', { type: 'image/png' })
}

function onKey(e) {
  // 焦点在输入框时不拦截
  const tag = (e.target && e.target.tagName) || ''
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
  if ((e.ctrlKey || e.metaKey) && (e.key === 'z' || e.key === 'Z')) {
    e.preventDefault()
    undo()
  } else if (e.key === '[') {
    brushSize.value = Math.max(6, Number(brushSize.value) - 4)
  } else if (e.key === ']') {
    brushSize.value = Math.min(120, Number(brushSize.value) + 4)
  }
}

watch(() => props.src, load)
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

defineExpose({ hasStrokes, exportMask, clear })
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
      </div>
      <div class="size">
        <span class="slabel">笔刷</span>
        <input v-model.number="brushSize" type="range" min="6" max="120" step="2" />
      </div>
      <div class="tools">
        <button class="tbtn" :disabled="!strokes.length" @click="undo">撤销</button>
        <button class="tbtn" :disabled="!strokes.length" @click="clear">清空</button>
      </div>
    </div>

    <div ref="wrap" class="cwrap">
      <canvas ref="baseCanvas" class="base" />
      <canvas
        ref="maskCanvas"
        class="mask"
        @pointerdown="down"
        @pointermove="move"
        @pointerup="up"
        @pointercancel="up"
      />
    </div>
    <p class="mhint">在图上涂抹需要重绘的区域（红色），提交时自动生成蒙版。快捷键：Ctrl+Z 撤销、[ / ] 调笔刷</p>
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
.mhint {
  margin: 0;
  font-size: 11px;
  color: var(--muted);
  line-height: 1.6;
}
</style>
