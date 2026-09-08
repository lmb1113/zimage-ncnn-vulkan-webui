<script setup>
import { reactive, watch, computed } from 'vue'
import api from '../api'
import { store, refreshSystem, notify } from '../store'

const form = reactive({
  exePath: '',
  workDir: '',
  outputDir: '',
  uploadDir: '',
  modelPath: 'z-image-turbo',
  gpuId: -2,
  port: 19777,
})

watch(
  () => store.system,
  (s) => {
    if (s && s.config) Object.assign(form, s.config)
  },
  { immediate: true, deep: true }
)

const models = computed(() => (store.system ? store.system.models || [] : []))
const exeFound = computed(() => !!(store.system && store.system.exeFound))

async function save() {
  try {
    await api.saveConfig({ ...form, gpuId: Number(form.gpuId), port: Number(form.port) })
    await refreshSystem()
    notify('设置已保存', 'success')
  } catch (e) {
    notify(e.message, 'error')
  }
}

async function openDir(path) {
  try {
    await api.openPath(path)
  } catch (e) {
    notify(e.message, 'error')
  }
}
</script>

<template>
  <div class="view">
    <div class="col">
      <section class="card block-card">
        <h3>引擎与路径</h3>
        <p class="hint">
          工作目录需要包含 <code>z-image-turbo</code> 模型文件夹；可执行文件通常就在同级目录。
        </p>

        <label>
          <span>可执行文件 zimage-ncnn-vulkan</span>
          <div class="with-btn">
            <input v-model="form.exePath" placeholder="F:\aiimage\zimage-ncnn-vulkan.exe" />
            <button class="btn btn-sm btn-ghost" @click="openDir(form.workDir)">浏览</button>
          </div>
        </label>

        <label>
          <span>工作目录（模型所在目录）</span>
          <div class="with-btn">
            <input v-model="form.workDir" placeholder="F:\aiimage" />
            <button class="btn btn-sm btn-ghost" @click="openDir(form.workDir)">打开</button>
          </div>
        </label>

        <label>
          <span>输出目录</span>
          <div class="with-btn">
            <input v-model="form.outputDir" placeholder="F:\aiimage\out" />
            <button class="btn btn-sm btn-ghost" @click="openDir(form.outputDir)">打开</button>
          </div>
        </label>

        <label>
          <span>上传目录（输入图 / 遮罩 / 控制图）</span>
          <input v-model="form.uploadDir" placeholder="F:\aiimage\uploads" />
        </label>
      </section>

      <section class="card block-card">
        <h3>默认参数</h3>
        <div class="two">
          <label>
            <span>默认模型 · -m</span>
            <input v-model="form.modelPath" placeholder="z-image-turbo" />
          </label>
          <label>
            <span>默认设备 · -g</span>
            <select v-model.number="form.gpuId">
              <option :value="-2">自动</option>
              <option :value="-1">CPU</option>
              <option :value="0">GPU 0</option>
              <option :value="1">GPU 1</option>
              <option :value="2">GPU 2</option>
            </select>
          </label>
        </div>
        <label>
          <span>服务端口（重启后生效）</span>
          <input v-model.number="form.port" type="number" />
        </label>
      </section>

      <div class="actions">
        <button class="btn btn-primary" @click="save">保存设置</button>
        <button class="btn btn-ghost" @click="refreshSystem">重新检测</button>
      </div>
    </div>

    <aside class="card block-card side">
      <h3>环境自检</h3>
      <div class="check">
        <span class="pill" :class="exeFound ? 'pill-ok' : 'pill-err'">
          <i class="dot" :class="{ ok: exeFound }" />{{ exeFound ? '引擎已找到' : '引擎缺失' }}
        </span>
        <code class="path">{{ form.exePath }}</code>
      </div>

      <ul class="models">
        <li v-for="m in models" :key="m.name">
          <span class="pill" :class="m.found ? 'pill-ok' : 'pill-warn'">
            <i class="dot" :class="{ ok: m.found }" />{{ m.name }}
          </span>
          <code class="path">{{ m.path }}</code>
        </li>
      </ul>

      <p class="hint">
        ControlNet 需要 <code>z-image-control</code>，Tile 放大需要
        <code>z-image-control-tile</code>，两者都要放在工作目录下。
      </p>
    </aside>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  gap: 16px;
  padding: 20px;
  height: 100%;
  overflow-y: auto;
}
.col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 720px;
}
.side {
  width: 380px;
  flex: none;
  align-self: flex-start;
}
.block-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
.hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-3);
  line-height: 1.7;
}
label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  color: var(--text-3);
}
.with-btn {
  display: flex;
  gap: 8px;
}
.two {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}
.code,
code {
  font-family: Consolas, 'Noto Sans Mono', monospace;
}
code {
  background: var(--tint);
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 11px;
}
.actions {
  display: flex;
  gap: 10px;
}
.check {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}
.path {
  font-size: 11px;
  color: var(--muted);
  word-break: break-all;
}
.models {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.models li {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #c9ccd1;
  display: inline-block;
}
.dot.ok {
  background: var(--success);
}
</style>
