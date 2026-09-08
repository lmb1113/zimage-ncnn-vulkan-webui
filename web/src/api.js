async function req(url, options) {
  const res = await fetch(url, options)
  if (!res.ok) {
    let msg = `请求失败 (${res.status})`
    try {
      const data = await res.json()
      if (data && data.error) msg = data.error
    } catch (e) {
      /* 保持默认提示 */
    }
    throw new Error(msg)
  }
  return res.json()
}

export default {
  system: () => req('/api/system'),

  saveConfig: (config) =>
    req('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    }),

  generate: (mode, params) =>
    req('/api/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode, params }),
    }),

  jobs: () => req('/api/jobs'),
  cancelJob: (id) => req(`/api/jobs/${encodeURIComponent(id)}/cancel`, { method: 'POST' }),
  cancelQueued: () => req('/api/jobs/cancel-queued', { method: 'POST' }),

  upload: async (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return req('/api/upload', { method: 'POST', body: fd })
  },

  gallery: () => req('/api/gallery'),
  galleryDetail: (name) => req(`/api/gallery/${encodeURIComponent(name)}`),
  deleteImage: (name) =>
    req(`/api/gallery/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  clearUploads: () => req('/api/uploads/clear', { method: 'POST' }),

  openPath: (path) =>
    req('/api/open', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    }),

  autostartStatus: () => req('/api/autostart'),
  setAutostart: (enable) =>
    req('/api/autostart', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enable }),
    }),

  mediaUrl: (name) => `/media/${encodeURIComponent(name)}`,
}
