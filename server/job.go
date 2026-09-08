package server

import (
	"bufio"
	"encoding/json"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Params 对应 zimage-ncnn-vulkan 的全部可控参数。
type Params struct {
	Prompt       string  `json:"prompt"`
	Negative     string  `json:"negative"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Steps        int     `json:"steps"`  // 0 = 自动
	Seed         int64   `json:"seed"`   // -1 = 随机
	Batch        int     `json:"batch"`  // -b
	ModelPath    string  `json:"modelPath"`
	GPUID        int     `json:"gpuId"`  // -2 自动 / -1 CPU / >=0 指定设备
	InputImage   string  `json:"inputImage"`   // -i
	MaskImage    string  `json:"maskImage"`    // -k
	Outpaint     string  `json:"outpaint"`     // -x l,t,r,b
	ControlImage string  `json:"controlImage"` // -c
	ControlScale float64 `json:"controlScale"` // -w
	TileUpscale  bool    `json:"tileUpscale"`  // -t
}

// Job 是一次生成任务。
type Job struct {
	ID       string `json:"id"`
	Mode     string `json:"mode"`
	Status   string `json:"status"` // queued / running / success / failed / canceled
	Progress int    `json:"progress"`
	Current  int    `json:"current"`
	Total    int    `json:"total"`
	Params   Params `json:"params"`
	Command  string `json:"command"`
	Outputs  []string `json:"outputs"`
	Error    string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	ElapsedMs int64      `json:"elapsedMs"`
}

const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

// maxKeepJobs 内存中保留的最大任务数。
const maxKeepJobs = 100

// Event 是 SSE 推送的消息。
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Bus 负责把任务状态与日志广播给所有前端连接。
type Bus struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func newBus() *Bus {
	return &Bus{subs: map[chan Event]struct{}{}}
}

func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *Bus) Publish(t string, data interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- Event{Type: t, Data: data}:
		default:
		}
	}
}

// Manager 管理任务队列与唯一的执行 worker（GPU 同时只跑一个任务）。
type Manager struct {
	mu      sync.Mutex
	jobs    map[string]*Job
	order   []string
	queue   chan *Job
	bus     *Bus
	running *Job
	cmd     *exec.Cmd
	cancel  func()
	once    sync.Once
}

// NewManager 创建任务管理器并启动后台 worker。
func NewManager(bus *Bus) *Manager {
	m := &Manager{
		jobs:  map[string]*Job{},
		queue: make(chan *Job, 128),
		bus:   bus,
	}
	m.loadPersisted()
	go m.worker()
	return m
}

// jobsFile 任务历史的持久化位置（输出目录 .meta 下）。
func jobsFile() string {
	return filepath.Join(metaDir(GetConfig().OutputDir), "jobs.json")
}

var persistMu sync.Mutex

// persist 将任务历史原子写入磁盘（提交/状态变更/删除时调用）。
func (m *Manager) persist() {
	m.mu.Lock()
	jobs := make([]*Job, 0, len(m.order))
	for _, id := range m.order {
		if j := m.jobs[id]; j != nil {
			jobs = append(jobs, j)
		}
	}
	m.mu.Unlock()

	b, err := json.Marshal(jobs)
	if err != nil {
		return
	}
	persistMu.Lock()
	defer persistMu.Unlock()
	f := jobsFile()
	_ = os.MkdirAll(filepath.Dir(f), 0o755)
	tmp := f + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, f)
}

// loadPersisted 启动时恢复任务历史；上次未完成的任务标记为中断。
func (m *Manager) loadPersisted() {
	b, err := os.ReadFile(jobsFile())
	if err != nil {
		return
	}
	var jobs []*Job
	if err := json.Unmarshal(b, &jobs); err != nil {
		return
	}
	m.mu.Lock()
	for _, j := range jobs {
		if j.Status == StatusRunning || j.Status == StatusQueued {
			j.Status = StatusFailed
			j.Error = "服务重启，任务中断"
			now := time.Now()
			j.EndedAt = &now
		}
		m.jobs[j.ID] = j
		m.order = append(m.order, j.ID)
	}
	m.mu.Unlock()
}

// Submit 入队一个新任务。
func (m *Manager) Submit(p Params, mode string) *Job {
	j := &Job{
		ID:        time.Now().Format("20060102_150405.000"),
		Mode:      mode,
		Status:    StatusQueued,
		Params:    p,
		CreatedAt: time.Now(),
		Total:     p.Batch,
	}
	if j.Total < 1 {
		j.Total = 1
	}
	if j.Mode == "" {
		j.Mode = "txt2img"
	}

	m.mu.Lock()
	m.jobs[j.ID] = j
	m.order = append([]string{j.ID}, m.order...)
	if len(m.order) > maxKeepJobs {
		for _, id := range m.order[maxKeepJobs:] {
			delete(m.jobs, id)
		}
		m.order = m.order[:maxKeepJobs]
	}
	m.mu.Unlock()

	m.bus.Publish("job", j)
	m.queue <- j
	m.persist()
	return j
}

// List 返回全部任务（按创建时间倒序）。
func (m *Manager) List() []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Job, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.jobs[id])
	}
	return out
}

// Get 返回单个任务。
func (m *Manager) Get(id string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[id]
}

// Delete 移除一条历史记录。
func (m *Manager) Delete(id string) {
	m.mu.Lock()
	j := m.jobs[id]
	if j != nil && (j.Status == StatusRunning || j.Status == StatusQueued) {
		m.mu.Unlock()
		return
	}
	delete(m.jobs, id)
	order := m.order[:0]
	for _, x := range m.order {
		if x != id {
			order = append(order, x)
		}
	}
	m.order = order
	m.mu.Unlock()
	m.bus.Publish("job", &Job{ID: id, Status: "deleted"})
	m.persist()
}

// Cancel 取消正在运行或排队中的任务。
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	j := m.jobs[id]
	if j == nil {
		m.mu.Unlock()
		return false
	}
	if j.Status == StatusRunning {
		j.Status = StatusCanceled
		if m.cancel != nil {
			m.cancel()
		}
	}
	if j.Status == StatusQueued {
		j.Status = StatusCanceled
		now := time.Now()
		j.EndedAt = &now
		m.mu.Unlock()
		m.bus.Publish("job", j)
		m.persist()
		return true
	}
	m.mu.Unlock()
	if j.Status == StatusRunning {
		m.persist()
	}
	return j.Status == StatusRunning
}

func (m *Manager) setRunning(j *Job) {
	m.mu.Lock()
	m.running = j
	j.Status = StatusRunning
	now := time.Now()
	j.StartedAt = &now
	m.mu.Unlock()
	m.bus.Publish("job", j)
}

func (m *Manager) finish(j *Job, status, errMsg string) {
	m.mu.Lock()
	j.Status = status
	j.Error = errMsg
	now := time.Now()
	j.EndedAt = &now
	if j.StartedAt != nil {
		j.ElapsedMs = now.Sub(*j.StartedAt).Milliseconds()
	}
	if status == StatusSuccess {
		j.Progress = 100
	}
	m.running = nil
	m.cmd = nil
	m.cancel = nil
	m.mu.Unlock()
	m.bus.Publish("job", j)
	m.persist()
}

func (m *Manager) worker() {
	for j := range m.queue {
		m.mu.Lock()
		if j.Status == StatusCanceled {
			m.mu.Unlock()
			continue
		}
		m.mu.Unlock()
		m.run(j)
	}
}

// BuildArgs 把参数翻译成 zimage-ncnn-vulkan 的命令行。
func BuildArgs(j *Job, outPath string) []string {
	p := j.Params
	a := []string{}

	prompt := strings.TrimSpace(p.Prompt)
	if prompt == "" {
		prompt = "rand"
	}
	a = append(a, "-p", prompt)

	if strings.TrimSpace(p.Negative) != "" {
		a = append(a, "-n", strings.TrimSpace(p.Negative))
	}
	a = append(a, "-o", outPath)
	// 扩图：引擎按「原图 + -x 边距」自行推导画布，README 示例也不传 -s，
	// 传入与扩展后尺寸不符的 -s 会导致引擎崩溃，因此该模式跳过 -s。
	if p.Width > 0 && p.Height > 0 && j.Mode != "outpaint" {
		a = append(a, "-s", fmt.Sprintf("%d,%d", p.Width, p.Height))
	}
	if p.Steps > 0 {
		a = append(a, "-l", strconv.Itoa(p.Steps))
	}
	if p.Seed >= 0 {
		a = append(a, "-r", strconv.FormatInt(p.Seed, 10))
	}
	if strings.TrimSpace(p.ModelPath) != "" {
		a = append(a, "-m", strings.TrimSpace(p.ModelPath))
	}
	if p.GPUID != -2 {
		a = append(a, "-g", strconv.Itoa(p.GPUID))
	}
	if p.Batch > 1 {
		a = append(a, "-b", strconv.Itoa(p.Batch))
	}

	switch j.Mode {
	case "inpaint":
		if p.InputImage != "" {
			a = append(a, "-i", p.InputImage)
		}
		if p.MaskImage != "" {
			a = append(a, "-k", p.MaskImage)
		}
	case "outpaint":
		if p.InputImage != "" {
			a = append(a, "-i", p.InputImage)
		}
		if strings.TrimSpace(p.Outpaint) != "" {
			a = append(a, "-x", strings.TrimSpace(p.Outpaint))
		}
	case "controlnet", "tile":
		if p.ControlImage != "" {
			a = append(a, "-c", p.ControlImage)
		}
		if p.ControlScale > 0 {
			a = append(a, "-w", fmt.Sprintf("%.2f", p.ControlScale))
		}
		if j.Mode == "tile" {
			a = append(a, "-t")
		}
	}
	return a
}

var (
	reStepSlash = regexp.MustCompile(`(?i)(?:step|it|iter)[^\d]{0,4}(\d{1,4})\s*/\s*(\d{1,4})`)
	rePercent   = regexp.MustCompile(`(^|\s)(\d{1,3})\s*%`)
	reVaeDone   = regexp.MustCompile(`(?i)^vae\s+done`)
)

// progTracker 把引擎的 step 输出换算成整体百分比；批量 (-b) 时按张累计。
type progTracker struct {
	imgTotal int
	tot, cur int
	img      int // 当前是第几张（从 0 开始）
	pct      int
}

func newProgTracker(imgTotal int) *progTracker {
	if imgTotal < 1 {
		imgTotal = 1
	}
	return &progTracker{imgTotal: imgTotal}
}

func (t *progTracker) step(cur, tot int) {
	if tot <= 0 || cur <= 0 {
		return
	}
	if cur == 1 && t.cur > 1 {
		t.img++ // 引擎重新从 step 1/ 开始 → 进入下一张
	}
	if tot != t.tot {
		t.tot = tot
	}
	t.cur = cur
	overall := (float64(t.img) + float64(cur)/float64(tot)) / float64(t.imgTotal)
	p := int(overall * 100)
	if p > 100 {
		p = 100
	}
	t.pct = p
}

func (t *progTracker) done() {
	t.pct = 100
}

// parseProgress 返回一行输出对应的整体进度。
func (t *progTracker) parseProgress(line string) (int, bool) {
	if m := reStepSlash.FindStringSubmatch(line); len(m) == 3 {
		cur, _ := strconv.Atoi(m[1])
		tot, _ := strconv.Atoi(m[2])
		t.step(cur, tot)
		return t.pct, true
	}
	if reVaeDone.MatchString(line) {
		t.done()
		return t.pct, true
	}
	if m := rePercent.FindStringSubmatch(line); len(m) == 3 {
		v, _ := strconv.Atoi(m[2])
		if v >= 0 && v <= 100 {
			t.pct = v
			return t.pct, true
		}
	}
	return 0, false
}

func (m *Manager) run(j *Job) {
	cfg := GetConfig()
	outPath := filepath.Join(cfg.OutputDir, j.ID+".png")
	args := BuildArgs(j, outPath)
	j.Command = quoteCommand(cfg.ExePath, args)

	m.setRunning(j)
	m.emit(j, "$ "+j.Command)

	if _, err := os.Stat(cfg.ExePath); err != nil {
		m.emit(j, "[错误] 找不到可执行文件: "+cfg.ExePath)
		m.finish(j, StatusFailed, "找不到可执行文件: "+cfg.ExePath)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancel = cancel
	m.mu.Unlock()

	cmd := exec.CommandContext(ctx, cfg.ExePath, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Env = append(os.Environ(), "NCNN_VULKAN=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		m.finish(j, StatusFailed, err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		m.finish(j, StatusFailed, err.Error())
		return
	}

	if err := cmd.Start(); err != nil {
		cancel()
		m.emit(j, "[错误] "+err.Error())
		m.finish(j, StatusFailed, err.Error())
		return
	}
	m.mu.Lock()
	m.cmd = cmd
	m.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	tr := newProgTracker(j.Total)
	lineFn := func(line string) {
		line = strings.TrimRight(line, " \t\r\n")
		if line == "" {
			return
		}
		m.emit(j, line)
		if p, ok := tr.parseProgress(line); ok {
			m.mu.Lock()
			j.Progress = p
			m.mu.Unlock()
			m.bus.Publish("job", j)
		}
	}
	go func() { defer wg.Done(); pump(stdout, lineFn) }()
	go func() { defer wg.Done(); pump(stderr, lineFn) }()

	runErr := cmd.Wait()
	wg.Wait()
	cancel()

	if runErr != nil {
		if j.Status == StatusCanceled {
			m.finish(j, StatusCanceled, "已取消")
			return
		}
		msg := runErr.Error()
		m.emit(j, "[错误] "+msg)
		m.finish(j, StatusFailed, msg)
		return
	}

	outs := collectOutputs(cfg.OutputDir, j.ID)
	if len(outs) == 0 {
		m.emit(j, "[警告] 任务结束但未检测到输出文件")
		m.finish(j, StatusFailed, "未检测到输出文件")
		return
	}
	if j.StartedAt != nil {
		m.mu.Lock()
		j.ElapsedMs = time.Since(*j.StartedAt).Milliseconds()
		m.mu.Unlock()
	}
	j.Outputs = outs
	saveMeta(cfg.OutputDir, j)
	m.emit(j, fmt.Sprintf("[完成] 输出 %d 个文件，用时 %.1fs", len(outs), float64(j.ElapsedMs)/1000))
	m.finish(j, StatusSuccess, "")
}

func (m *Manager) emit(j *Job, line string) {
	ts := time.Now().Format("15:04:05")
	m.bus.Publish("log", map[string]interface{}{
		"id":   j.ID,
		"line": fmt.Sprintf("[%s] %s", ts, line),
	})
}

// pump 按行（同时兼容 \r 进度回车）读取子进程输出。
func pump(r io.Reader, fn func(string)) {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		chunk, err := br.ReadString('\n')
		if len(chunk) > 0 {
			for _, part := range strings.Split(chunk, "\r") {
				part = strings.TrimRight(part, "\n")
				if strings.TrimSpace(part) != "" {
					fn(part)
				} else if part != "" {
					fn(part)
				}
			}
		}
		if err != nil {
			if err != io.EOF && strings.TrimSpace(chunk) != "" {
				fn(chunk)
			}
			return
		}
	}
}

// collectOutputs 收集本次任务产出的图片文件（兼容 -b 批量命名）。
func collectOutputs(dir, id string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, id) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func quoteCommand(exe string, args []string) string {
	parts := []string{strconv.Quote(exe)}
	for _, a := range args {
		if strings.ContainsAny(a, " \t\"") {
			parts = append(parts, strconv.Quote(a))
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}
