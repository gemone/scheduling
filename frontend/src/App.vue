<template>
  <div id="app">
    <!-- Header -->
    <div class="header">
      <h1>📋 排班表</h1>
      <div class="header-actions">
        <div class="month-nav">
          <button @click="prevMonth" title="上个月">◀</button>
          <span class="current-month">{{ year }}年{{ month }}月</span>
          <button @click="nextMonth" title="下个月">▶</button>
        </div>
        <button class="btn btn-success" @click="generateSchedule">🔄 生成排班</button>
        <button class="btn" :class="hasSchedule ? 'btn-warning' : 'btn-outline'" style="color:white;border-color:rgba(255,255,255,0.4)" @click="pinAll">
          📌 {{ allPinned ? '取消全部固定' : '保存排班' }}
        </button>
        <button class="btn btn-success" @click="exportXLSX">📄 导出排班表</button>
      </div>
    </div>

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Sidebar -->
      <div class="sidebar">
        <div class="sidebar-tabs">
          <button class="sidebar-tab" :class="{ active: tab === 'people' }" @click="tab = 'people'">👥 人员</button>
          <button class="sidebar-tab" :class="{ active: tab === 'rules' }" @click="tab = 'rules'">⚙️ 规则</button>
          <button class="sidebar-tab" :class="{ active: tab === 'vacation' }" @click="tab = 'vacation'">🏖️ 休假</button>
        </div>

        <div class="sidebar-content">
          <!-- People Tab -->
          <div v-if="tab === 'people'">
            <div class="person-toolbar">
              <button class="btn btn-primary" style="flex:1" @click="openPersonModal(null)">➕ 添加人员</button>
              <button class="btn btn-outline" @click="importPeople" title="导入人员">📥</button>
              <button class="btn btn-outline" @click="exportPeople" title="导出人员">📤</button>
            </div>

            <div class="person-list" style="margin-top:12px">
              <div class="person-card" v-for="p in data.people" :key="p.id">
                <button class="remove-btn" @click="removePerson(p.id)">✕</button>
                <button class="edit-btn" @click="openPersonModal(p)" title="编辑">✎</button>
                <div class="name">{{ p.name }}</div>
                <div class="limits">
                  <span v-if="p.min_total > 0" style="color:var(--primary)">✦{{ p.min_total }}</span>
                  <span>总计{{ p.max_total }}次</span>
                  <span class="shift-tag day-tag" :class="{ dim: !p.day_shift_pos }">☀白{{ p.day_shift_pos ? p.max_day : 'x' }}</span>
                  <span class="shift-tag night-tag" :class="{ dim: !p.night_shift_pos }">🌙夜{{ p.night_shift_pos ? p.max_night : 'x' }}</span>
                  <span v-if="p.weekend_day_shift_pos || p.weekend_night_shift_pos" class="shift-tag weekend-tag">{{ p.weekend_day_shift_pos ? '☀' : '' }}{{ p.weekend_night_shift_pos ? '🌙' : '' }}</span>
                  <span v-if="p.holiday_day_shift_pos || p.holiday_night_shift_pos" class="shift-tag holiday-tag-sm">{{ p.holiday_day_shift_pos ? 'w' : '' }}{{ p.holiday_night_shift_pos ? 'n' : '' }}</span>
                </div>
              </div>
              <div v-if="data.people.length === 0" class="empty-state">
                <div class="icon">👥</div>
                <p>暂无人员，请添加</p>
              </div>
            </div>
          </div>

          <!-- Rules Tab -->
          <div v-if="tab === 'rules'">
            <div style="font-weight:600;margin-bottom:8px">💼 工作日</div>
            <div class="form-group">
              <label>白班人数</label>
              <input v-model.number="globalRules.day_shift_per_day" type="number" min="0" @change="saveGlobalRules" />
            </div>
            <div class="form-group">
              <label>夜班人数</label>
              <input v-model.number="globalRules.night_shift_per_day" type="number" min="0" @change="saveGlobalRules" />
            </div>

            <div style="font-weight:600;margin:12px 0 8px;border-top:1px solid var(--border-color);padding-top:12px">📅 周末（周六日）</div>
            <div class="form-group">
              <label>白班人数</label>
              <input v-model.number="globalRules.weekend_day_shift" type="number" min="0" @change="saveGlobalRules" />
            </div>
            <div class="form-group">
              <label>夜班人数</label>
              <input v-model.number="globalRules.weekend_night_shift" type="number" min="0" @change="saveGlobalRules" />
            </div>

            <div style="font-weight:600;margin:12px 0 8px;border-top:1px solid var(--border-color);padding-top:12px">🎆 法定节假日</div>
            <div class="form-group">
              <label>白班人数</label>
              <input v-model.number="globalRules.holiday_day_shift" type="number" min="0" @change="saveGlobalRules" />
            </div>
            <div class="form-group">
              <label>夜班人数</label>
              <input v-model.number="globalRules.holiday_night_shift" type="number" min="0" @change="saveGlobalRules" />
            </div>
            <div style="font-size:11px;color:var(--text-secondary);margin-top:2px">💡 在日历上点击日期左上角图标可设定节假日/工作日</div>

            <div style="margin-top:16px;font-size:12px;color:var(--text-secondary)">
              <p>💡 规则说明：</p>
              <ul style="margin-left:16px;margin-top:4px;line-height:1.8">
                <li>系统自动均衡每人排班次数</li>
                <li>优先排班次数少的人员</li>
                <li>✦ 强制排满的人员会被优先安排</li>
                <li>前一天夜班 → 第二天不能白班</li>
                <li>休假日期自动跳过</li>
                <li>规避日期不排班但不标记休假</li>
                <li>📌 固定的天不会被重新排班</li>
                <li>法定节假日优先于周末规则</li>
                <li>在日历上点击日期图标可设定节假日或工作日覆盖</li>
              </ul>
            </div>
          </div>

          <!-- Vacation Tab -->
          <div v-if="tab === 'vacation'">
            <div class="form-group">
              <label>选择人员</label>
              <select v-model="newVacation.personId">
                <option value="">请选择</option>
                <option v-for="p in data.people" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </div>
            <div class="form-group">
              <label>日期范围</label>
              <input type="date" v-model="newVacation.startDate" />
              <input type="date" v-model="newVacation.endDate" style="margin-top:4px" />
            </div>
            <div class="form-group">
              <label>类型</label>
              <div class="checkbox-group">
                <label><input type="radio" v-model="newVacation.type" value="vacation" /> 休假</label>
                <label><input type="radio" v-model="newVacation.type" value="avoid" /> 规避</label>
              </div>
            </div>
            <button class="btn btn-primary btn-block" @click="addVacation">➕ 添加</button>

            <div style="margin-top:16px">
              <div class="vacation-item" v-for="(v, idx) in data.vacations" :key="idx">
                <span>{{ getPersonName(v.person_id) }}</span>
                <span class="date-badge">{{ v.date }}</span>
                <span style="font-size:11px;color:var(--text-secondary)">{{ v.type === 'vacation' ? '休假' : '规避' }}</span>
                <button class="btn btn-sm btn-danger" @click="removeVacation(idx)">✕</button>
              </div>
              <div v-if="data.vacations.length === 0" class="empty-state">
                <div class="icon">🏖️</div>
                <p>暂无休假记录</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Calendar Area -->
      <div class="calendar-area">
        <!-- Stats -->
        <div class="stats-bar" v-if="data.schedule.length > 0">
          <h3>📊 排班统计</h3>
          <div class="stats-grid">
            <div class="stat-card" v-for="p in data.people" :key="'stat-'+p.id">
              <div class="stat-name">{{ p.name }}</div>
              <div class="stat-detail">
                总计 {{ getPersonStats(p.name).total }}/{{ p.max_total }}次
                <span v-if="p.min_total > 0" :style="{ color: getPersonStats(p.name).total < p.min_total ? 'var(--danger)' : 'var(--success)' }">
                  (最低{{ p.min_total }})
                </span>
                · 白 {{ getPersonStats(p.name).day }}/{{ p.max_day }}
                · 夜 {{ getPersonStats(p.name).night }}/{{ p.max_night }}
              </div>
            </div>
          </div>
        </div>

        <!-- Legend -->
        <div class="calendar-header">
          <h2>{{ year }}年{{ month }}月排班表</h2>
          <div class="legend">
            <div class="legend-item">
              <div class="legend-dot" style="background:var(--day-bg)"></div>
              <span>白班</span>
            </div>
            <div class="legend-item">
              <div class="legend-dot" style="background:var(--night-bg)"></div>
              <span>夜班</span>
            </div>
            <div class="legend-item">
              <div class="legend-dot" style="background:var(--off-bg)"></div>
              <span>休</span>
            </div>
            <div class="legend-item" v-if="pinnedCount > 0">
              <div class="legend-dot" style="background:var(--pinned-bg)"></div>
              <span>📌 已固定 {{ pinnedCount }}天</span>
            </div>
          </div>
        </div>

        <!-- Calendar Grid -->
        <div class="calendar-grid">
          <div class="calendar-weekday" v-for="w in weekdays" :key="w">{{ w }}</div>
          <div v-for="offset in firstDayOffset" :key="'e-'+offset"></div>
          <div
            class="calendar-day"
            :class="{
              weekend: isWeekend(day),
              pinned: isDayPinned(day),
              'drag-over': dragOverDay === day
            }"
            v-for="day in daysInMonth"
            :key="day"
            @dragover.prevent="!isDayPinned(day) && onDragOver(day, $event)"
            @dragleave="onDragLeave(day)"
            @drop="!isDayPinned(day) && onDrop(day, $event)"
          >
            <div class="day-header">
              <span class="day-number" :class="{ 'weekend-num': isWeekend(day) && !isHoliday(day) && !isWorkdayOverride(day), 'holiday-num': isHoliday(day), 'workday-override-num': isWorkdayOverride(day) }">{{ day }}</span>
              <button
                class="day-type-btn"
                @click="cycleDayType(day)"
                :title="dayTypeLabel(day)"
              >{{ dayTypeIcon(day) }}</button>
              <button
                class="pin-btn"
                :class="{ pinned: isDayPinned(day) }"
                @click="togglePin(day)"
                :title="isDayPinned(day) ? '取消固定' : '固定此天'"
              >📌</button>
            </div>
            <template v-for="(entry, idx) in getDayEntriesSorted(day)" :key="entry.person + '-' + idx">
              <span
                class="shift-badge"
                :class="[shiftClass(entry.shift_type), { locked: isDayPinned(day) }]"
                :draggable="!isDayPinned(day)"
                @dragstart="!isDayPinned(day) && onDragStart(day, entry, $event)"
                @click="!isDayPinned(day) && openEditModal(day, entry)"
              >
                {{ entry.person }} {{ entry.shift_type }}
              </span>
            </template>
            <button v-if="data.people.length > 0 && !isDayPinned(day)" class="add-shift-btn" @click="openAddModal(day)" title="添加排班">+</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Edit Modal -->
    <div class="modal-overlay" v-if="editModal.show" @click.self="editModal.show = false">
      <div class="modal">
        <h3>{{ editModal.isAdd ? '添加排班' : '编辑排班' }} - {{ editModal.date }}</h3>
        <div class="form-group">
          <label>人员</label>
          <select v-model="editModal.person" :disabled="!editModal.isAdd">
            <option v-for="p in data.people" :key="p.id" :value="p.name">{{ p.name }}</option>
          </select>
        </div>
        <div class="form-group">
          <label>班次</label>
          <div class="checkbox-group">
            <label><input type="radio" v-model="editModal.shiftType" value="白班" /> ☀️ 白班</label>
            <label><input type="radio" v-model="editModal.shiftType" value="夜班" /> 🌙 夜班</label>
            <label><input type="radio" v-model="editModal.shiftType" value="休" /> 😴 休</label>
          </div>
        </div>
        <div class="modal-actions">
          <button v-if="!editModal.isAdd" class="btn btn-danger" @click="deleteShift">🗑️ 删除</button>
          <div style="flex:1"></div>
          <button class="btn btn-outline" @click="editModal.show = false">取消</button>
          <button class="btn btn-primary" @click="saveShift">💾 保存</button>
        </div>
      </div>
    </div>

    <!-- Person Modal -->
    <div class="modal-overlay" v-if="personModal.show" @click.self="personModal.show = false">
      <div class="modal">
        <h3>{{ personModal.isAdd ? '添加人员' : '编辑人员' }}</h3>
        <div class="form-group">
          <label>姓名</label>
          <input v-model="personModal.name" placeholder="输入姓名" @keyup.enter="savePersonModal" />
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>强制排满次数</label>
            <input v-model.number="personModal.minTotal" type="number" min="0" />
          </div>
          <div class="form-group">
            <label>月最大总班次</label>
            <input v-model.number="personModal.maxTotal" type="number" min="1" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label>月最大白班</label>
            <input v-model.number="personModal.maxDay" type="number" min="0" />
          </div>
          <div class="form-group">
            <label>月最大夜班</label>
            <input v-model.number="personModal.maxNight" type="number" min="0" />
          </div>
        </div>
        <div class="form-group">
          <label>可值班类型</label>
          <div class="checkbox-group" style="flex-direction:column;gap:6px">
            <div style="font-size:11px;color:var(--text-secondary)">工作日</div>
            <div class="checkbox-row">
              <label><input type="checkbox" v-model="personModal.dayShiftPos" :true-value="1" :false-value="0" /> ☀白班</label>
              <label><input type="checkbox" v-model="personModal.nightShiftPos" :true-value="1" :false-value="0" /> 🌙夜班</label>
            </div>
            <div style="font-size:11px;color:var(--text-secondary)">周末</div>
            <div class="checkbox-row">
              <label><input type="checkbox" v-model="personModal.weekendDayShiftPos" :true-value="1" :false-value="0" /> ☀白班</label>
              <label><input type="checkbox" v-model="personModal.weekendNightShiftPos" :true-value="1" :false-value="0" /> 🌙夜班</label>
            </div>
            <div style="font-size:11px;color:var(--text-secondary)">节假日</div>
            <div class="checkbox-row">
              <label><input type="checkbox" v-model="personModal.holidayDayShiftPos" :true-value="1" :false-value="0" /> ☀白班</label>
              <label><input type="checkbox" v-model="personModal.holidayNightShiftPos" :true-value="1" :false-value="0" /> 🌙夜班</label>
            </div>
          </div>
        </div>
        <div class="modal-actions">
          <div style="flex:1"></div>
          <button class="btn btn-outline" @click="personModal.show = false">取消</button>
          <button class="btn btn-primary" @click="savePersonModal">💾 保存</button>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <div class="toast" v-if="toastMessage">{{ toastMessage }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'

// @ts-ignore - Wails bindings
import {
  LoadMonthData,
  SaveMonthData,
  LoadPeople,
  SavePeople,
  LoadRules,
  SaveRules,
  GenerateSchedule,
  ExportXLSX,
  OpenFile,
  UpdateShiftEntry
} from '../wailsjs/go/main/App'

interface Person {
  id: string
  name: string
  min_total: number
  max_total: number
  max_day: number
  max_night: number
  day_shift_pos: number
  night_shift_pos: number
  weekend_day_shift_pos: number
  weekend_night_shift_pos: number
  holiday_day_shift_pos: number
  holiday_night_shift_pos: number
}

interface Vacation {
  person_id: string
  date: string
  type: string
}

interface ShiftEntry {
  date: string
  person: string
  shift_type: string
}

interface ScheduleRule {
  day_shift_per_day: number
  night_shift_per_day: number
  weekend_day_shift: number
  weekend_night_shift: number
  holiday_day_shift: number
  holiday_night_shift: number
}

interface MonthData {
  people: Person[]
  vacations: Vacation[]
  rules: ScheduleRule
  schedule: ShiftEntry[]
  pinned_days: string[]
  day_types: Record<string, string>  // YYYY-MM-DD -> "holiday" | "workday"
  year: number
  month: number
}

const weekdays = ['一', '二', '三', '四', '五', '六', '日']

const now = new Date()
const year = ref(now.getFullYear())
const month = ref(now.getMonth() + 1)
const tab = ref('people')
const toastMessage = ref('')
let toastTimer: ReturnType<typeof setTimeout> | null = null

// Global people (shared across all months)
const globalPeople = ref<Person[]>([])

// Global rules (shared across all months)
const globalRules = ref<ScheduleRule>({
  day_shift_per_day: 1,
  night_shift_per_day: 1,
  weekend_day_shift: 1,
  weekend_night_shift: 1,
  holiday_day_shift: 1,
  holiday_night_shift: 1,
})

// Per-month data (no people inside)
const data = ref<MonthData>({
  people: [],
  vacations: [],
  rules: { day_shift_per_day: 1, night_shift_per_day: 1, weekend_day_shift: 1, weekend_night_shift: 1, holiday_day_shift: 1, holiday_night_shift: 1 },
  schedule: [],
  pinned_days: [],
  day_types: {},
  year: year.value,
  month: month.value,
})
// Sync: data.rules always points to globalRules
data.value.rules = globalRules.value

const personModal = ref({
  show: false,
  isAdd: true,
  editId: null as string | null,
  name: '',
  minTotal: 0,
  maxTotal: 22,
  maxDay: 15,
  maxNight: 10,
  dayShiftPos: 1,
  nightShiftPos: 1,
  weekendDayShiftPos: 1,
  weekendNightShiftPos: 1,
  holidayDayShiftPos: 1,
  holidayNightShiftPos: 1,
})

const newVacation = ref({
  personId: '',
  startDate: '',
  endDate: '',
  type: 'vacation',
})

// Edit modal state
const editModal = ref<{
  show: boolean
  date: string
  person: string
  shiftType: string
  isAdd: boolean
}>({
  show: false,
  date: '',
  person: '',
  shiftType: '白班',
  isAdd: false,
})

// Drag state
const dragOverDay = ref<number | null>(null)
const dragData = ref<{ fromDay: number; entry: ShiftEntry } | null>(null)

// Calendar computations
const daysInMonth = computed(() => {
  return new Date(year.value, month.value, 0).getDate()
})

const firstDayOffset = computed(() => {
  const d = new Date(year.value, month.value - 1, 1).getDay()
  return d === 0 ? 6 : d - 1
})

const hasSchedule = computed(() => data.value.schedule.length > 0)

const allPinned = computed(() => {
  if (data.value.schedule.length === 0) return false
  const total = daysInMonth.value
  return data.value.pinned_days.length >= total
})

const pinnedCount = computed(() => data.value.pinned_days.length)

function isWeekend(day: number): boolean {
  const d = new Date(year.value, month.value - 1, day).getDay()
  return d === 0 || d === 6
}

function isHoliday(day: number): boolean {
  return (data.value.day_types || {})[dateStr(day)] === 'holiday'
}

function getEffectiveDayType(day: number): string {
  const ds = dateStr(day)
  const dt = (data.value.day_types || {})[ds]
  if (dt) return dt  // "holiday" or "workday"
  return isWeekend(day) ? 'weekend' : 'workday'
}

function cycleDayType(day: number) {
  const ds = dateStr(day)
  if (!data.value.day_types) data.value.day_types = {}
  const current = data.value.day_types[ds]
  if (!current) {
    data.value.day_types[ds] = 'holiday'
  } else if (current === 'holiday') {
    if (isWeekend(day)) {
      data.value.day_types[ds] = 'workday'
    } else {
      delete data.value.day_types[ds]
    }
  } else {
    delete data.value.day_types[ds]
  }
  saveData()
}

function isWorkdayOverride(day: number): boolean {
  return (data.value.day_types || {})[dateStr(day)] === 'workday'
}

function dayTypeIcon(day: number): string {
  const t = getEffectiveDayType(day)
  if (t === 'holiday') return '🎆'
  if (t === 'weekend') return '📅'
  return '💼'
}

function dayTypeLabel(day: number): string {
  const t = getEffectiveDayType(day)
  const overrides = (data.value.day_types || {})[dateStr(day)]
  if (t === 'holiday') return overrides ? '节假日（手动设定）点击切换' : '节假日'
  if (t === 'weekend') return '周末'
  return overrides ? '工作日（手动设定）点击切换' : '工作日'
}

function dateStr(day: number): string {
  return `${year.value}-${String(month.value).padStart(2, '0')}-${String(day).padStart(2, '0')}`
}

function isDayPinned(day: number): boolean {
  return data.value.pinned_days.includes(dateStr(day))
}

function getDayEntries(day: number): ShiftEntry[] {
  const ds = dateStr(day)
  return data.value.schedule.filter((e: ShiftEntry) => e.date === ds)
}

function getDayEntriesSorted(day: number): ShiftEntry[] {
  const entries = getDayEntries(day)
  const order: Record<string, number> = { '白班': 0, '夜班': 1, '休': 2 }
  return [...entries].sort((a, b) => (order[a.shift_type] ?? 3) - (order[b.shift_type] ?? 3))
}

function shiftClass(type: string): string {
  if (type === '白班') return 'day'
  if (type === '夜班') return 'night'
  return 'off'
}

function getPersonName(id: string): string {
  const p = data.value.people.find((p: Person) => p.id === id)
  return p ? p.name : id
}

function getPersonStats(name: string) {
  const entries = data.value.schedule.filter((e: ShiftEntry) => e.person === name)
  const day = entries.filter((e: ShiftEntry) => e.shift_type === '白班').length
  const night = entries.filter((e: ShiftEntry) => e.shift_type === '夜班').length
  return {
    total: day + night,
    day,
    night,
  }
}

// ==================== Pin / Unpin ====================

function togglePin(day: number) {
  const ds = dateStr(day)
  const idx = data.value.pinned_days.indexOf(ds)
  if (idx >= 0) {
    data.value.pinned_days.splice(idx, 1)
  } else {
    data.value.pinned_days.push(ds)
  }
  saveData()
}

async function pinAll() {
  if (data.value.schedule.length === 0) {
    showToast('请先生成排班')
    return
  }
  if (allPinned.value) {
    // Unpin all
    data.value.pinned_days = []
  } else {
    // Pin all days that have schedule entries
    const dates = new Set<string>()
    for (const e of data.value.schedule) {
      dates.add(e.date)
    }
    data.value.pinned_days = Array.from(dates)
  }
  await saveData()
  showToast(allPinned.value ? '📌 已取消全部固定' : '📌 已固定所有排班')
}

// ==================== Drag & Drop ====================

function onDragStart(day: number, entry: ShiftEntry, event: DragEvent) {
  dragData.value = { fromDay: day, entry }
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', JSON.stringify({ fromDay: day, entry }))
  }
}

