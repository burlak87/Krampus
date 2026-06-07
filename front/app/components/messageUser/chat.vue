<script setup lang="ts">
const props = defineProps(["messageData", "highlight"])
const userMessage = props.messageData
const isHover = ref(false)

const userStore = useUserStore()

// Own messages align right. Prefer a robust user_id comparison; fall back to
// username when the message carries no id.
const isActiveUser = computed(() => {
    const me = userStore.userData
    if (userMessage?.userId != null && me?.id != null) {
        return String(userMessage.userId) === String(me.id)
    }
    return userMessage?.name === me?.userName
})
</script>

<template>
    <article @mouseenter="isActiveUser ? isHover = true : isHover = false" @mouseleave="isHover = false"
        :class='{ "w-full relative flex justify-end": isActiveUser, "w-full relative h-fit": !isActiveUser }'>
        <section
            :class='{ "w-fit flex p-4 rounded-xl bg-body-900 flex-row-reverse gap-5 relative box-border": isActiveUser, "w-fit p-4 rounded-xl relative  bg-body-900 flex flex-row gap-5  box-border ": !isActiveUser, "ring-2 ring-yellow-400": highlight }'>
            <p class=" text-[20px] max-w-100 text-white break-all">{{ userMessage.data }}</p>
            <article class="absolute w-full top-0 right-0 h-full">
                <p
                    :class="{ 'text-[16px] text-white/60 bg-body-100 py-0.5 px-5 rounded-full absolute bottom-[-20px]': true, 'left-0': !isActiveUser, 'right-0': isActiveUser }">
                    {{ userMessage.time }}</p>
            </article>
        </section>
        <section v-if="isHover" class="flex flex-row gap-1 items-center absolute top-[-10px]">
            <svg @click.stop="$emit('deleteMessage', userMessage.id)" class="fill-red-500" enable-background="new 0 0 91 91" height="30px" id="Layer_1" version="1.1"
                viewBox="0 0 91 91" width="30px" xml:space="preserve" xmlns="http://www.w3.org/2000/svg"
                xmlns:xlink="http://www.w3.org/1999/xlink">
                <g>
                    <path
                        d="M67.305,36.442v-8.055c0-0.939-0.762-1.701-1.7-1.701H54.342v-5.524c0-0.938-0.761-1.7-1.699-1.7h-12.75   c-0.939,0-1.701,0.762-1.701,1.7v5.524H26.93c-0.939,0-1.7,0.762-1.7,1.701v8.055c0,0.938,0.761,1.699,1.7,1.699h0.488v34.021   c0,0.938,0.761,1.7,1.699,1.7h29.481c3.595,0,6.52-2.924,6.52-6.518V38.142h0.486C66.543,38.142,67.305,37.381,67.305,36.442z    M41.592,22.862h9.35v3.824h-9.35V22.862z M61.719,67.345c0,1.719-1.4,3.117-3.12,3.117h-27.78v-32.32l30.9,0.002V67.345z    M63.904,34.742H28.629v-4.655h11.264h12.75h11.262V34.742z" />
                    <rect height="19.975" width="3.4" x="36.066" y="44.962" />
                    <rect height="19.975" width="3.4" x="44.566" y="44.962" />
                    <rect height="19.975" width="3.4" x="53.066" y="44.962" />
                </g>
            </svg>
            <svg @click.stop="$emit('editMessage', userMessage)" class="fill-white" width="25px" height="25px" data-name="Layer 1" id="Layer_1" viewBox="0 0 32 32"
                xmlns="http://www.w3.org/2000/svg">
                <title />
                <path
                    d="M25.384,11.987a.993.993,0,0,1-.707-.293L20.434,7.452a1,1,0,0,1,0-1.414l2.122-2.121a3.07,3.07,0,0,1,4.242,0l1.414,1.414a3,3,0,0,1,0,4.242l-2.122,2.121A.993.993,0,0,1,25.384,11.987ZM22.555,6.745l2.829,2.828L26.8,8.159a1,1,0,0,0,0-1.414L25.384,5.331a1.023,1.023,0,0,0-1.414,0Z" />
                <path
                    d="M11.9,22.221a2,2,0,0,1-1.933-2.487l.875-3.5a3.02,3.02,0,0,1,.788-1.393l8.8-8.8a1,1,0,0,1,1.414,0l4.243,4.242a1,1,0,0,1,0,1.414l-8.8,8.8a3,3,0,0,1-1.393.79h0l-3.5.875A2.027,2.027,0,0,1,11.9,22.221Zm3.752-1.907h0ZM21.141,8.159l-8.094,8.093a1,1,0,0,0-.262.465l-.876,3.5,3.5-.876a1,1,0,0,0,.464-.263l8.094-8.094Z" />
                <path
                    d="M22,29H8a5.006,5.006,0,0,1-5-5V10A5.006,5.006,0,0,1,8,5h9.64a1,1,0,0,1,0,2H8a3,3,0,0,0-3,3V24a3,3,0,0,0,3,3H22a3,3,0,0,0,3-3V14.61a1,1,0,0,1,2,0V24A5.006,5.006,0,0,1,22,29Z" />
            </svg>
        </section>
    </article>
</template>

<style scoped></style>