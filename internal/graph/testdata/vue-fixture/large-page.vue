<template>
  <div class="dashboard">
    <header class="dashboard-header">
      <h1>{{ pageTitle }}</h1>
      <nav>
        <router-link to="/overview">Overview</router-link>
        <router-link to="/analytics">Analytics</router-link>
        <router-link to="/settings">Settings</router-link>
      </nav>
    </header>
    <main class="dashboard-content">
      <StatsPanel :stats="dashboardStats" />
      <ChartWidget :data="chartData" :options="chartOptions" />
      <ActivityFeed :items="recentActivity" />
      <DataTable :rows="tableRows" :columns="tableColumns" @sort="handleSort" />
    </main>
    <footer class="dashboard-footer">
      <p>Last updated: {{ lastUpdated }}</p>
    </footer>
  </div>
</template>

<script>
export default {
  name: 'DashboardPage',
  inheritAttrs: false,
}
</script>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useDashboardStore } from '../stores/dashboard'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notifications'
import { fetchDashboardStats } from '../api/stats'
import { fetchChartData } from '../api/charts'
import { fetchActivityFeed } from '../api/activity'
import { formatCurrency, formatNumber, formatDate } from '../utils/format'
import { debounce, throttle } from '../utils/performance'
import { logger } from '../utils/logger'
import StatsPanel from '../components/StatsPanel.vue'
import ChartWidget from '../components/ChartWidget.vue'
import ActivityFeed from '../components/ActivityFeed.vue'
import DataTable from '../components/DataTable.vue'
import ExportButton from '../components/ExportButton.vue'
import FilterPanel from '../components/FilterPanel.vue'
import DateRangePicker from '../components/DateRangePicker.vue'
import LoadingSpinner from '../components/LoadingSpinner.vue'
import ErrorBoundary from '../components/ErrorBoundary.vue'
import NotificationToast from '../components/NotificationToast.vue'

interface DashboardStats {
  totalRevenue: number
  activeUsers: number
  conversionRate: number
  avgSessionDuration: number
  bounceRate: number
  pageViews: number
}

interface ChartDataPoint {
  date: string
  value: number
  label: string
  category: string
}

interface ActivityItem {
  id: string
  type: 'login' | 'purchase' | 'signup' | 'upgrade' | 'churn'
  userId: string
  userName: string
  timestamp: string
  metadata: Record<string, unknown>
}

interface TableColumn {
  key: string
  label: string
  sortable: boolean
  formatter?: (value: unknown) => string
}

interface FilterState {
  dateRange: { start: Date; end: Date }
  categories: string[]
  minRevenue: number
  maxRevenue: number
  status: 'all' | 'active' | 'inactive'
}

type SortDirection = 'asc' | 'desc'

const router = useRouter()
const route = useRoute()
const dashboardStore = useDashboardStore()
const authStore = useAuthStore()
const notificationStore = useNotificationStore()

const isLoading = ref(true)
const hasError = ref(false)
const errorMessage = ref('')
const pageTitle = ref('Dashboard')
const lastUpdated = ref('')
const refreshInterval = ref<ReturnType<typeof setInterval> | null>(null)

const dashboardStats = ref<DashboardStats>({
  totalRevenue: 0,
  activeUsers: 0,
  conversionRate: 0,
  avgSessionDuration: 0,
  bounceRate: 0,
  pageViews: 0,
})

const chartData = ref<ChartDataPoint[]>([])
const recentActivity = ref<ActivityItem[]>([])
const tableRows = ref<Record<string, unknown>[]>([])

const filters = ref<FilterState>({
  dateRange: {
    start: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000),
    end: new Date(),
  },
  categories: [],
  minRevenue: 0,
  maxRevenue: Infinity,
  status: 'all',
})

const sortColumn = ref('timestamp')
const sortDirection = ref<SortDirection>('desc')

const tableColumns: TableColumn[] = [
  { key: 'userName', label: 'User', sortable: true },
  { key: 'type', label: 'Event', sortable: true },
  { key: 'revenue', label: 'Revenue', sortable: true, formatter: (v) => formatCurrency(v as number) },
  { key: 'timestamp', label: 'Time', sortable: true, formatter: (v) => formatDate(v as string) },
]

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top' },
    tooltip: { enabled: true, mode: 'index' },
  },
  scales: {
    x: { display: true, grid: { display: false } },
    y: { display: true, beginAtZero: true },
  },
}))