function onDragOver(day: number, event: DragEvent) {
  if (dragData.value && dragData.value.fromDay !== day) {
    dragOverDay.value = day
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move'
    }
  }
}

function onDragLeave(day: number) {
  if (dragOverDay.value === day) {
    dragOverDay.value = null
  }
}

async function onDrop(toDay: number, event: DragEvent) {
  dragOverDay.value = null
  if (!dragData.value || dragData.value.fromDay === toDay) return

  const fromEntry = dragData.value.entry
  const fromDate = dragData.value.fromDay
  const toDate = toDay

  // Move the shift: remove from old day, add to new day
  try {
    // Remove from source
    const result1 = await UpdateShiftEntry(data.value, dateStr(fromDate), fromEntry.person, '')
    data.value.schedule = result1.schedule

    // Add to target
    const result2 = await UpdateShiftEntry(data.value, dateStr(toDate), fromEntry.person, fromEntry.shift_type)
    data.value.schedule = result2.schedule

    await saveData()
    showToast(`✅ 已将 ${fromEntry.person} 移至 ${dateStr(toDate)}`)
  } catch (e: any) {
    showToast('移动失败: ' + e)
  }
  dragData.value = null
}

// ==================== Edit Modal ====================

function openEditModal(day: number, entry: ShiftEntry) {
  editModal.value = {
    show: true,
    date: dateStr(day),
    person: entry.person,
    shiftType: entry.shift_type,
    isAdd: false,
  }
}

