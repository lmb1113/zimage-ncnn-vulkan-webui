<script setup>
import { computed, ref, watch, nextTick } from 'vue'
import { store, activeJob } from '../store'
import api from '../api'

const autoScroll = ref(true)
const box = ref(null)

const job = computed(() => activeJob.value || store.jobs[0] || null)
const lines = computed(() => (job.value ? store.logs[job.value.id] || [] : []))

watch(
  lines,
  async () => {
    if (!autoScroll.value) return
    await nextTick()
    if (box.value) box.value.scrollTop = box.value.scrollHeight
  },
  { deep: true }
)

async function openFolder() {
  try {
    await api.openPath('')
  } catch (e) {
    /* 忽略 */
  }
}
</script>

<template>
  <section class="console card">
    <header>
      <span class="title">控制台</span>
      <div class="right">
        <button class="chip" :class="{ on: autoScroll }" @click="autoScroll = !autoScroll">
          自动滚动
        </button>
        <button class="chip" @click="openFolder">打开输出目录</button>
      </div>
    </header>
    <div ref="box" class="log">
      <p v-for="(l, i) in lines" :key="i" :class="{ ok: l.includes('[完成]'), err: l.includes('[错误]') }">
        {{ l }}
      </p>
      <p v-if="!lines.length" class="placeholder">
        {{ job ? '等待引擎输出…' : '暂无日志' }}
      </p>
    </div>
  </section>
</template>

<style scoped>
.console {
  flex: none;
  height: 190px;
  display: flex;
  flex-direction: column;
  padding: 16px;
  gap: 10px;
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.title {
  font-size: 13px;
  font-weight: 500;
}
.right {
  display: flex;
  gap: 8px;
}
.chip {
  font-size: 11px;
  padding: 5px 9px;
  border-radius: var(--r-sm);
  background: var(--tint);
  color: var(--text-3);
}
.chip.on {
  background: var(--ink);
  color: #fff;
}
.log {
  flex: 1;
  overflow-y: auto;
  font-family: 'SFMono-Regular', Consolas, 'Noto Sans Mono', monospace;
  font-size: 11px;
  line-height: 18px;
  color: var(--text-2);
  background: var(--tint-2);
  border-radius: var(--r-md);
  padding: 10px 12px;
}
.log p {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}
.log .ok {
  color: var(--success);
}
.log .err {
  color: var(--danger);
}
.placeholder {
  color: var(--muted);
}
</style>
