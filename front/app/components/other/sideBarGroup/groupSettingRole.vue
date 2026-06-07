<script setup lang="ts">
    let roleData = defineProps(['roleData'])
    roleData = roleData.roleData
</script>

<template>
    <section>
        <p @click.stop="$emit('settingRole', roleData)" class="text-[18px] text-white font-bold" :style="{color: roleData.color}">#{{ roleData.name }}</p>
         <svg @click.stop="$emit('deleteRoleGroup', roleData.id)" viewBox="0 0 32 32" class="w-5 fill-red-500"
            xmlns="http://www.w3.org/2000/svg">
            <path d="M4,29a1,1,0,0,1-.71-.29,1,1,0,0,1,0-1.42l24-24a1,1,0,1,1,1.42,1.42l-24,24A1,1,0,0,1,4,29Z" />
            <path d="M28,29a1,1,0,0,1-.71-.29l-24-24A1,1,0,0,1,4.71,3.29l24,24a1,1,0,0,1,0,1.42A1,1,0,0,1,28,29Z" />
        </svg>
    </section>
</template>

<style scoped></style>