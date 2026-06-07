<script setup lang="ts">
import { messageManagment } from '~/composabels/messageManagment'
import type { genericRef } from '~~/types/other'

const activeChat = ref(inject("ChatOpen"))



const typeAreaChat: genericRef<"User" | "Group" | "Voice" | ""> = ref("")
const activeSearch = ref(false)
const searchData = ref("")
watch(activeChat, (newVal, oldVal) => {
    console.log(activeChat.value)
    const chat = (activeChat.value as {0:String, 1: object})
    switch(chat[0]) {
        case "chat":
        case "group":
        case "private":
            typeAreaChat.value = "Group"
            break
        case "voice":
        case "video_call":
            typeAreaChat.value = "Voice"
            break
        case "user":
        case "personal":
            typeAreaChat.value = "User"
            break
    }


})
</script>


<template>
    <section>
        <article v-if="typeAreaChat == ''" class="bg-body-500 h-full border-l-6 rounded-tl-2xl border-body-100">
            <article class="flex flex-row justify-between w-full ">
                <section
                    class="flex w-full justify-center col-start-1 bg-body-500 col-span-3 gap-5 jsutify-center items-center border-r-5 border-body-500 rounded-tr-2xl">
                    <article class="w-fit flex flex-row gap-8 items-center px-5 py-3 bg-body-900">
                        <section :class='{ "flex gap-3 items-center bg-body-100 px-2 rounded-md": activeSearch }'>
                            <input placeholder="Search"
                                class="w-80 h-10 m-[2px] rounded-full p-2 placeholder:text-white/50 placeholder:text-[18px] text-white text-[18px] bg-body-500"
                                v-model="searchData" v-if="activeSearch">
                            <svg @click.stop="activeSearch = !activeSearch" width="30" height="30" viewBox="0 0 40 40"
                                fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path fill-rule="evenodd" clip-rule="evenodd"
                                    d="M26.0333 28.3833C23.0385 30.7775 19.2405 31.9339 15.4196 31.615C11.5987 31.2962 8.04493 29.5263 5.48832 26.6689C2.93172 23.8114 1.56638 20.0835 1.67277 16.2508C1.77917 12.4181 3.34921 8.77158 6.0604 6.0604C8.77158 3.34921 12.4181 1.77917 16.2508 1.67277C20.0835 1.56638 23.8114 2.93172 26.6689 5.48832C29.5263 8.04493 31.2962 11.5987 31.615 15.4196C31.9339 19.2405 30.7775 23.0385 28.3833 26.0333L36.1833 33.8167C36.3387 33.972 36.462 34.1565 36.5461 34.3596C36.6302 34.5626 36.6735 34.7802 36.6735 35C36.6735 35.2197 36.6302 35.4374 36.5461 35.6404C36.462 35.8434 36.3387 36.0279 36.1833 36.1833C36.0279 36.3387 35.8434 36.462 35.6404 36.5461C35.4374 36.6302 35.2197 36.6735 35 36.6735C34.7802 36.6735 34.5626 36.6302 34.3596 36.5461C34.1565 36.462 33.972 36.3387 33.8167 36.1833L26.0333 28.3833ZM28.3333 16.6666C28.3333 18.1987 28.0316 19.7158 27.4452 21.1313C26.8589 22.5468 25.9996 23.8329 24.9162 24.9162C23.8329 25.9996 22.5468 26.8589 21.1313 27.4452C19.7158 28.0316 18.1987 28.3333 16.6666 28.3333C15.1346 28.3333 13.6175 28.0316 12.202 27.4452C10.7865 26.8589 9.50042 25.9996 8.41707 24.9162C7.33372 23.8329 6.47436 22.5468 5.88805 21.1313C5.30175 19.7158 4.99998 18.1987 4.99998 16.6666C4.99998 13.5725 6.22915 10.605 8.41707 8.41707C10.605 6.22915 13.5725 4.99998 16.6666 4.99998C19.7608 4.99998 22.7283 6.22915 24.9162 8.41707C27.1042 10.605 28.3333 13.5725 28.3333 16.6666Z"
                                    fill="white" />
                            </svg>
                        </section>
                        <svg width="30" height="30" viewBox="0 0 43 43" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M33.6651 1.5V41.5M41.5002 29.4365L33.6651 41.5L25.8301 29.4365" stroke="white"
                                stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
                            <path d="M9.33536 41.5L9.33536 1.5M1.50031 13.5635L9.33536 1.5L17.1704 13.5635"
                                stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
                        </svg>
                    </article>
                </section>
                <section class="w-30 bg-body-500">
                    <article class="flex bg-body-900 h-full flex-row justify-end gap-5 items-center px-5 py-2">
                        <svg width="30" height="30" viewBox="0 0 43 43" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M14.463 1.5H31.5C37.0228 1.5 41.5 5.97715 41.5 11.5V31.5C41.5 37.0228 37.0229 41.5 31.5 41.5H14.463M14.463 1.5H11.5C5.97716 1.5 1.5 5.97715 1.5 11.5V31.5C1.5 37.0228 5.97715 41.5 11.5 41.5H14.463M14.463 1.5V41.5"
                                stroke="white" stroke-width="3" />
                        </svg>
                        <svg width="30" height="30" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd"
                                d="M16.6665 6.66659C16.6665 7.55064 17.0177 8.39849 17.6428 9.02361C18.2679 9.64873 19.1158 9.99992 19.9998 9.99992C20.8839 9.99992 21.7317 9.64873 22.3569 9.02361C22.982 8.39849 23.3332 7.55064 23.3332 6.66659C23.3332 5.78253 22.982 4.93468 22.3569 4.30956C21.7317 3.68444 20.8839 3.33325 19.9998 3.33325C19.1158 3.33325 18.2679 3.68444 17.6428 4.30956C17.0177 4.93468 16.6665 5.78253 16.6665 6.66659ZM19.9998 23.3333C19.1158 23.3333 18.2679 22.9821 17.6428 22.3569C17.0177 21.7318 16.6665 20.884 16.6665 19.9999C16.6665 19.1159 17.0177 18.268 17.6428 17.6429C18.2679 17.0178 19.1158 16.6666 19.9998 16.6666C20.8839 16.6666 21.7317 17.0178 22.3569 17.6429C22.982 18.268 23.3332 19.1159 23.3332 19.9999C23.3332 20.884 22.982 21.7318 22.3569 22.3569C21.7317 22.9821 20.8839 23.3333 19.9998 23.3333ZM19.9998 36.6666C19.1158 36.6666 18.2679 36.3154 17.6428 35.6903C17.0177 35.0652 16.6665 34.2173 16.6665 33.3333C16.6665 32.4492 17.0177 31.6014 17.6428 30.9762C18.2679 30.3511 19.1158 29.9999 19.9998 29.9999C20.8839 29.9999 21.7317 30.3511 22.3569 30.9762C22.982 31.6014 23.3332 32.4492 23.3332 33.3333C23.3332 34.2173 22.982 35.0652 22.3569 35.6903C21.7317 36.3154 20.8839 36.6666 19.9998 36.6666Z"
                                fill="white" />
                        </svg>
                    </article>
                </section>
            </article>
        </article>
        <MainAreaChatGroup :Room="activeChat" v-if="typeAreaChat == 'Group'" />
        <MainAreaChatUser :Room="activeChat" v-if="typeAreaChat == 'User'" />
        <MainAreaChatVoice :Room="activeChat" v-if="typeAreaChat == 'Voice'" />
    </section>
</template>

<style lang="scss" scoped></style>