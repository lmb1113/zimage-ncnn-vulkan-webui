<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { store, reuseImage, lightboxNav } from '../store'
import CompareSlider from './CompareSlider.vue'

function close() {
  store.lightbox = null
}

// 由 /media/<name> 反查图库条目，拿到服务端 path 才能联动
const item = computed(() => {
  if (!store.lightbox) return null
  const name = decodeURIComponent(String(store.lightbox).split('/media/')[1] || '')
  if (!name) return null
  return store.gallery.find((g) => g.name === name) || null
})

function act(mode) {
  if (reuseImage(item.value, mode)) close()
}

// 全屏对比模式（仅当该图有输入图/参考图时可用）
const showCompare = ref(false)
const compareUrl = computed(() => {
  const it = item.value
  return it ? it.inputUrl || it.controlUrl || '' : ''
})

const canNav = computed(() => store.lightboxList.length > 1)

function onKey(e) {
  if (e.key === 'Escape') {
    if (showCompare.value) showCompare.value = false
    else close()
  } else if (!showCompare.value) {
    if (e.key === 'ArrowLeft') lightboxNav(-1)
    else if (e.key === 'ArrowRight') lightboxNav(1)
  }
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <transition name="fade">
    <div v-if="store.lightbox" class="lb" @click="close">
      <CompareSlider
        v-if="showCompare && compareUrl"
        class="lb-cmp"
        :before="compareUrl"
        :after="store.lightbox"
        @click.stop
      />
      <img v-else :src="store.lightbox" alt="" @click.stop />
      <button class="close" title="关闭 (Esc)" @click="close">×</button>
      <template v-if="canNav && !showCompare">
        <button class="nav prev" title="上一张 (←)" @click.stop="lightboxNav(-1)">‹</button>
        <button class="nav next" title="下一张 (→)" @click.stop="lightboxNav(1)">›</button>
        <span class="counter">{{ store.lightboxIdx + 1 }} / {{ store.lightboxList.length }}</span>
      </template>
      <div class="actions" @click.stop>
        <template v-if="item">
          <button class="lb-btn" @click="act('img2img')">作为图生图参考</button>
          <button class="lb-btn" @click="act('inpaint')">作为重绘输入</button>
        </template>
        <button
          v-if="compareUrl"
          class="lb-btn"
          :class="{ on: showCompare }"
          @click="showCompare = !showCompare"
        >
          {{ showCompare ? '退出对比' : '对比原图' }}
        </button>
        <a class="lb-btn" :href="store.lightbox" download>下载原图</a>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.lb {
  position: fixed;
  inset: 0;
  background: rgba(16, 18, 22, 0.82);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  cursor: zoom-out;
}
.lb img {
  max-width: 92vw;
  max-height: 82vh;
  border-radius: 10px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
  cursor: default;
}
.close {
  position: absolute;
  top: 20px;
  right: 24px;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.14);
  color: #fff;
  font-size: 22px;
  line-height: 1;
}
.close:hover {
  background: rgba(255, 255, 255, 0.26);
}
.nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.14);
  color: #fff;
  font-size: 28px;
  line-height: 1;
}
.nav:hover {
  background: rgba(255, 255, 255, 0.26);
}
.nav.prev {
  left: 24px;
}
.nav.next {
  right: 24px;
}
.counter {
  position: absolute;
  top: 28px;
  left: 50%;
  transform: translateX(-50%);
  color: rgba(255, 255, 255, 0.85);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}
.lb-cmp {
  max-width: 92vw;
  max-height: 82vh;
  border-radius: 10px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
  cursor: ew-resize;
}
.lb-btn.on {
  background: #fff;
  color: #14161a;
}
.actions {
  position: absolute;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 10px;
}
.lb-btn {
  background: rgba(255, 255, 255, 0.14);
  color: #fff;
  font-size: 13px;
  padding: 9px 18px;
  border-radius: 999px;
  text-decoration: none;
  white-space: nowrap;
}
.lb-btn:hover {
  background: rgba(255, 255, 255, 0.26);
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
