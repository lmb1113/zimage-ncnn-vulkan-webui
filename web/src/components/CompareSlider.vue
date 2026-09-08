<script setup>
import { ref } from 'vue'

// CompareSlider：原图/结果拖动分割线对比（支持触屏）
const props = defineProps({
  before: { type: String, required: true }, // 原图 URL
  after: { type: String, required: true }, // 结果 URL
})

const pct = ref(50)
const el = ref(null)
let down = false

function move(e) {
  if (!down || !el.value) return
  const r = el.value.getBoundingClientRect()
  const p = ((e.clientX - r.left) / r.width) * 100
  pct.value = Math.max(0, Math.min(100, p))
}
</script>

<template>
  <div
    ref="el"
    class="cmp"
    title="拖动分割线对比"
    @pointerdown="down = true"
    @pointermove="move"
    @pointerup="down = false"
    @pointerleave="down = false"
  >
    <img class="cmp-img" :src="before" alt="原图" />
    <img
      class="cmp-img cmp-top"
      :src="after"
      alt="结果"
      :style="{ clipPath: 'inset(0 0 0 ' + pct + '%)' }"
    />
    <div class="cmp-line" :style="{ left: pct + '%' }" />
    <span class="cmp-tag left">原图</span>
    <span class="cmp-tag right">结果</span>
  </div>
</template>

<style scoped>
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
</style>
