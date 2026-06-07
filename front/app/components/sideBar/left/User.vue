<script setup lang="ts">
import { User } from '~/composabels/User'
import { GroupManagment } from '~/composabels/GroupManagment'
import { apiClient } from '~/composabels/apiClient'

const allUser = ref<any[]>([])
const rooms = ref<any[]>([])
const userClass = new User
const groupClass = new GroupManagment()
const activeGroup = ref<any>(null)
const activeCreateDialog = ref(false)
const joinLink = ref('')
const groupAvatarInput = ref<HTMLInputElement | null>(null)
const folderOpen = ref(true)
const showCreate = ref(false)

const emit = defineEmits(['openChat'])

function pickGroupAvatar() {
    groupAvatarInput.value?.click()
}

async function onGroupAvatarPick(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0]
    const g = activeGroup.value
    if (!file || !g?.id) return
    const dataUrl: string = await new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result))
        reader.onerror = reject
        reader.readAsDataURL(file)
    })
    try {
        await apiClient.setRoomAvatar(String(g.id), dataUrl)
        g.avatar = dataUrl
        await loadAll()
    } catch (err) {
        console.error("[group avatar] upload failed", err)
    }
}

// Channel rooms encode their parent group id in the name: "<groupId>::<channelName>".
const CHANNEL_SEP = '::'
const isChannelRoom = (r: any) => typeof r?.name === 'string' && r.name.includes(CHANNEL_SEP)

async function loadAll() {
    allUser.value = await userClass.getAllUser()
    rooms.value = await groupClass.requestGroup()
    // If a group panel is open, rebuild it so newly-synced channels show up.
    if (activeGroup.value) {
        const g = activeGroup.value
        activeGroup.value = buildChannels(String(g.id), g.username, g.members ?? [], g.avatar ?? '')
    }
}

// Constant polling, but group-scoped: it only refreshes the rooms list (groups
// and their channels), never the user list or an open personal conversation.
const POLL_MS = 5000
let pollTimer: ReturnType<typeof setInterval> | undefined

async function pollGroups() {
    rooms.value = await groupClass.requestGroup()
    if (activeGroup.value) {
        const g = activeGroup.value
        activeGroup.value = buildChannels(String(g.id), g.username, g.members ?? [], g.avatar ?? '')
    }
}

onMounted(() => {
    loadAll()
    pollTimer = setInterval(pollGroups, POLL_MS)
})

onUnmounted(() => {
    if (pollTimer) clearInterval(pollTimer)
})

// Top-level entries: real groups/personals, excluding channel rooms.
const displayRooms = computed(() => rooms.value.filter((r: any) => !isChannelRoom(r)))

async function createGroup(name: string) {
    console.log("[User.vue] createGroup called with", name)
    await groupClass.createGroup(name)
    activeCreateDialog.value = false
    await loadAll()
}

// Build the channel sidebar for a group, reading persisted channel rooms.
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

// A "group" username has a trailing _<digits> suffix.
function isGroup(el: any): boolean {
    return /_\d+$/.test(el?.username ?? '')
}

// Click on a user → OPEN the existing personal chat with them (no creation).
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
        console.log("[User.vue] no personal chat with", el.username, "- create one from the + dialog")
    }
}

// Created group rooms → open the channel sidebar.
function openRoom(room: any) {
    if (room.type === 'personal') {
        emit('openChat', ['personal', room])
        return
    }
    activeGroup.value = buildChannels(String(room.id), room.name, room.members ?? [], room.avatar ?? '')
}

function openChannel(chat: any) {
    emit('openChat', [chat.type, chat])
}

// Personal chat creation moved to the modal (a named personal room).
async function createPersonal(name: string) {
    console.log("[User.vue] createPersonal called with", name)
    await groupClass.createPersonal('', name)
    activeCreateDialog.value = false
    await loadAll()
}

// Invite link for any room (group OR personal) — joinable via /rooms/join.
function inviteFor(id: string): string {
    return `krampus://join/${id}`
}

