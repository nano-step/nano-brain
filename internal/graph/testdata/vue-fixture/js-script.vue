<script>
var Vue = require('vue')
var axios = require('axios')
var _ = require('lodash')

module.exports = {
  name: 'DataView',
  data: function () {
    return {
      items: [],
      loading: false,
      page: 1,
      pageSize: 20,
    }
  },
  computed: {
    totalPages: function () {
      return Math.ceil(this.items.length / this.pageSize)
    },
    paginatedItems: function () {
      var start = (this.page - 1) * this.pageSize
      return _.chunk(this.items, this.pageSize)[this.page - 1] || []
    },
  },
  methods: {
    fetchItems: function () {
      this.loading = true
      var self = this
      axios.get('/api/items', { params: { page: this.page } })
        .then(function (response) {
          self.items = response.data
        })
        .catch(function (err) {
          console.error('Fetch failed:', err)
        })
        .finally(function () {
          self.loading = false
        })
    },
    nextPage: function () {
      if (this.page < this.totalPages) {
        this.page++
        this.fetchItems()
      }
    },
    prevPage: function () {
      if (this.page > 1) {
        this.page--
        this.fetchItems()
      }
    },
  },
  mounted: function () {
    this.fetchItems()
  },
}
</script>

<template>
  <div class="data-view">
    <div v-if="loading" class="spinner">Loading...</div>
    <ul v-else>
      <li v-for="item in paginatedItems" :key="item.id">{{ item.name }}</li>
    </ul>
    <div class="pagination">
      <button @click="prevPage" :disabled="page <= 1">Previous</button>
      <span>Page {{ page }} of {{ totalPages }}</span>
      <button @click="nextPage" :disabled="page >= totalPages">Next</button>
    </div>
  </div>
</template>