function openAddModal(day: number) {
  if (data.value.people.length === 0) return
  editModal.value = {
    show: true,
    date: dateStr(day),
    person: data.value.people[0].name,
    shiftType: '白班',
    isAdd: true,
  }
}

async function saveShift() {
  try {
    const result = await UpdateShiftEntry(data.value, editModal.value.date, editModal.value.person, editModal.value.shiftType)
    data.value.schedule = result.schedule
    await saveData()
    editModal.value.show = false
    showToast('✅ 已保存')
  } catch (e: any) {
    showToast('保存失败: ' + e)
  }
}

async function deleteShift() {
  try {
    const result = await UpdateShiftEntry(data.value, editModal.value.date, editModal.value.person, '')
    data.value.schedule = result.schedule
    await saveData()
    editModal.value.show = false
    showToast('🗑️ 已删除')
  } catch (e: any) {
    showToast('删除失败: ' + e)
  }
}

// ==================== Actions ====================

let personIdCounter = 0

function resetPersonModal() {
  personModal.value = {
    ...personModal.value,
    show: false,
    editId: null,
    isAdd: true,
    name: '',
    minTotal: 0,
    maxTotal: 22,
    maxDay: 15,
    maxNight: 10,
    dayShiftPos: 1,
    nightShiftPos: 1,
    weekendDayShiftPos: 1,
    weekendNightShiftPos: 1,
    holidayDayShiftPos: 1,
    holidayNightShiftPos: 1,
  }
}