async function copyInviteId(id: string) {
    const link = inviteFor(id)
    try {
        await navigator.clipboard.writeText(link)
        console.log("[User.vue] invite copied", link)
    } catch (e) {
        console.error("[User.vue] copy failed", e)
    }
}

// Invite link for the currently open group.
const inviteLink = computed(() => activeGroup.value ? inviteFor(activeGroup.value.id) : '')
const copyInvite = () => activeGroup.value && copyInviteId(activeGroup.value.id)

async function joinByLink() {
    const token = joinLink.value.trim()
    if (!token) return
    const room = await groupClass.addNewUser(token, '')
    joinLink.value = ''
    await loadAll()
    if (room) console.log("[User.vue] joined", room)
}

// Create a named channel inside the currently open group (persisted as a room).
const newChannelName = ref('')
async function createChannel(type: 'chat' | 'voice') {
    const g = activeGroup.value
    if (!g) return
    const name = (newChannelName.value || '').trim()
    if (!name) return
    const encoded = `${g.id}${CHANNEL_SEP}${name}`
    // include the group's current members so everyone already in the group
    // gets the new channel too (late joiners are added on /rooms/join).
    await groupClass.createChat(encoded, type === 'voice' ? 'video_call' : 'private', '', g.id, g.members ?? [])
    newChannelName.value = ''
    await loadAll()
    // rebuild the open group's channel list from the refreshed rooms
    activeGroup.value = buildChannels(String(g.id), g.username)
}
</script>