const filteredActivity = computed(() => {
  return recentActivity.value.filter(item => {
    if (filters.value.categories.length > 0 && !filters.value.categories.includes(item.type)) {
      return false
    }
    if (item.timestamp < filters.value.dateRange.start.toISOString()) return false
    if (item.timestamp > filters.value.dateRange.end.toISOString()) return false
    return true
  })
})

const sortedTableRows = computed(() => {
  const rows = [...tableRows.value]
  rows.sort((a, b) => {
    const aVal = a[sortColumn.value]
    const bVal = b[sortColumn.value]
    const cmp = String(aVal).localeCompare(String(bVal))
    return sortDirection.value === 'asc' ? cmp : -cmp
  })
  return rows
})

function handleSort(column: string) {
  if (sortColumn.value === column) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortColumn.value = column
    sortDirection.value = 'asc'
  }
}

function handleFilterChange(newFilters: Partial<FilterState>) {
  filters.value = { ...filters.value, ...newFilters }
  applyFilters()
}

function applyFilters() {
  tableRows.value = filteredActivity.value.map(item => ({
    ...item,
    revenue: (item.metadata as Record<string, unknown>)?.revenue ?? 0,
  }))
  updateChart()
}

function updateChart() {
  const grouped: Record<string, number> = {}
  for (const item of filteredActivity.value) {
    const date = item.timestamp.split('T')[0]
    grouped[date] = (grouped[date] ?? 0) + 1
  }
  chartData.value = Object.entries(grouped).map(([date, value]) => ({
    date,
    value,
    label: formatDate(date),
    category: 'activity',
  }))
}

async function loadDashboardData() {
  isLoading.value = true
  hasError.value = false
  try {
    const [stats, activity] = await Promise.all([
      fetchDashboardStats(filters.value),
      fetchActivityFeed(filters.value),
    ])
    dashboardStats.value = stats
    recentActivity.value = activity
    applyFilters()
    lastUpdated.value = formatDate(new Date().toISOString())
    logger.info('Dashboard data loaded', { stats, activityCount: activity.length })
  } catch (err) {
    hasError.value = true
    errorMessage.value = err instanceof Error ? err.message : 'Unknown error'
    notificationStore.showError('Failed to load dashboard data')
    logger.error('Dashboard load failed', err)
  } finally {
    isLoading.value = false
  }
}

async function loadChartData() {
  try {
    const data = await fetchChartData(filters.value)
    chartData.value = data
  } catch (err) {
    logger.error('Chart data load failed', err)
  }
}

function handleRefresh() {
  loadDashboardData()
  loadChartData()
}

function handleExport(format: 'csv' | 'pdf' | 'xlsx') {
  logger.info('Exporting dashboard', { format })
  notificationStore.showInfo(`Exporting as ${format.toUpperCase()}...`)
  exportDashboard(format)
}

async function exportDashboard(format: string) {
  try {
    const response = await fetch(`/api/dashboard/export?format=${format}`)
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `dashboard.${format}`
    a.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    notificationStore.showError('Export failed')
    logger.error('Export failed', err)
  }
}

function handleResize() {
  nextTick(() => {
    updateChart()
  })
}

const debouncedResize = debounce(handleResize, 300)
const throttledScroll = throttle(() => {
  console.log('scroll')
}, 100)

onMounted(async () => {
  await loadDashboardData()
  await loadChartData()
  window.addEventListener('resize', debouncedResize)
  window.addEventListener('scroll', throttledScroll)
  refreshInterval.value = setInterval(handleRefresh, 5 * 60 * 1000)
})

onUnmounted(() => {
  window.removeEventListener('resize', debouncedResize)
  window.removeEventListener('scroll', throttledScroll)
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
})

watch(
  () => route.params,
  () => {
    loadDashboardData()
  }
)

watch(
  filters,
  () => {
    applyFilters()
  },
  { deep: true }
)
</script>

<style scoped>
.dashboard { display: flex; flex-direction: column; min-height: 100vh; }
.dashboard-header { padding: 16px 24px; border-bottom: 1px solid #e0e0e0; }
.dashboard-content { flex: 1; padding: 24px; }
.dashboard-footer { padding: 16px 24px; border-top: 1px solid #e0e0e0; text-align: center; }
</style>
