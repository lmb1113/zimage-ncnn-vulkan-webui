<script setup>
import { onMounted, onUnmounted } from 'vue'
import { store } from '../store'

function close() {
  store.lightbox = null
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
      <a class="dl" :href="store.lightbox" download @click.stop>下载原图</a>
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
  max-height: 88vh;
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
.dl {
  position: absolute;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(255, 255, 255, 0.14);
  color: #fff;
  font-size: 13px;
  padding: 9px 18px;
  border-radius: 999px;
  text-decoration: none;
}
.dl:hover {
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
