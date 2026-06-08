<script setup lang="ts">
import { callStore } from '#imports';
import type { genericRef } from '~~/types/other';
const videoRef = useTemplateRef('videoRef')
const userId = defineProps(['userId'])
const call = callStore()
let currentStream: MediaStream | null = null

let muthAtribute = ref()


watch(
    () => call.peerConnectionUsers.get(userId.userId).stream,
    (newStream) => {
        console.log(newStream)
        console.log(videoRef.value)
        if (videoRef.value) {
            console.log(11)

            if (currentStream) {

                videoRef.value.srcObject = null;
            }
            videoRef.value.srcObject = newStream ?? null;
            currentStream = newStream ?? null;

        }
    },
    { deep: false }
);

watch(() => call.peerConnectionUsers.get(userId.userId).muth, (muth) => {
    console.log(muth)
    if(muth) {
        muthAtribute.value = "muted"
    } else {
        muthAtribute.value = null
    }
})

onMounted(() => {
    if (videoRef.value && call.peerConnectionUsers.get(userId.userId)?.stream) {
        videoRef.value.srcObject = call.peerConnectionUsers.get(userId.userId)!.stream
    }
})

onUnmounted(() => {
    if (videoRef.value) {

        videoRef.value.srcObject = null;
    }
});
</script>

<template>
    <video v-bind:[muthAtribute]="''" class="bg-inherit w-80 h-50" ref="videoRef"  autoplay poster="/img/userCall.png" playsinline :id="`User${userId.userId}`" />
</template>

<style scoped></style>