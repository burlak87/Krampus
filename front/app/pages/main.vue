<script setup lang="ts">
import LeftSidersBar from '~/components/leftSidersBar.vue';
import type { genericRef } from '~~/types/other';

definePageMeta({
    middleware: 'auth',
})

const OpenChat: genericRef<object> = ref([])

console.log(OpenChat.value)

provide("ChatOpen", OpenChat)
</script>


<template>
    <section class="grid grid-cols-4 h-screen box-border">
        <LeftSidersBar @exitGroup="" @openChat="(chatData: object) => {OpenChat = chatData}" class="col-span-1 h-screen box-border" />
        <MainAreaChat class="col-span-3 col-start-2" />
    </section>
</template>

<style lang="scss" scoped></style>