<template>
    <!-- Channel sidebar for a selected group (Discord-like) -->
    <article v-if="activeGroup" class="flex flex-col relative min-h-screen">
        <!-- Group header -->
        <article class="flex flex-row items-center gap-3 px-3 py-4 border-b border-body-100">
            <svg @click.stop="activeGroup = null" class="cursor-pointer flex-shrink-0"
                width="20" height="20" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                <path d="M15 6l-6 6 6 6" stroke="white" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <img @click.stop="pickGroupAvatar" :src="activeGroup.avatar || activeGroup.logo || ''"
                class="w-11 h-11 rounded-full object-cover bg-white/10 cursor-pointer flex-shrink-0" title="Сменить фото группы">
            <input ref="groupAvatarInput" type="file" accept="image/*" class="hidden" @change="onGroupAvatarPick">
            <section class="flex flex-col flex-1 min-w-0">
                <h2 class="text-[22px] text-white/90 font-bold truncate">{{ activeGroup.username }}</h2>
                <p class="text-[14px] text-white/50">{{ (activeGroup.members?.length ?? 0) }} участников</p>
            </section>
        </article>

        <section class="flex flex-col gap-1 px-2 pt-4 overflow-y-auto scrollbar-hide scroll-smooth pb-24">
            <!-- collapsible folder -->
            <article class="flex flex-row items-center justify-between px-2 py-2 cursor-pointer hover:bg-white/5 rounded"
                @click.stop="folderOpen = !folderOpen">
                <section class="flex flex-row items-center gap-2">
                    <svg :class="{ 'transition-transform': true, '-rotate-90': !folderOpen }" width="18" height="12" viewBox="0 0 25 15" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M0.526313 0.659511C0.858915 0.318085 1.30608 0.126843 1.77182 0.126843C2.23756 0.126843 2.68473 0.318085 3.01733 0.659511L12.4476 10.5169L21.8779 0.640876C22.0387 0.447095 22.2361 0.29022 22.4576 0.180093C22.679 0.0699668 22.9199 0.00896447 23.1649 0.00091577C23.41 -0.00713293 23.654 0.037945 23.8816 0.133321C24.1093 0.228696 24.3156 0.372312 24.4877 0.555157C24.6599 0.738001 24.7941 0.956128 24.8819 1.19585C24.9697 1.43558 25.0093 1.69173 24.9982 1.94823C24.987 2.20473 24.9253 2.45604 24.817 2.68641C24.7088 2.91678 24.5562 3.12122 24.3689 3.28691L13.6931 14.4673C13.3605 14.8088 12.9133 15 12.4476 15C11.9819 15 11.5347 14.8088 11.2021 14.4673L0.526313 3.28691C0.359542 3.11368 0.227172 2.90759 0.13684 2.68052C0.0465073 2.45344 0 2.20988 0 1.96389C0 1.7179 0.0465073 1.47434 0.13684 1.24727C0.227172 1.0202 0.359542 0.814104 0.526313 0.640876V0.659511Z" fill="white"/>
                    </svg>
                    <p class="text-[18px] text-white font-bold">{{ activeGroup.username }}</p>
                </section>
            </article>

            <!-- channels -->
            <article v-show="folderOpen" class="flex flex-col gap-1 pl-3">
                <article v-for="chat in activeGroup.chat[activeGroup.username]" :key="chat.id"
                    @click.stop="openChannel(chat)"
                    class="flex flex-row gap-3 items-center px-2 py-2 rounded cursor-pointer hover:bg-white/10">
                    <svg v-if="chat.type === 'chat'" class="fill-white flex-shrink-0" width="20" height="18" viewBox="0 0 28 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M7.51444 2.13923C5.24165 3.56915 3.52251 5.66358 2.61795 8.10463C1.71338 10.5457 1.67276 13.2001 2.50226 15.6651C2.73218 16.385 2.70919 17.1485 2.27234 17.7594L0.364038 20.6391C0.141344 20.9684 0.0161075 21.3485 0.00145149 21.7396C-0.0132045 22.1307 0.0832618 22.5183 0.280739 22.8618C0.478216 23.2053 0.769432 23.4921 1.12388 23.6922C1.47832 23.8922 1.88294 23.9981 2.29534 23.9987H14.9407C18.3478 24.0454 21.6353 22.8088 24.0832 20.5597C26.531 18.3106 27.9395 15.2326 28 12C27.9395 8.7674 26.531 5.68935 24.0832 3.4403C21.6353 1.19124 18.3478 -0.0454209 14.9407 0.00127599C12.1817 0.00127599 9.60668 0.786646 7.51444 2.13923Z"/>
                    </svg>
                    <svg v-else class="fill-white flex-shrink-0" width="20" height="18" viewBox="0 0 30 26" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M15 3.25035C15 2.96303 14.8683 2.68748 14.6339 2.48432C14.3995 2.28115 14.0815 2.16702 13.75 2.16702H13.675C13.5015 2.166 13.3297 2.1963 13.1704 2.25598C13.0112 2.31566 12.868 2.40343 12.75 2.51368L7.4 7.58368H3.75C3.41848 7.58368 3.10054 7.69782 2.86612 7.90098C2.6317 8.10415 2.5 8.3797 2.5 8.66702V17.3337C2.5 17.621 2.6317 17.8966 2.86612 18.0997C3.10054 18.3029 3.41848 18.417 3.75 18.417H7.4L12.75 23.487C12.868 23.5973 13.0112 23.685 13.1704 23.7447C13.3297 23.8044 13.5015 23.8347 13.675 23.8337H13.75C14.0815 23.8337 14.3995 23.7195 14.6339 23.5164C14.8683 23.3132 15 23.0377 15 22.7503V3.25035ZM18.875 22.4795C18.15 22.6312 17.5 22.122 17.5 21.4828V21.4503C17.5 20.9087 17.9625 20.4537 18.5625 20.3128C20.4108 19.8729 22.0413 18.919 23.2034 17.5979C24.3655 16.2768 24.9949 14.6616 24.9949 13.0003C24.9949 11.3391 24.3655 9.72387 23.2034 8.40277C22.0413 7.08168 20.4108 6.12785 18.5625 5.68785C18.2657 5.62597 18.0007 5.48096 17.8086 5.27531C17.6165 5.06966 17.508 4.81485 17.5 4.55035V4.51785C17.5 3.86785 18.15 3.36952 18.875 3.52118C21.3306 4.03354 23.5158 5.24713 25.0788 6.9666C26.6419 8.68607 27.4918 10.8114 27.4918 13.0003C27.4918 15.1893 26.6419 17.3146 25.0788 19.0341C23.5158 20.7536 21.3306 21.9672 18.875 22.4795Z"/>
                    </svg>
                    <p class="text-white text-[16px]">{{ chat.name }}</p>
                </article>
            </article>

            <!-- create channel form (toggled by the + button) -->
            <section v-if="showCreate" class="flex flex-col gap-2 mt-3 pl-3">
                <input v-model="newChannelName" placeholder="Название канала"
                    class="border border-white/30 bg-inherit text-white text-[14px] rounded px-2 py-1 placeholder:text-white/30 focus:outline-none">
                <section class="flex flex-row gap-2">
                    <button type="button" @click.stop="createChannel('chat'); showCreate = false"
                        class="flex-1 border border-white/30 text-white/60 hover:text-white hover:bg-white/10 rounded px-2 py-1 text-[14px]">
                        + текстовый
                    </button>
                    <button type="button" @click.stop="createChannel('voice'); showCreate = false"
                        class="flex-1 border border-white/30 text-white/60 hover:text-white hover:bg-white/10 rounded px-2 py-1 text-[14px]">
                        + голосовой
                    </button>
                </section>
            </section>

            <!-- invite link -->
            <section class="flex flex-col gap-1 mt-6 px-1">
                <p class="text-white/50 text-[13px] font-bold uppercase tracking-wider">Пригласительная ссылка</p>
                <section class="flex flex-row gap-2 items-center">
                    <input :value="inviteLink" readonly
                        class="flex-1 border border-white/30 bg-inherit text-white/80 text-[13px] rounded px-2 py-1 focus:outline-none">
                    <button type="button" @click.stop="copyInvite"
                        class="border border-white/30 text-white/60 hover:text-white hover:bg-white/10 rounded px-3 py-1 text-[14px]">
                        Копировать
                    </button>
                </section>
            </section>
        </section>

        <!-- floating create button -->
        <article class="absolute bottom-4 right-4 bg-body-900 p-2 rounded-full cursor-pointer hover:bg-white/10 shadow-lg"
            @click.stop="showCreate = !showCreate">
            <svg width="32" height="32" viewBox="0 0 35 35" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M27.7082 20.4166C28.0949 20.4166 28.4659 20.5703 28.7394 20.8438C29.0129 21.1173 29.1665 21.4882 29.1665 21.875V26.25H33.5415C33.9283 26.25 34.2992 26.4036 34.5727 26.6771C34.8462 26.9506 34.9998 27.3215 34.9998 27.7083C34.9998 28.0951 34.8462 28.466 34.5727 28.7395C34.2992 29.013 33.9283 29.1666 33.5415 29.1666H29.1665V33.5416C29.1665 33.9284 29.0129 34.2993 28.7394 34.5728C28.4659 34.8463 28.0949 35 27.7082 35C27.3214 35 26.9505 34.8463 26.677 34.5728C26.4035 34.2993 26.2498 33.9284 26.2498 33.5416V29.1666H21.8748C21.4881 29.1666 21.1171 29.013 20.8436 28.7395C20.5701 28.466 20.4165 28.0951 20.4165 27.7083C20.4165 27.3215 20.5701 26.9506 20.8436 26.6771C21.1171 26.4036 21.4881 26.25 21.8748 26.25H26.2498V21.875C26.2498 21.4882 26.4035 21.1173 26.677 20.8438C26.9505 20.5703 27.3214 20.4166 27.7082 20.4166Z" fill="white" />
                <path d="M30.2751 18.3312C30.8584 18.7687 32.0689 18.5208 32.0834 17.7916V17.5C32.0842 15.2367 31.5582 13.0043 30.547 10.9794C29.5358 8.9546 28.0672 7.19291 26.2575 5.8338C24.4477 4.47469 22.3464 3.55546 20.12 3.14887C17.8935 2.74228 15.6029 2.85949 13.4296 3.49122C11.2563 4.12295 9.25981 5.25185 7.59822 6.7886C5.93663 8.32534 4.65552 10.2277 3.8563 12.3452C3.05707 14.4627 2.76167 16.7371 2.99346 18.9885C3.22525 21.2399 3.97788 23.4064 5.19178 25.3166C5.36678 25.5937 5.33761 25.9583 5.13344 26.2062L2.11469 29.6625C1.92972 29.873 1.80934 30.1324 1.76795 30.4096C1.72656 30.6868 1.76591 30.97 1.8813 31.2254C1.99669 31.4808 2.18323 31.6975 2.4186 31.8497C2.65396 32.0018 2.92818 32.0829 3.20844 32.0833H17.7918C18.5209 32.0687 18.7689 30.8583 18.3314 30.275C17.8583 29.6219 17.5749 28.8508 17.5125 28.0468C17.4501 27.2429 17.6111 26.4373 17.9778 25.7191C18.3444 25.0008 18.9024 24.3979 19.5902 23.9769C20.278 23.5559 21.0687 23.3332 21.8751 23.3333H22.6043C22.7977 23.3333 22.9831 23.2565 23.1199 23.1197C23.2566 22.983 23.3334 22.7975 23.3334 22.6041V21.875C23.3333 21.0686 23.556 20.2778 23.9771 19.59C24.3981 18.9023 25.001 18.3443 25.7192 17.9776C26.4374 17.6109 27.243 17.4499 28.047 17.5123C28.851 17.5747 29.6221 17.8581 30.2751 18.3312Z" fill="white" />
            </svg>
        </article>
    </article>

    <!-- User list (default) -->
    <article v-else class="flex flex-col pt-10 px-2 overflow-y-auto scrollbar-hide scroll-smooth">
        <ModalCreateGroup v-if="activeCreateDialog"
            @createGroup="(name: string) => createGroup(name)"
            @createPersonal="(name: string) => createPersonal(name)"
            @dropDialog="activeCreateDialog = false" />

        <!-- Join a room by invite link -->
        <section class="flex flex-row gap-2 mb-6">
            <input v-model="joinLink" placeholder="krampus://join/… или id комнаты"
                class="flex-1 border border-white/30 bg-inherit text-white text-[13px] rounded px-2 py-1 placeholder:text-white/30 focus:outline-none">
            <button type="button" @click.stop="joinByLink"
                class="border border-white/30 text-white/60 hover:text-white hover:bg-white/10 rounded px-3 py-1 text-[14px]">
                Войти
            </button>
        </section>

        <section class="flex flex-col gap-6">
            <!-- Created rooms: groups and personal chats -->
            <section
                class="flex hover:bg-white/10 flex-row justify-between items-center w-full group/room"
                v-for="room in displayRooms" :key="room.id">
                <section class="flex flex-row gap-10 items-center cursor-pointer flex-1" @click.stop="openRoom(room)">
                    <img :src="room.avatar || room.logo || ''" class="w-10 h-10 rounded-full object-cover bg-white/10">
                    <p class="text-[18px] text-white font-bold">
                        {{ room.name }}</p>
                </section>
                <!-- copy invite link for this room (group or personal) -->
                <svg @click.stop="copyInviteId(room.id)" :title="inviteFor(room.id)"
                    class="w-6 h-6 fill-white/40 hover:fill-white cursor-pointer flex-shrink-0 mr-2"
                    viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                    <path d="M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7c-2.76 0-5 2.24-5 5s2.24 5 5 5h4v-1.9H7c-1.71 0-3.1-1.39-3.1-3.1zM8 13h8v-2H8v2zm9-6h-4v1.9h4c1.71 0 3.1 1.39 3.1 3.1s-1.39 3.1-3.1 3.1h-4V17h4c2.76 0 5-2.24 5-5s-2.24-5-5-5z"/>
                </svg>
            </section>

            <!-- Users -->
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

        <!-- Floating create button -->
        <article class="sticky bottom-4 self-end mt-6 bg-body-900 p-2 rounded-full cursor-pointer hover:bg-white/10 shadow-lg" @click.stop="activeCreateDialog = true">
            <svg width="35" height="35" viewBox="0 0 35 35" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M27.7082 20.4166C28.0949 20.4166 28.4659 20.5703 28.7394 20.8438C29.0129 21.1173 29.1665 21.4882 29.1665 21.875V26.25H33.5415C33.9283 26.25 34.2992 26.4036 34.5727 26.6771C34.8462 26.9506 34.9998 27.3215 34.9998 27.7083C34.9998 28.0951 34.8462 28.466 34.5727 28.7395C34.2992 29.013 33.9283 29.1666 33.5415 29.1666H29.1665V33.5416C29.1665 33.9284 29.0129 34.2993 28.7394 34.5728C28.4659 34.8463 28.0949 35 27.7082 35C27.3214 35 26.9505 34.8463 26.677 34.5728C26.4035 34.2993 26.2498 33.9284 26.2498 33.5416V29.1666H21.8748C21.4881 29.1666 21.1171 29.013 20.8436 28.7395C20.5701 28.466 20.4165 28.0951 20.4165 27.7083C20.4165 27.3215 20.5701 26.9506 20.8436 26.6771C21.1171 26.4036 21.4881 26.25 21.8748 26.25H26.2498V21.875C26.2498 21.4882 26.4035 21.1173 26.677 20.8438C26.9505 20.5703 27.3214 20.4166 27.7082 20.4166Z" fill="white" />
                <path d="M30.2751 18.3312C30.8584 18.7687 32.0689 18.5208 32.0834 17.7916V17.5C32.0842 15.2367 31.5582 13.0043 30.547 10.9794C29.5358 8.9546 28.0672 7.19291 26.2575 5.8338C24.4477 4.47469 22.3464 3.55546 20.12 3.14887C17.8935 2.74228 15.6029 2.85949 13.4296 3.49122C11.2563 4.12295 9.25981 5.25185 7.59822 6.7886C5.93663 8.32534 4.65552 10.2277 3.8563 12.3452C3.05707 14.4627 2.76167 16.7371 2.99346 18.9885C3.22525 21.2399 3.97788 23.4064 5.19178 25.3166C5.36678 25.5937 5.33761 25.9583 5.13344 26.2062L2.11469 29.6625C1.92972 29.873 1.80934 30.1324 1.76795 30.4096C1.72656 30.6868 1.76591 30.97 1.8813 31.2254C1.99669 31.4808 2.18323 31.6975 2.4186 31.8497C2.65396 32.0018 2.92818 32.0829 3.20844 32.0833H17.7918C18.5209 32.0687 18.7689 30.8583 18.3314 30.275C17.8583 29.6219 17.5749 28.8508 17.5125 28.0468C17.4501 27.2429 17.6111 26.4373 17.9778 25.7191C18.3444 25.0008 18.9024 24.3979 19.5902 23.9769C20.278 23.5559 21.0687 23.3332 21.8751 23.3333H22.6043C22.7977 23.3333 22.9831 23.2565 23.1199 23.1197C23.2566 22.983 23.3334 22.7975 23.3334 22.6041V21.875C23.3333 21.0686 23.556 20.2778 23.9771 19.59C24.3981 18.9023 25.001 18.3443 25.7192 17.9776C26.4374 17.6109 27.243 17.4499 28.047 17.5123C28.851 17.5747 29.6221 17.8581 30.2751 18.3312Z" fill="white" />
            </svg>
        </article>
    </article>
</template>

<script lang="scss" scoped></script>
