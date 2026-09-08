import { reactive, computed } from 'vue'
import api from './api'

export const store = reactive({
  view: 'generate',
  system: null,
  jobs: [],
  logs: {},
  gallery: [],
  selected: null,
  toast: null,
  connected: false,
  // 从图库「复用参数」时暂存，由参数面板消费
  pendingParams: null,
  // 全屏预览的图片地址
  lightbox: null,
})

export function openLightbox(url) {
  store.lightbox = url
}

// 一键联动：把图库/预览中的图带入生成工作台（mode: img2img | inpaint）
// 返回是否成功；图片必须带服务端 path（图库条目或任务输出）
export function reuseImage(item, mode) {
  if (!item || !item.path) return false
  const base = {
    prompt: '',
    negative: '',
    width: item.width || 1024,
    height: item.height || 1024,
    steps: 0,
    seed: -1,
    batch: 1,
    inputImage: '',
    maskImage: '',
    controlImage: '',
    controlScale: 1.0,
  }
  if (mode === 'img2img') {
    store.pendingParams = {
      ...base,
      mode: 'img2img',
      previews: { controlImage: item.url },
      controlImage: item.path,
    }
    store.view = 'generate'
    notify('已切换到图生图，参考图已就位')
  } else if (mode === 'inpaint') {
    store.pendingParams = {
      ...base,
      mode: 'inpaint',
      previews: { inputImage: item.url },
      inputImage: item.path,
    }
    store.view = 'generate'
    notify('已切换到局部重绘，输入图已就位')
  } else {
    return false
  }
  return true
}

let toastTimer = null
export function notify(message, kind = 'info') {
  store.toast = { message, kind }
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (store.toast = null), 3000)
}

export const activeJob = computed(() =>
  store.jobs.find((j) => j.status === 'running' || j.status === 'queued') || null
)

export const isBusy = computed(() => !!activeJob.value)

export async function refreshSystem() {
  try {
    store.system = await api.system()
  } catch (e) {
    notify(e.message, 'error')
  }
}

export async function refreshJobs() {
  try {
    const jobs = await api.jobs()
    store.jobs = jobs.filter((j) => j.status !== 'deleted')
  } catch (e) {
    /* 静默 */
  }
}

export async function refreshGallery() {
  try {
    store.gallery = await api.gallery()
  } catch (e) {
    notify(e.message, 'error')
  }
}

export async function selectImage(name) {
  if (!name) {
    store.selected = null
    return
  }
  try {
    store.selected = await api.galleryDetail(name)
  } catch (e) {
    notify(e.message, 'error')
  }
}

export async function removeImage(name) {
  try {
    await api.deleteImage(name)
    if (store.selected && store.selected.name === name) store.selected = null
    await refreshGallery()
    notify('已删除该图片')
  } catch (e) {
    notify(e.message, 'error')
  }
}

function pushLog(id, line) {
  if (!store.logs[id]) store.logs[id] = []
  store.logs[id].push(line)
  if (store.logs[id].length > 500) store.logs[id].shift()
}

export function connectEvents() {
  const es = new EventSource('/api/events')
  es.onopen = () => (store.connected = true)
  es.onerror = () => (store.connected = false)
  es.onmessage = (ev) => {
    let payload
    try {
      payload = JSON.parse(ev.data)
    } catch (e) {
      return
    }
    if (payload.type === 'log') {
      pushLog(payload.data.id, payload.data.line)
    } else if (payload.type === 'job') {
      const j = payload.data
      if (j.status === 'deleted') {
        store.jobs = store.jobs.filter((x) => x.id !== j.id)
        return
      }
      const idx = store.jobs.findIndex((x) => x.id === j.id)
      if (idx >= 0) store.jobs[idx] = { ...store.jobs[idx], ...j }
      else store.jobs = [j, ...store.jobs]

      if (j.status === 'success' || j.status === 'failed' || j.status === 'canceled') {
        refreshGallery()
      }
    } else if (payload.type === 'config') {
      store.system = payload.data
    }
  }
  return es
}

export async function bootstrap() {
  await refreshSystem()
  await refreshJobs()
  await refreshGallery()
  connectEvents()
}