function openPersonModal(p: Person | null) {
  if (p) {
    personModal.value = {
      ...personModal.value,
      show: true,
      isAdd: false,
      editId: p.id,
      name: p.name,
      minTotal: p.min_total,
      maxTotal: p.max_total,
      maxDay: p.max_day,
      maxNight: p.max_night,
      dayShiftPos: p.day_shift_pos,
      nightShiftPos: p.night_shift_pos,
      weekendDayShiftPos: p.weekend_day_shift_pos,
      weekendNightShiftPos: p.weekend_night_shift_pos,
      holidayDayShiftPos: p.holiday_day_shift_pos,
      holidayNightShiftPos: p.holiday_night_shift_pos,
    }
  } else {
    personModal.value = {
      ...personModal.value,
      show: true,
      isAdd: true,
      editId: null,
      name: '',
      minTotal: 0,
      maxTotal: 22,
      maxDay: 15,
      maxNight: 10,
      dayShiftPos: 1,
      nightShiftPos: 1,
      weekendDayShiftPos: 1,
      weekendNightShiftPos: 1,
      holidayDayShiftPos: 1,
      holidayNightShiftPos: 1,
    }
  }
}

function savePersonModal() {
  const m = personModal.value
  if (!m.name.trim()) {
    showToast('请输入姓名')
    return
  }
  if (m.isAdd) {
    const id = `p_${Date.now()}_${personIdCounter++}`
    const person: Person = {
      id,
      name: m.name.trim(),
      min_total: m.minTotal,
      max_total: m.maxTotal,
      max_day: m.maxDay,
      max_night: m.maxNight,
      day_shift_pos: m.dayShiftPos,
      night_shift_pos: m.nightShiftPos,
      weekend_day_shift_pos: m.weekendDayShiftPos,
      weekend_night_shift_pos: m.weekendNightShiftPos,
      holiday_day_shift_pos: m.holidayDayShiftPos,
      holiday_night_shift_pos: m.holidayNightShiftPos,
    }
    globalPeople.value.push(person)
    showToast('已添加人员')
  } else if (m.editId) {
    const idx = globalPeople.value.findIndex((p: Person) => p.id === m.editId)
    if (idx === -1) return
    globalPeople.value[idx] = {
      ...globalPeople.value[idx],
      name: m.name.trim(),
      min_total: m.minTotal,
      max_total: m.maxTotal,
      max_day: m.maxDay,
      max_night: m.maxNight,
      day_shift_pos: m.dayShiftPos,
      night_shift_pos: m.nightShiftPos,
      weekend_day_shift_pos: m.weekendDayShiftPos,
      weekend_night_shift_pos: m.weekendNightShiftPos,
      holiday_day_shift_pos: m.holidayDayShiftPos,
      holiday_night_shift_pos: m.holidayNightShiftPos,
    }
    showToast('已更新人员')
  }
  data.value.people = [...globalPeople.value]
  saveGlobalPeople()
  saveData()
  resetPersonModal()
}

