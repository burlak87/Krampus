<script setup lang="ts">
import { User as UserComposable } from '~/composabels/User'
import { GroupManagment } from '~/composabels/GroupManagment'
import { apiClient } from '~/composabels/apiClient'

const props = defineProps(["RoomData"])
const settingUser = useSettingUser()
const userClass = new UserComposable()
const groupClass = new GroupManagment()

const members = ref<any[]>([])
const roomAvatar = ref<string>('')
const roomAvatarInput = ref<HTMLInputElement | null>(null)

function pickRoomAvatar() {
    roomAvatarInput.value?.click()
}

async function onRoomAvatarPick(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file || !props.RoomData?.id) return
    const dataUrl: string = await new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result))
        reader.onerror = reject
        reader.readAsDataURL(file)
    })
    try {
        await apiClient.setRoomAvatar(String(props.RoomData.id), dataUrl)
        roomAvatar.value = dataUrl
        if (props.RoomData) props.RoomData.avatar = dataUrl
    } catch (err) {
        console.error("[room avatar] upload failed", err)
    }
}

async function loadMembers() {
    const room = props.RoomData
    if (!room?.id) {
        members.value = []
        return
    }
    let ids: string[] = Array.isArray(room.members) ? room.members : []
    if (!ids.length) {
        const full = await groupClass.openGroup(String(room.id))
        ids = Array.isArray(full?.members) ? full!.members : []
    }
    if (!ids.length) {
        members.value = []
        return
    }
    const all = await userClass.getAllUser()
    const byId: Record<string, any> = {}
    for (const u of all) byId[String(u.id)] = u
    members.value = ids.map((id: string) => byId[String(id)] ?? { id, username: String(id) })
    roomAvatar.value = room.avatar ?? ''
    if (!roomAvatar.value) {
        const full = await groupClass.openGroup(String(room.id))
        roomAvatar.value = full?.avatar ?? ''
    }
}

onMounted(loadMembers)
watch(() => props.RoomData, loadMembers, { deep: true })
</script>

<template>
    <section class="h-full w-full flex flex-col gap-4">
        <article class="flex flex-col gap-4 px-2 py-6">
            <section class="flex flex-row justify-start items-center gap-3">
                <img @click.stop="pickRoomAvatar" :src="roomAvatar || ''"
                    class="w-12 h-12 rounded-full object-cover bg-white/10 cursor-pointer flex-shrink-0"
                    :title="settingUser.language == 'Englend' ? 'Change avatar' : 'Сменить аватар'">
                <input ref="roomAvatarInput" type="file" accept="image/*" class="hidden" @change="onRoomAvatarPick">
                <p class="text-[20px] font-bold text-white">{{ props.RoomData?.name }}</p>
            </section>
            <section class="flex flex-col gap-2 p-2 justify-start items-start">
                <p class="text-[15px] text-white/50 p-2">
                    {{ settingUser.language == 'Englend' ? 'Members' : 'Пользователи' }} ({{ members.length }})
                </p>
                <hr class="w-full h-[5px] bg-white rounded-full border-0">
            </section>
        </article>
        <article class="flex flex-col gap-4 px-5 py-2 items-start justify-start overflow-y-auto scrollbar-hide">
            <p v-if="members.length === 0" class="text-white/30 text-[16px]">
                {{ settingUser.language == 'Englend' ? 'No members' : 'Нет участников' }}
            </p>
            <section class="flex flex-row justify-start items-center gap-4 w-full"
                v-for="el in members" :key="el.id">
                <img class="w-10 h-10 rounded-full object-cover bg-white/10" :src="el.avatar || el.logo || ''" alt="logoUser">
                <p class="text-[18px] text-white font-bold">{{ el.username }}</p>
            </section>
        </article>
    </section>
</template>

<style scoped></style>
