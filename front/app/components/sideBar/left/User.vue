<script setup lang="ts">
import { User } from '~/composabels/User'
import { GroupManagment } from '~/composabels/GroupManagment'

const allUser = ref<any[]>([])
const rooms = ref<any[]>([])
const userClass = new User
const groupClass = new GroupManagment()
const activeGroup = ref<any>(null)
const activeCreateDialog = ref(false)
const joinLink = ref('')

const emit = defineEmits(['openChat'])

const CHANNEL_SEP = '::'
const isChannelRoom = (r: any) => typeof r?.name === 'string' && r.name.includes(CHANNEL_SEP)

async function loadAll() {
    allUser.value = await userClass.getAllUser()
    rooms.value = await groupClass.requestGroup()

    if (activeGroup.value) {
        const g = activeGroup.value
        activeGroup.value = buildChannels(String(g.id), g.username, g.members ?? [], g.avatar ?? '')
    }
}

onMounted(() => {
    loadAll()

})


function buildChannels(id: string, name: string, members: string[] = [], avatar = '') {
    const channels = rooms.value
        .filter((r: any) => isChannelRoom(r) && r.name.startsWith(id + CHANNEL_SEP))
        .map((r: any) => ({
            id: r.id,
            name: r.name.split(CHANNEL_SEP).slice(1).join(CHANNEL_SEP),
            type: r.type === 'video_call' ? 'voice' : 'chat',
            newMessage: false,
        }))
    return { id, name, username: name, members, avatar, chat: { [name]: channels } }
}


function isGroup(el: any): boolean {
    return /_\d+$/.test(el?.username ?? '')
}


async function handleClick(el: any) {
    if (isGroup(el)) {
        activeGroup.value = buildChannels(String(el.id), el.username)
        return
    }
    const existing = rooms.value.find((r: any) =>
        r.type === 'personal' && Array.isArray(r.members) && r.members.includes(String(el.id))
    )
    if (existing) {
        emit('openChat', ['personal', existing])
    } else {

    }
}

async function joinByLink() {
    const token = joinLink.value.trim()
    if (!token) return
    const room = await groupClass.addNewUser(token, '')
    joinLink.value = ''
    await loadAll()
    if (room) console.log("User add", room)
}


</script>

<template>
    <article class="flex flex-col pt-10 px-2 overflow-y-auto scrollbar-hide scroll-smooth">


        <section class="flex flex-row gap-2 mb-6">
            <input v-model="joinLink" placeholder="krampus://join/… или id комнаты"
                class="flex-1 border border-white/30 bg-inherit text-white text-[13px] rounded px-2 py-1 placeholder:text-white/30 focus:outline-none">
            <button type="button" @click.stop="joinByLink"
                class="border border-white/30 text-white/60 hover:text-white hover:bg-white/10 rounded px-3 py-1 text-[14px]">
                Войти
            </button>
        </section>

        <section class="flex flex-col gap-6">
            <section @click.stop="handleClick(el)"
                class="flex hover:bg-white/10 flex-row justify-between jutify-center items-center w-full cursor-pointer"
                v-for="el in allUser" :key="el.id">
                <section class="flex flex-row gap-10 items-center">
                    <img :src="el.avatar || el.logo || ''" class="w-10 h-10 rounded-full object-cover bg-white/10">
                    <p class="text-[18px] text-white font-bold">
                        {{ el.username }}</p>
                </section>
            </section>
        </section>


        <article
            class="sticky bottom-4 self-end mt-6 bg-body-900 p-2 rounded-full cursor-pointer hover:bg-white/10 shadow-lg"
            @click.stop="activeCreateDialog = true">
            <svg width="35" height="35" viewBox="0 0 35 35" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path
                    d="M27.7082 20.4166C28.0949 20.4166 28.4659 20.5703 28.7394 20.8438C29.0129 21.1173 29.1665 21.4882 29.1665 21.875V26.25H33.5415C33.9283 26.25 34.2992 26.4036 34.5727 26.6771C34.8462 26.9506 34.9998 27.3215 34.9998 27.7083C34.9998 28.0951 34.8462 28.466 34.5727 28.7395C34.2992 29.013 33.9283 29.1666 33.5415 29.1666H29.1665V33.5416C29.1665 33.9284 29.0129 34.2993 28.7394 34.5728C28.4659 34.8463 28.0949 35 27.7082 35C27.3214 35 26.9505 34.8463 26.677 34.5728C26.4035 34.2993 26.2498 33.9284 26.2498 33.5416V29.1666H21.8748C21.4881 29.1666 21.1171 29.013 20.8436 28.7395C20.5701 28.466 20.4165 28.0951 20.4165 27.7083C20.4165 27.3215 20.5701 26.9506 20.8436 26.6771C21.1171 26.4036 21.4881 26.25 21.8748 26.25H26.2498V21.875C26.2498 21.4882 26.4035 21.1173 26.677 20.8438C26.9505 20.5703 27.3214 20.4166 27.7082 20.4166Z"
                    fill="white" />
                <path
                    d="M30.2751 18.3312C30.8584 18.7687 32.0689 18.5208 32.0834 17.7916V17.5C32.0842 15.2367 31.5582 13.0043 30.547 10.9794C29.5358 8.9546 28.0672 7.19291 26.2575 5.8338C24.4477 4.47469 22.3464 3.55546 20.12 3.14887C17.8935 2.74228 15.6029 2.85949 13.4296 3.49122C11.2563 4.12295 9.25981 5.25185 7.59822 6.7886C5.93663 8.32534 4.65552 10.2277 3.8563 12.3452C3.05707 14.4627 2.76167 16.7371 2.99346 18.9885C3.22525 21.2399 3.97788 23.4064 5.19178 25.3166C5.36678 25.5937 5.33761 25.9583 5.13344 26.2062L2.11469 29.6625C1.92972 29.873 1.80934 30.1324 1.76795 30.4096C1.72656 30.6868 1.76591 30.97 1.8813 31.2254C1.99669 31.4808 2.18323 31.6975 2.4186 31.8497C2.65396 32.0018 2.92818 32.0829 3.20844 32.0833H17.7918C18.5209 32.0687 18.7689 30.8583 18.3314 30.275C17.8583 29.6219 17.5749 28.8508 17.5125 28.0468C17.4501 27.2429 17.6111 26.4373 17.9778 25.7191C18.3444 25.0008 18.9024 24.3979 19.5902 23.9769C20.278 23.5559 21.0687 23.3332 21.8751 23.3333H22.6043C22.7977 23.3333 22.9831 23.2565 23.1199 23.1197C23.2566 22.983 23.3334 22.7975 23.3334 22.6041V21.875C23.3333 21.0686 23.556 20.2778 23.9771 19.59C24.3981 18.9023 25.001 18.3443 25.7192 17.9776C26.4374 17.6109 27.243 17.4499 28.047 17.5123C28.851 17.5747 29.6221 17.8581 30.2751 18.3312Z"
                    fill="white" />
            </svg>
        </article>
    </article>
</template>

<script lang="scss" scoped></script>