function removePerson(id: string) {
  globalPeople.value = globalPeople.value.filter((p: Person) => p.id !== id)
  data.value.people = [...globalPeople.value]
  data.value.vacations = data.value.vacations.filter((v: Vacation) => v.person_id !== id)
  saveGlobalPeople()
  saveData()
}

function exportPeople() {
  const json = JSON.stringify(globalPeople.value, null, 2)
  const blob = new Blob([json], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'people.json'
  a.click()
  URL.revokeObjectURL(url)
  showToast('📤 已导出人员列表')
}

function importPeople() {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = '.json'
  input.onchange = async (e: Event) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    try {
      const text = await file.text()
      const imported: Person[] = JSON.parse(text)
      if (!Array.isArray(imported)) {
        showToast('格式错误：需要数组格式')
        return
      }
      // Merge: add new people by name, update existing by name
      let added = 0
      let updated = 0
      for (const p of imported) {
        if (!p.name) continue
        // Ensure ID exists
        if (!p.id) p.id = `p_${Date.now()}_${personIdCounter++}`
        const existing = globalPeople.value.findIndex((ep: Person) => ep.name === p.name)
        if (existing >= 0) {
          globalPeople.value[existing] = { ...globalPeople.value[existing], ...p }
          updated++
        } else {
          globalPeople.value.push(p)
          added++
        }
      }
      data.value.people = [...globalPeople.value]
      saveGlobalPeople()
      saveData()
      showToast(`📥 导入完成：新增${added}人，更新${updated}人`)
    } catch (err) {
      showToast('导入失败：文件格式错误')
    }
  }
  input.click()
}

