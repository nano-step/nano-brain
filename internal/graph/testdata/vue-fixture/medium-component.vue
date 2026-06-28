<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import UserAvatar from './UserAvatar.vue'
import SearchInput from './SearchInput.vue'
import StatusBadge from './StatusBadge.vue'

interface User {
  id: number
  name: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
}

enum SortOrder {
  Asc = 'asc',
  Desc = 'desc',
}

type FilterFn = (user: User) => boolean

const users = ref<User[]>([])
const searchQuery = ref('')
const sortField = ref<keyof User>('name')
const sortOrder = ref<SortOrder>(SortOrder.Asc)
const selectedUserId = ref<number | null>(null)

const filteredUsers = computed(() => {
  let result = users.value.filter(u =>
    u.name.toLowerCase().includes(searchQuery.value.toLowerCase())
  )
  result.sort((a, b) => {
    const val = String(a[sortField.value]).localeCompare(String(b[sortField.value]))
    return sortOrder.value === SortOrder.Asc ? val : -val
  })
  return result
})

const selectedUser = computed(() =>
  users.value.find(u => u.id === selectedUserId.value) ?? null
)

function addUser(name: string, email: string, role: User['role']) {
  const newUser: User = {
    id: Date.now(),
    name,
    email,
    role,
  }
  users.value.push(newUser)
  persistUsers()
}

function removeUser(id: number) {
  users.value = users.value.filter(u => u.id !== id)
  if (selectedUserId.value === id) {
    selectedUserId.value = null
  }
  persistUsers()
}

function selectUser(id: number) {
  selectedUserId.value = id
}

function toggleSort(field: keyof User) {
  if (sortField.value === field) {
    sortOrder.value = sortOrder.value === SortOrder.Asc ? SortOrder.Desc : SortOrder.Asc
  } else {
    sortField.value = field
    sortOrder.value = SortOrder.Asc
  }
}

async function fetchUsers() {
  try {
    const response = await fetch('/api/users')
    const data = await response.json()
    users.value = data
  } catch (err) {
    console.error('Failed to fetch users:', err)
  }
}

function persistUsers() {
  localStorage.setItem('users', JSON.stringify(users.value))
}

function loadLocalUsers() {
  const stored = localStorage.getItem('users')
  if (stored) {
    users.value = JSON.parse(stored)
  }
}

onMounted(() => {
  loadLocalUsers()
  if (users.value.length === 0) {
    fetchUsers()
  }
})

watch(selectedUserId, (newId) => {
  if (newId !== null) {
    console.log('Selected user:', newId)
  }
})
</script>

<template>
  <div class="user-management">
    <SearchInput v-model="searchQuery" placeholder="Search users..." />
    <div class="user-list">
      <div
        v-for="user in filteredUsers"
        :key="user.id"
        class="user-row"
        :class="{ selected: user.id === selectedUserId }"
        @click="selectUser(user.id)"
      >
        <UserAvatar :name="user.name" />
        <span>{{ user.name }}</span>
        <StatusBadge :status="user.role" />
      </div>
    </div>
  </div>
</template>
