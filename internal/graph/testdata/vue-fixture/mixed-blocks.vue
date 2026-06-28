<script>
export default {
  name: 'UserProfile',
  inheritAttrs: false,
  methods: {
    legacyMethod() {
      console.log('legacy method called')
    },
  },
}
</script>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import ProfileCard from './ProfileCard.vue'
import ActivityLog from './ActivityLog.vue'
import SettingsForm from './SettingsForm.vue'

const userName = ref('')
const isLoading = ref(true)
const activeTab = ref('profile')

async function loadProfile() {
  try {
    const response = await fetch('/api/profile')
    const data = await response.json()
    userName.value = data.name
  } catch (err) {
    console.error('Failed to load profile:', err)
  } finally {
    isLoading.value = false
  }
}

function switchTab(tab: string) {
  activeTab.value = tab
}

onMounted(() => {
  loadProfile()
})
</script>

<template>
  <div class="user-profile">
    <div v-if="isLoading">Loading...</div>
    <div v-else>
      <h2>{{ userName }}</h2>
      <nav class="tabs">
        <button @click="switchTab('profile')">Profile</button>
        <button @click="switchTab('activity')">Activity</button>
        <button @click="switchTab('settings')">Settings</button>
      </nav>
      <ProfileCard v-if="activeTab === 'profile'" />
      <ActivityLog v-if="activeTab === 'activity'" />
      <SettingsForm v-if="activeTab === 'settings'" />
    </div>
  </div>
</template>