function addVacation() {
  if (!newVacation.value.personId) {
    showToast('请选择人员')
    return
  }
  if (!newVacation.value.startDate) {
    showToast('请选择日期')
    return
  }

  const start = new Date(newVacation.value.startDate)
  const end = newVacation.value.endDate ? new Date(newVacation.value.endDate) : start

  if (end < start) {
    showToast('结束日期不能早于开始日期')
    return
  }

  const current = new Date(start)
  while (current <= end) {
    const dateStr = current.toISOString().slice(0, 10)
    data.value.vacations.push({
      person_id: newVacation.value.personId,
      date: dateStr,
      type: newVacation.value.type,
    })
    current.setDate(current.getDate() + 1)
  }

  newVacation.value.personId = ''
  newVacation.value.startDate = ''
  newVacation.value.endDate = ''
  saveData()
  showToast('已添加休假')
}

function removeVacation(idx: number) {
  data.value.vacations.splice(idx, 1)
  saveData()
}

async function generateSchedule() {
  if (data.value.people.length === 0) {
    showToast('请先添加人员')
    return
  }
  const hasExisting = data.value.schedule.length > 0
  if (hasExisting) {
    const choice = prompt(
      '已有排班数据，请选择：\n\n1 = 保留已固定天数，只重新排未固定的天\n2 = 全部重新排班（清除所有固定）\n3 = 放弃\n\n请输入 1、2 或 3：'
    )
    if (choice === '1') {
      // Keep pinned days, only regenerate unpinned
    } else if (choice === '2') {
      data.value.pinned_days = []
    } else {
      return
    }
  }
  try {
    const result = await GenerateSchedule({
      people: data.value.people,
      vacations: data.value.vacations,
      rules: data.value.rules,
      schedule: data.value.schedule,
      pinned_days: data.value.pinned_days,
      day_types: data.value.day_types || {},
      year: year.value,
      month: month.value,
    })
    data.value.schedule = result.schedule
    await saveData()
    showToast('✅ 排班已生成！')
  } catch (e: any) {
    showToast('生成失败: ' + e)
  }
}

