<!-- src/ui-plugins/SlaWidget.vue -->
<script setup>
defineProps({
  issueData: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <!-- Only render if the WASM backend plugin actually injected the SLA -->
  <div v-if="issueData?.custom_data?.sla_deadline" class="sla-plugin-box">
    🚨 <strong>URGENT SLA DEADLINE:</strong> 
    {{ issueData.custom_data.sla_deadline }}
    <p class="notice">{{ issueData.custom_data.plugin_notice }}</p>
  </div>
</template>

<style scoped>
.sla-plugin-box {
  background-color: #ffeaea;
  color: #cc0000;
  border: 2px solid #ff0000;
  padding: 15px;
  border-radius: 8px;
  margin-top: 20px;
  font-size: 1.1em;
  text-align: center;
  animation: pulse 2s infinite;
}
.notice { font-size: 0.8em; color: #666; margin: 5px 0 0 0; }
@keyframes pulse {
  0% { box-shadow: 0 0 0 0 rgba(255, 0, 0, 0.4); }
  70% { box-shadow: 0 0 0 10px rgba(255, 0, 0, 0); }
  100% { box-shadow: 0 0 0 0 rgba(255, 0, 0, 0); }
}
</style>
