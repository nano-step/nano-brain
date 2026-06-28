<script setup lang="ts">
import { ref } from 'vue'
import IconButton from './IconButton.vue'

const count = ref(0)

function increment() {
  count.value++
  console.log('count:', count.value)
}

function reset() {
  count.value = 0
}
</script>

<template>
  <div class="counter">
    <IconButton @click="increment">+</IconButton>
    <span>{{ count }}</span>
    <button @click="reset">Reset</button>
  </div>
</template>

<style scoped>
.counter { display: flex; align-items: center; gap: 8px; }
</style>
