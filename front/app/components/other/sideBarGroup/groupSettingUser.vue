<script setup lang="ts">
    let userData = defineProps(["userGroup"])
    userData = userData.userGroup
</script>

<template>
    <section class="flex flex-row justify-between items-center w-full py-2 px-1">
        <article class="flex flex-row gap-5 items-center">
            <img class="w-10 h-10 rounded-full" :src="userData.logo">
            <p class="text-[18px] text-white">{{ userData.userName }}</p>
        </article>
        <svg @click.stop="$emit('deleteUserGroup', userData.id)" viewBox="0 0 32 32" class="w-5 fill-red-500"
            xmlns="http://www.w3.org/2000/svg">
            <path d="M4,29a1,1,0,0,1-.71-.29,1,1,0,0,1,0-1.42l24-24a1,1,0,1,1,1.42,1.42l-24,24A1,1,0,0,1,4,29Z" />
            <path d="M28,29a1,1,0,0,1-.71-.29l-24-24A1,1,0,0,1,4.71,3.29l24,24a1,1,0,0,1,0,1.42A1,1,0,0,1,28,29Z" />
        </svg>
    </section>
</template>

<style scoped></style>