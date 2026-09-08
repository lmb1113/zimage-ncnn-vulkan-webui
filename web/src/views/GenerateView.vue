<script setup>
import { computed, ref, onUnmounted } from 'vue'
import ParamPanel from '../components/ParamPanel.vue'
import PreviewPane from '../components/PreviewPane.vue'
import ConsolePane from '../components/ConsolePane.vue'
import { store, activeJob, isBusy } from '../store'
import api from '../api'

const now = ref(Date.now())
const timer = setInterval(() => (now.value = Date.now()), 500)
onUnmounted(() => clearInterval(timer))

const elapsed = computed(() => {
  const j = activeJob.value
  if (!j) return ''
  if (j.elapsedMs) return `${(j.elapsedMs / 1000).toFixed(1)}s`
  if (!j.startedAt) return ''
  const s = (now.value - new Date(j.startedAt).getTime()) / 1000
  return `${s.toFixed(0)}s`
})

const queued = computed(
  () => store.jobs.filter((j) => j.status === 'queued').length
)

const headLine = computed(() => {
  if (!activeJob.value) return '空闲 · 可以开始生成'
  const j = activeJob.value
  if (j.status === 'queued') return '排队中 · 等待上一个任务释放显卡'
  return `正在生成 · ${j.progress}%`
})

function cancel() {
  if (activeJob.value) api.cancelJob(activeJob.value.id)
}
</script>

<template>
  <div class="view">
    <ParamPanel />
    <div class="right">
      <div class="status card">
        <div class="left">
          <svg
            v-if="isBusy"
            class="spin"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
          >
            <path d="M12 3a9 9 0 1 0 9 9" stroke="#14161A" stroke-width="2.2" stroke-linecap="round" />
          </svg>
          <span class="head">{{ headLine }}</span>
        </div>
        <div class="right-side">
          <span v-if="queued > 1" class="pill">队列 {{ queued }}</span>
          <span class="muted">{{ elapsed }}</span>
          <button v-if="isBusy" class="btn btn-sm btn-danger" @click="cancel">终止</button>
        </div>
      </div>

      <PreviewPane />
      <ConsolePane />
    </div>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  height: 100%;
  min-height: 0;
}
.right {
  flex: 1;
  min-width: 0;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
/* 窄窗口（平板/分屏）时改为上下布局 */
@media (max-width: 920px) {
  .view {
    flex-direction: column;
    overflow-y: auto;
  }
  .right {
    padding: 16px;
  }
}
.status {
  flex: none;
  padding: 14px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.head {
  font-size: 14px;
  font-weight: 500;
}
.right-side {
  display: flex;
  align-items: center;
  gap: 12px;
}
.muted {
  font-size: 12px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
</style>
