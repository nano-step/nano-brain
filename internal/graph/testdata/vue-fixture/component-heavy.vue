<script setup lang="ts">
import { ref, computed } from 'vue'
import AppHeader from './AppHeader.vue'
import AppSidebar from './AppSidebar.vue'
import AppFooter from './AppFooter.vue'
import UserCard from './UserCard.vue'
import NotificationBell from './NotificationBell.vue'
import SearchBar from './SearchBar.vue'
import DataGrid from './DataGrid.vue'
import ChartPanel from './ChartPanel.vue'
import FilterDropdown from './FilterDropdown.vue'
import ModalDialog from './ModalDialog.vue'
import ToastNotification from './ToastNotification.vue'
import LoadingOverlay from './LoadingOverlay.vue'
import IconBadge from './IconBadge.vue'
import StatusIndicator from './StatusIndicator.vue'

const isSidebarOpen = ref(true)
const activeModal = ref<string | null>(null)
const searchQuery = ref('')

const filteredItems = computed(() => {
  return [] as Record<string, unknown>[]
})

function toggleSidebar() {
  isSidebarOpen.value = !isSidebarOpen.value
}

function openModal(name: string) {
  activeModal.value = name
}

function closeModal() {
  activeModal.value = null
}

function handleSearch(query: string) {
  searchQuery.value = query
}
</script>

<template>
  <div class="app-layout" :class="{ 'sidebar-open': isSidebarOpen }">
    <AppHeader @toggle-sidebar="toggleSidebar" />
    <AppSidebar :open="isSidebarOpen">
      <SearchBar @search="handleSearch" />
      <FilterDropdown />
    </AppSidebar>
    <main class="app-main">
      <LoadingOverlay />
      <DataGrid :items="filteredItems" />
      <ChartPanel />
    </main>
    <AppFooter />
    <ModalDialog v-if="activeModal" @close="closeModal" />
    <ToastNotification />
  </div>
</template>