async function exportXLSX() {
  try {
    const path = await ExportXLSX(data.value)
    showToast('✅ 已导出')
    await OpenFile(path)
  } catch (e: any) {
    showToast('导出失败: ' + e)
  }
}

function prevMonth() {
  month.value--
  if (month.value < 1) {
    month.value = 12
    year.value--
  }
}

function nextMonth() {
  month.value++
  if (month.value > 12) {
    month.value = 1
    year.value++
  }
}

async function loadGlobalPeople() {
  try {
    const people = await LoadPeople()
    globalPeople.value = people || []
  } catch (e: any) {
    console.error('Load people failed:', e)
  }
}

async function saveGlobalPeople() {
  try {
    await SavePeople(globalPeople.value)
  } catch (e: any) {
    console.error('Save people failed:', e)
  }
}

async function loadGlobalRules() {
  try {
    const rules = await LoadRules()
    globalRules.value = rules
    data.value.rules = globalRules.value
  } catch (e: any) {
    console.error('Load rules failed:', e)
  }
}

async function saveGlobalRules() {
  data.value.rules = { ...globalRules.value }
  try {
    await SaveRules(globalRules.value)
  } catch (e: any) {
    console.error('Save rules failed:', e)
  }
}

async function loadData() {
  try {
    const loaded = await LoadMonthData(year.value, month.value)
    data.value = loaded
    if (!data.value.pinned_days) {
      data.value.pinned_days = []
    }
    // Always use global people and rules
    data.value.people = [...globalPeople.value]
    data.value.rules = { ...globalRules.value }
  } catch (e: any) {
    console.error('Load failed:', e)
  }
}

async function saveData() {
  data.value.year = year.value
  data.value.month = month.value
  try {
    await SaveMonthData(data.value)
  } catch (e: any) {
    console.error('Save failed:', e)
  }
}

function showToast(msg: string) {
  toastMessage.value = msg
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toastMessage.value = ''
  }, 2500)
}

// Watch month change
watch([year, month], () => {
  loadData()
})

onMounted(async () => {
  await loadGlobalPeople()
  await loadGlobalRules()
  await loadData()
})
</script>
