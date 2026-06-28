<template>
  <div class="page-not-found">
    <h1>404</h1>
    <p>Page not found</p>
    <router-link to="/">Go home</router-link>
  </div>
</template>

<style scoped>
.page-not-found {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  text-align: center;
}

.page-not-found h1 {
  font-size: 72px;
  margin-bottom: 16px;
}

.page-not-found p {
  font-size: 18px;
  color: #666;
  margin-bottom: 24px;
}
</style>
