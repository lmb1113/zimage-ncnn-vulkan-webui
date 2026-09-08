<script setup>
import { computed, onMounted, onUnmounted } from 'vue'
import { store, reuseImage } from '../store'

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

function onKey(e) {
  if (e.key === 'Escape') close()
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <transition name="fade">
    <div v-if="store.lightbox" class="lb" @click="close">
      <img :src="store.lightbox" alt="" @click.stop />
      <button class="close" title="关闭 (Esc)" @click="close">×</button>
      <div class="actions" @click.stop>
        <template v-if="item">
          <button class="lb-btn" @click="act('img2img')">作为图生图参考</button>
          <button class="lb-btn" @click="act('inpaint')">作为重绘输入</button>
        </template>
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
