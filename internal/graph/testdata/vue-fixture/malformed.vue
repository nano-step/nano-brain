<script setup lang="ts">
import { ref } from 'vue'
import BrokenComponent from './BrokenComponent.vue'

const count = ref(0)

function broken( {
  return count.value
}

function alsoBroken() {
  const items = [1, 2, 3]
  for (const item of items) {
    console.log(item)
  // missing closing brace
</script>

<template>
  <div>
    <BrokenComponent />
    <p>{{ count }}</p>
  </div>
</template>
