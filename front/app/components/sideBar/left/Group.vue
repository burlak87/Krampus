<script lang="ts" setup>
import { GroupManagment } from '~/composabels/GroupManagment';
import { WebRTC } from '~/composabels/WebRTC';
import type { CallParticipant, genericRef, User } from '~~/types/other';



let groupClass: GroupManagment;
const WebSocketCall = ref([])
const groupName = ref()
const activeCreateDialog: genericRef<boolean> = ref(false)
const activeCreateChatDialog: genericRef<boolean> = ref(false)
const activeUserInWebRTCCall: genericRef<[{ id: String, users: [CallParticipant] }?]> = ref([])
const activeRoleSetting = ref()
const user = useUserStore()
const addUserEmail = ref()
const activeModelCreateRole = ref()
const settingUser = useSettingUser()

onMounted(async () => {
    groupClass = new GroupManagment()

    groupName.value = await groupClass.requestGroup()
    console.log(groupName.value)

})

async function createGroup(name: string) {
    console.log("[Group.vue] createGroup called with", name)
    await groupClass.createGroup(name)
    activeCreateDialog.value = false
    groupName.value = await groupClass.requestGroup()
}

/*watch(groupName, async (oldValue, newValue) => {
    if (oldValue !== newValue) {
        groupName.value = await groupClass.requestGroup()
        console.log(groupName.value)
    }
})*/

const activeGroup = ref();

const optionGroup = ref()

function checkUserCall(group: object) {

    (Object.values(group.chat)).forEach((el) => {
        el.forEach(async (room) => {
            if ("id" in room && room.type == "voice") {
                console.log(room)
                await WebSocketCall.value.push(markRaw(new WebRTC(room.id, "preliminary")))
            }
        });
    })

    console.log(WebSocketCall.value)

    const userGroup = groupClass.requreAllUser()
    console.log(userGroup)
    const user = useUserStore()
    console.log('WebSocketCall.value length:', WebSocketCall.value.length)
    console.log('WebSocketCall.value:', WebSocketCall.value)

    WebSocketCall.value.forEach((el: WebRTC, id) => {
        console.log(id)
        const idRoom = el.getIdRoom()
        console.log(el.getWebSoketObject())
        el.getWebSoketObject().onmessage = (event) => {

            const data = JSON.parse(event.data)
            console.log(data)
            if (data.type == 'Answer' && data.preliminary && data.action == 'checkUserActive' && data.idUserAnswer == user.userData.id) {
                console.log(data)
                userGroup.forEach((user) => {
                    if (user.id == data.idUserTarget && activeUserInWebRTCCall.value != undefined) {

                        if ((activeUserInWebRTCCall.value.filter((active) => active?.id == idRoom)).length) {
                            activeUserInWebRTCCall.value.forEach((userRoom) => {
                                if (userRoom?.id == el.getIdRoom()) {
                                    userRoom.users.push(user)
                                }
                            })
                        } else {
                            activeUserInWebRTCCall.value.push({
                                id: idRoom,
                                users: [
                                    {
                                        ...user,
                                        audio: true,
                                        video: false,
                                        muth: false
                                    } as CallParticipant
                                ]
                            })
                        }

                    }
                })
            } else if (data.type == "Status") {
                if (data.statusUser == "Active") {
                    console.log(data)
                    if (activeUserInWebRTCCall.value?.filter((userRoom) => userRoom.id == data.idRoom).length) {
                        activeUserInWebRTCCall.value?.forEach(userRoom => {
                            if (userRoom.id == data.idRoom) {
                                const indexUser = userRoom?.users.findIndex(user => user.id === data.idUserTarget)

                                if (indexUser !== -1 && indexUser != undefined) {
                                    userRoom.users[indexUser] = {
                                        ...userRoom.users[indexUser],
                                        audio: data.audio,
                                        video: data.video,
                                        muth: data.muth
                                    } as CallParticipant

                                    return
                                } else if (indexUser != undefined) {
                                    const user = userGroup.filter(user => user.id == data.idUserTarget)[0]
                                    userRoom?.users.push(
                                        {
                                            ...user,
                                            audio: data.audio,
                                            video: data.video,
                                            muth: data.muth
                                        } as CallParticipant)
                                }
                            }
                        })
                    } else {
                        const user = userGroup.filter(user => user.id == data.idUserTarget)[0]
                        activeUserInWebRTCCall.value.push({
                            id: idRoom,
                            users: [
                                {
                                    ...user,
                                    audio: true,
                                    video: false,
                                    muth: false
                                } as CallParticipant
                            ]
                        })
                    }

                } else if (data.statusUser == "Close") {
                    activeUserInWebRTCCall.value?.forEach((userRoom, id) => {
                        if (userRoom?.id == data.idRoom) {
                            const userIndex = userRoom?.users.findIndex(user => user.id == data.idUserTarget)

                            if (userIndex !== -1 && userIndex != undefined) {
                                userRoom?.users.splice(userIndex, 1)
                            } else {
                                console.log("Тут что-то не так :(")
                            }

                            if (!userRoom?.users.length && userIndex != undefined) {
                                console.log("ООО даааа")
                                activeUserInWebRTCCall.value?.splice(userIndex, 1)
                            }
                        }
                    })
                }
            }
        }
        el.sendSignalCheckUser(true)
    })
    console.log(WebSocketCall.value)
    console.log(activeUserInWebRTCCall.value)
}

watch(activeUserInWebRTCCall, () => {
    console.log(activeUserInWebRTCCall.value)
}, { deep: true })


function openOptionChat() {

}


watch(activeGroup, async () => {
    for (let el of WebSocketCall.value) {
        console.log(el)
        const element = (el as WebRTC)
        await element.getWebSoketObject().close()
    }
    console.log(WebSocketCall.value)
    activeUserInWebRTCCall.value = []
    WebSocketCall.value = []
    console.log(activeGroup.value)
    checkUserCall(activeGroup.value)
}, { immediate: false, deep: false })

function returnArrayUserCall(id) {
    let data = JSON.parse(JSON.stringify(activeUserInWebRTCCall.value?.filter((userRoom) => userRoom?.id == id)))
    data = data[0]
    return data.users
}

</script>

<template>
    <ModalCreateGroup @createGroup="(nameGroup: string) => createGroup(nameGroup)" @dropDialog="activeCreateDialog = false" v-if="activeCreateDialog" />
    <ModalCreateChat @createFolder="async (nameFolder: string) => { await groupClass.createFolder(nameFolder, activeGroup.id)}" @createChat="(nameChat: string, typeChat: string, idFolder: string) => {groupClass.createChat(nameChat, typeChat, idFolder, activeGroup.id)}" :folderGroup="() => {/*Тут должна быть какая-то логика, но я хз, напиши*/}" @dropDialog="activeCreateChatDialog = false" v-if="activeCreateChatDialog"></ModalCreateChat>
    <ModalCreateRole @createRole="async (nameRole: string, settingRole: object) => {await groupClass.createRole(nameRole, settingRole, activeGroup.id)}" v-if="activeModelCreateRole" @dropDialog="activeModelCreateRole = false"/>
    <article :class="{
        grid: activeGroup,
        'h-screen': true,
        'grid-cols-5': activeGroup
    }">
        <section :class="{
            'flex': true,
            'flex-col': true,
            'h-screen': true,
            'justify-start': activeGroup,
            'items-center': activeGroup,
            'pt-10': true,
            'border-r-3': activeGroup,
            'border-body-100': activeGroup,
            'gap-6': true,
            'pl-2': !activeGroup,
            'pb-10': !activeGroup,
            'pb-25': activeGroup,
            'px-2': activeGroup,
            'w-full': !activeGroup,
        }">
            <article class="flex flex-col h-full gap-6 overflow-y-auto scrollbar-hide scroll-smooth w-full">
                <article :class="{
                    flex: true,
                    'hover:bg-white/10': !activeGroup,
                    'flex-row': true,
                    'gap-10': true,
                    'justify-center': activeGroup,
                    'justify-start': !activeGroup,
                    'items-center': true,
                    'w-full': !activeGroup,
                }" v-for="el in groupName" :key="el.id" @click.stop="
                    () => {
                        activeGroup = groupClass.openGroup(el.id);
                        console.log(activeGroup)
                    }
                ">
                    <img :src="el.src || ''" class="w-10 h-10 rounded-full bg-white/10" />
                    <p :class="{
                        'text-[18px]': true,
                        'text-white': true,
                        'font-bold': true,
                        'hidden': activeGroup,
                    }">
                        {{ el.name }}
                    </p>
                </article>
            </article>
            <article
                :class="{ 'sticky bottom-0 left-[100%] w-fit py-5 flex flex-row items-center bg-inherit': true, 'justify-end gap-10 px-4': !activeGroup, 'justify-center w-full': activeGroup }">
                <svg @click.stop="" width="35" height="35" viewBox="0 0 50 50" fill="none"
                    xmlns="http://www.w3.org/2000/svg">
                    <path
                        d="M20.2083 6.02118C20.5833 5.87535 20.875 5.52118 20.9792 5.12535C21.219 4.24267 21.7427 3.46346 22.4694 2.90792C23.196 2.35238 24.0853 2.05139 25 2.05139C25.9147 2.05139 26.804 2.35238 27.5306 2.90792C28.2573 3.46346 28.781 4.24267 29.0208 5.12535C29.125 5.54202 29.4167 5.87535 29.8125 6.02118C32.6683 7.01954 35.143 8.88134 36.8936 11.3487C38.6443 13.816 39.5843 16.7667 39.5833 19.792V24.1462C39.5833 24.3962 39.6875 24.6462 39.8542 24.8337L42.1458 27.3753C43.1801 28.5247 43.7517 30.0166 43.75 31.5628V32.1462C43.75 33.542 43.0417 34.8337 41.7708 35.3962C39.0417 36.6462 33.4375 38.542 25 38.542C16.5625 38.542 10.9583 36.6462 8.22917 35.417C6.95833 34.8337 6.25 33.542 6.25 32.1462V31.5628C6.25347 30.0238 6.82467 28.5402 7.85417 27.3962L10.1458 24.8337C10.3171 24.6453 10.4134 24.4007 10.4167 24.1462V19.792C10.4173 16.7642 11.3603 13.8116 13.1149 11.344C14.8694 8.87635 17.3486 7.01604 20.2083 6.02118ZM19.125 41.3337C19.0779 41.3277 19.03 41.3319 18.9846 41.346C18.9392 41.3601 18.8974 41.3837 18.8619 41.4153C18.8264 41.4468 18.798 41.4856 18.7787 41.529C18.7594 41.5725 18.7496 41.6195 18.75 41.667C18.75 43.3246 19.4085 44.9143 20.5806 46.0864C21.7527 47.2585 23.3424 47.917 25 47.917C26.6576 47.917 28.2473 47.2585 29.4194 46.0864C30.5915 44.9143 31.25 43.3246 31.25 41.667C31.25 41.4587 31.0625 41.3129 30.875 41.3337C26.971 41.7794 23.029 41.7794 19.125 41.3337Z"
                        fill="white" />
                </svg>
                <svg @click.stop="activeCreateDialog = true" v-if="!activeGroup" width="35" height="35"
                    viewBox="0 0 35 35" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path
                        d="M27.7082 20.4166C28.0949 20.4166 28.4659 20.5703 28.7394 20.8438C29.0129 21.1173 29.1665 21.4882 29.1665 21.875V26.25H33.5415C33.9283 26.25 34.2992 26.4036 34.5727 26.6771C34.8462 26.9506 34.9998 27.3215 34.9998 27.7083C34.9998 28.0951 34.8462 28.466 34.5727 28.7395C34.2992 29.013 33.9283 29.1666 33.5415 29.1666H29.1665V33.5416C29.1665 33.9284 29.0129 34.2993 28.7394 34.5728C28.4659 34.8463 28.0949 35 27.7082 35C27.3214 35 26.9505 34.8463 26.677 34.5728C26.4035 34.2993 26.2498 33.9284 26.2498 33.5416V29.1666H21.8748C21.4881 29.1666 21.1171 29.013 20.8436 28.7395C20.5701 28.466 20.4165 28.0951 20.4165 27.7083C20.4165 27.3215 20.5701 26.9506 20.8436 26.6771C21.1171 26.4036 21.4881 26.25 21.8748 26.25H26.2498V21.875C26.2498 21.4882 26.4035 21.1173 26.677 20.8438C26.9505 20.5703 27.3214 20.4166 27.7082 20.4166Z"
                        fill="white" />
                    <path
                        d="M30.2751 18.3312C30.8584 18.7687 32.0689 18.5208 32.0834 17.7916V17.5C32.0842 15.2367 31.5582 13.0043 30.547 10.9794C29.5358 8.9546 28.0672 7.19291 26.2575 5.8338C24.4477 4.47469 22.3464 3.55546 20.12 3.14887C17.8935 2.74228 15.6029 2.85949 13.4296 3.49122C11.2563 4.12295 9.25981 5.25185 7.59822 6.7886C5.93663 8.32534 4.65552 10.2277 3.8563 12.3452C3.05707 14.4627 2.76167 16.7371 2.99346 18.9885C3.22525 21.2399 3.97788 23.4064 5.19178 25.3166C5.36678 25.5937 5.33761 25.9583 5.13344 26.2062L2.11469 29.6625C1.92972 29.873 1.80934 30.1324 1.76795 30.4096C1.72656 30.6868 1.76591 30.97 1.8813 31.2254C1.99669 31.4808 2.18323 31.6975 2.4186 31.8497C2.65396 32.0018 2.92818 32.0829 3.20844 32.0833H17.7918C18.5209 32.0687 18.7689 30.8583 18.3314 30.275C17.8583 29.6219 17.5749 28.8508 17.5125 28.0468C17.4501 27.2429 17.6111 26.4373 17.9778 25.7191C18.3444 25.0008 18.9024 24.3979 19.5902 23.9769C20.278 23.5559 21.0687 23.3332 21.8751 23.3333H22.6043C22.7977 23.3333 22.9831 23.2565 23.1199 23.1197C23.2566 22.983 23.3334 22.7975 23.3334 22.6041V21.875C23.3333 21.0686 23.556 20.2778 23.9771 19.59C24.3981 18.9023 25.001 18.3443 25.7192 17.9776C26.4374 17.6109 27.243 17.4499 28.047 17.5123C28.851 17.5747 29.6221 17.8581 30.2751 18.3312Z"
                        fill="white" />
                </svg>
            </article>
        </section>

        <section v-if="activeGroup" :class="{
            'flex': true,
            'flex-col': true,
            'col-start-2': true,
            'relative': true,
            'col-span-4': true,
            'h-screen': true,
            'animate-group': activeGroup,
        }">
            <article v-if="optionGroup" class="bg-body-500 z-10 absolute top-0 left-0 w-full h-screen">
                <section class="flex justify-end p-2">
                    <svg @click.stop="optionGroup = false" viewBox="0 0 32 32" class="w-8 fill-white"
                        xmlns="http://www.w3.org/2000/svg">
                        <path
                            d="M4,29a1,1,0,0,1-.71-.29,1,1,0,0,1,0-1.42l24-24a1,1,0,1,1,1.42,1.42l-24,24A1,1,0,0,1,4,29Z" />
                        <path
                            d="M28,29a1,1,0,0,1-.71-.29l-24-24A1,1,0,0,1,4.71,3.29l24,24a1,1,0,0,1,0,1.42A1,1,0,0,1,28,29Z" />
                    </svg>
                </section>
                <section
                    class="flex flex-col h-screen pb-50 gap-6 px-1 overflow-y-auto scrollbar-hide scroll-smooth w-full">
                    <article>
                        <p class=" pl-1 text-[20px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Name group:' : 'Название группы:'}}</p>
                        <input
                            class="w-full border-b-3 border-white focus:outline-none placeholder:text-[20px] placeholder:text-white text-white text-[20px] py-2 pl-1"
                            :placeholder="activeGroup.name">
                    </article>
                    <article>
                        <p class=" pl-1 text-[20px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Description group:' : 'Описание группы:'}}</p>
                        <textarea
                            class="box-border scrollbar-hide scroll-smooth resize-none  overflow-y-auto w-full border-b-3 border-white focus:outline-none placeholder:text-[20px] placeholder:text-white text-white text-[20px] py-2 pl-1"
                            :placeholder="activeGroup.description"></textarea>
                    </article>
                    <article>
                        <p class=" pl-1 text-[20px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Users group:' : 'Пользователи группы:'}}</p>
                        <section
                            class="w-full h-50 border-b-3 pt-2 border-white scrollbar-hide scroll-smooth resize-none  overflow-y-auto">
                            <OtherSideBarGroupSettingUser
                                @deleteUserGroup="(id: string) => groupClass.deleteUserInGroup(activeGroup.id, id)"
                                v-for="el in groupClass.requreAllUser()" :key="el.id" :user-group="el" />
                        </section>
                    </article>
                    <article>
                        <p class="pl-1 text-[20px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Role group:' : 'Роли группы:'}}</p>
                        <section class="w-full h-50  pt-2  scrollbar-hide scroll-smooth resize-none  overflow-y-auto">
                            <OtherSideBarGroupSettingRole
                                @settingRole="(roleData: object) => { activeRoleSetting = roleData }"
                                @deleteRoleGroup="(id: string) => groupClass.deleteRoleInGroup(activeGroup.id, id)"
                                v-for="el in groupClass.requreAllRole(activeGroup.id)" :key="el.id" />
                        </section>
                        <section class="w-full flex border-b-3 border-white justify-center items-center p-2">
                            <button @click.stop="activeModelCreateRole = true"
                                class="border-1 text-white/50 hover:text-white px-5 py-2 text-[20px]  border-white w-3/4 ">{{settingUser.language == 'Englend' ? 'Create role' : 'Создать роль'}}</button>
                        </section>
                    </article>
                    <article>
                        <p class="pl-1 pb-5 text-[20px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Add user:' : 'Добавить пользователя:'}}</p>
                        <input v-model="addUserEmail"
                            class="w-full pb-4 focus:outline-none placeholder:text-[20px] placeholder:text-white text-white text-[20px] py-2 pl-1"
                            :placeholder="settingUser.language == 'Englend' ? 'emailUser' : 'Почта пользователя'">
                        <section class="w-full flex justify-center p-2 border-b-3 border-white">
                            <button @click.stop="groupClass.addNewUser(activeGroup.id, addUserEmail)"
                                class="border-1 text-white/50 hover:text-white px-5 py-2 text-[20px]  border-white w-3/4 ">{{settingUser.language == 'Englend' ? 'Add user:' : 'Добавить пользователя:'}}</button>
                        </section>
                    </article>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <article class="flex flex-row justify-between px-4 py-2 border-b-4 border-body-100">
                    <section class="flex flex-col gap-1 text-start">
                        <h2 class="text-[26px] text-white/80">{{ activeGroup.name }}</h2>
                        <p class="text-[16px] text-white">{{ activeGroup.users }} {{settingUser.language == 'Englend' ? 'members' : 'пользователей'}}</p>
                    </section>
                    <section class="flex flex-row gap-10 justify-center items-center">
                        <svg @click="optionGroup = true" width="7" height="34" viewBox="0 0 7 34" fill="none"
                            xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd"
                                d="M0 3.33333C0 4.21739 0.35119 5.06523 0.976311 5.69036C1.60143 6.31548 2.44928 6.66667 3.33333 6.66667C4.21739 6.66667 5.06523 6.31548 5.69036 5.69036C6.31548 5.06523 6.66667 4.21739 6.66667 3.33333C6.66667 2.44928 6.31548 1.60143 5.69036 0.976311C5.06523 0.351189 4.21739 0 3.33333 0C2.44928 0 1.60143 0.351189 0.976311 0.976311C0.35119 1.60143 0 2.44928 0 3.33333ZM3.33333 20C2.44928 20 1.60143 19.6488 0.976311 19.0237C0.35119 18.3986 0 17.5507 0 16.6667C0 15.7826 0.35119 14.9348 0.976311 14.3096C1.60143 13.6845 2.44928 13.3333 3.33333 13.3333C4.21739 13.3333 5.06523 13.6845 5.69036 14.3096C6.31548 14.9348 6.66667 15.7826 6.66667 16.6667C6.66667 17.5507 6.31548 18.3986 5.69036 19.0237C5.06523 19.6488 4.21739 20 3.33333 20ZM3.33333 33.3333C2.44928 33.3333 1.60143 32.9821 0.976311 32.357C0.35119 31.7319 0 30.8841 0 30C0 29.1159 0.35119 28.2681 0.976311 27.643C1.60143 27.0179 2.44928 26.6667 3.33333 26.6667C4.21739 26.6667 5.06523 27.0179 5.69036 27.643C6.31548 28.2681 6.66667 29.1159 6.66667 30C6.66667 30.8841 6.31548 31.7319 5.69036 32.357C5.06523 32.9821 4.21739 33.3333 3.33333 33.3333Z"
                                fill="white" />
                        </svg>
                        <svg @click="async () => {await groupClass.liveGroup(user.userData?.id, activeGroup.id)}" width="34" height="40" viewBox="0 0 34 40" fill="none"
                            xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M33.1767 19.169L26.9494 12.942C26.4903 12.4828 25.7473 12.4822 25.2881 12.9383C24.8277 13.3947 24.8234 14.1394 25.2777 14.6017L29.4329 18.8281H12.7054C12.0555 18.8281 11.5283 19.3499 11.5283 20C11.5283 20.6501 12.0555 21.1719 12.7054 21.1719H29.5027L25.2844 25.3928C24.8252 25.8523 24.8252 26.5993 25.2844 27.0588C25.5143 27.2886 25.8159 27.4044 26.1169 27.4044C26.4179 27.4044 26.7195 27.2898 26.9494 27.0599L33.1767 20.8331C33.3973 20.6124 33.5212 20.3131 33.5212 20.001C33.5212 19.689 33.3974 19.3897 33.1767 19.169Z"
                                fill="white" />
                            <path
                                d="M24.0625 30.7638C23.4127 30.7638 22.8906 31.2907 22.8906 31.9408V37.6562H2.34375V2.34375H22.8906V8.06156C22.8906 8.71168 23.4127 9.23859 24.0625 9.23859C24.7123 9.23859 25.2344 8.71168 25.2344 8.06156V1.16852C25.2344 0.518359 24.6996 0 24.0498 0H1.17598C0.526172 0 0 0.518359 0 1.16852V38.8339C0 39.484 0.526172 40 1.17602 40H24.0499C24.6997 40 25.2344 39.484 25.2344 38.8339V31.9408C25.2344 31.2907 24.7123 30.7638 24.0625 30.7638Z"
                                fill="white" />
                        </svg>
                    </section>
                </article>
                <article class="h-screen pb-60">
                    <section class="flex flex-col h-full  gap-2 overflow-y-auto scrollbar-hide scroll-smooth w-full">
                        <section class="flex flex-col gap-2 px-5 text-[22px] text-white"
                            v-for="el in Object.keys(activeGroup.chat)" :key="activeGroup.id">
                            <article @click="($event) => {
                                const target = ($event.currentTarget as HTMLElement)
                                if (target?.children[0]?.classList.contains('rotate-90')) {
                                    target?.parentElement?.children[1]?.classList.remove('hidden');
                                    target?.children[0]?.classList.remove('rotate-90');
                                } else {
                                    target?.parentElement?.children[1]?.classList.add('hidden');
                                    target?.children[0]?.classList.add('rotate-90');
                                }
                            }"
                                class="flex relative flex-row justify-start gap-5 mb-2 items-center hover:bg-white/10 p-2">
                                <svg class="fill-white/100 rotate-90" width="25" height="15" viewBox="0 0 25 15"
                                    fill="none" xmlns="http://www.w3.org/2000/svg">
                                    <path
                                        d="M0.526313 0.659511C0.858915 0.318085 1.30608 0.126843 1.77182 0.126843C2.23756 0.126843 2.68473 0.318085 3.01733 0.659511L12.4476 10.5169L21.8779 0.640876C22.0387 0.447095 22.2361 0.29022 22.4576 0.180093C22.679 0.0699668 22.9199 0.00896447 23.1649 0.00091577C23.41 -0.00713293 23.654 0.037945 23.8816 0.133321C24.1093 0.228696 24.3156 0.372312 24.4877 0.555157C24.6599 0.738001 24.7941 0.956128 24.8819 1.19585C24.9697 1.43558 25.0093 1.69173 24.9982 1.94823C24.987 2.20473 24.9253 2.45604 24.817 2.68641C24.7088 2.91678 24.5562 3.12122 24.3689 3.28691L13.6931 14.4673C13.3605 14.8088 12.9133 15 12.4476 15C11.9819 15 11.5347 14.8088 11.2021 14.4673L0.526313 3.28691C0.359542 3.11368 0.227172 2.90759 0.13684 2.68052C0.0465073 2.45344 0 2.20988 0 1.96389C0 1.7179 0.0465073 1.47434 0.13684 1.24727C0.227172 1.0202 0.359542 0.814104 0.526313 0.640876V0.659511Z" />
                                </svg>
                                <p class="font-bold">{{ el }}</p>
                                <section @click="openOptionChat()" class="flex flex-col gap-[3px] absolute left-[90%]">
                                    <div class="w-[5px] h-[5px] rounded-full bg-white"></div>
                                    <div class="w-[5px] h-[5px] rounded-full bg-white"></div>
                                    <div class="w-[5px] h-[5px] rounded-full bg-white"></div>
                                </section>
                            </article>
                            <article class="flex flex-col gap-2 hidden">
                                <article
                                    class="flex flex-col gap-2 hover:bg-white/10 p-2 justify-between pl-5 items-start"
                                    @click='$emit("openChat", [chat.type, chat])' v-for="chat in activeGroup.chat[el]"
                                    :key="chat.id">
                                    <section class="flex flex-row gap-5 justify-between items-center">
                                        <article class="flex flex-row gap-5 justify-start items-center">
                                            <svg v-if="chat.type == 'chat'" width="30" height="26" viewBox="0 0 28 24"
                                                fill="none" xmlns="http://www.w3.org/2000/svg">
                                                <path
                                                    d="M7.51444 2.13923C5.24165 3.56915 3.52251 5.66358 2.61795 8.10463C1.71338 10.5457 1.67276 13.2001 2.50226 15.6651C2.73218 16.385 2.70919 17.1485 2.27234 17.7594L0.364038 20.6391C0.141344 20.9684 0.0161075 21.3485 0.00145149 21.7396C-0.0132045 22.1307 0.0832618 22.5183 0.280739 22.8618C0.478216 23.2053 0.769432 23.4921 1.12388 23.6922C1.47832 23.8922 1.88294 23.9981 2.29534 23.9987H14.9407C18.3478 24.0454 21.6353 22.8088 24.0832 20.5597C26.531 18.3106 27.9395 15.2326 28 12C27.9395 8.7674 26.531 5.68935 24.0832 3.4403C21.6353 1.19124 18.3478 -0.0454209 14.9407 0.00127599C12.1817 0.00127599 9.60668 0.786646 7.51444 2.13923Z"
                                                    fill="white" />
                                            </svg>
                                            <svg v-else width="30" height="26" viewBox="0 0 30 26" fill="none"
                                                xmlns="http://www.w3.org/2000/svg">
                                                <path
                                                    d="M15 3.25035C15 2.96303 14.8683 2.68748 14.6339 2.48432C14.3995 2.28115 14.0815 2.16702 13.75 2.16702H13.675C13.5015 2.166 13.3297 2.1963 13.1704 2.25598C13.0112 2.31566 12.868 2.40343 12.75 2.51368L7.4 7.58368H3.75C3.41848 7.58368 3.10054 7.69782 2.86612 7.90098C2.6317 8.10415 2.5 8.3797 2.5 8.66702V17.3337C2.5 17.621 2.6317 17.8966 2.86612 18.0997C3.10054 18.3029 3.41848 18.417 3.75 18.417H7.4L12.75 23.487C12.868 23.5973 13.0112 23.685 13.1704 23.7447C13.3297 23.8044 13.5015 23.8347 13.675 23.8337H13.75C14.0815 23.8337 14.3995 23.7195 14.6339 23.5164C14.8683 23.3132 15 23.0377 15 22.7503V3.25035ZM18.875 22.4795C18.15 22.6312 17.5 22.122 17.5 21.4828V21.4503C17.5 20.9087 17.9625 20.4537 18.5625 20.3128C20.4108 19.8729 22.0413 18.919 23.2034 17.5979C24.3655 16.2768 24.9949 14.6616 24.9949 13.0003C24.9949 11.3391 24.3655 9.72387 23.2034 8.40277C22.0413 7.08168 20.4108 6.12785 18.5625 5.68785C18.2657 5.62597 18.0007 5.48096 17.8086 5.27531C17.6165 5.06966 17.508 4.81485 17.5 4.55035V4.51785C17.5 3.86785 18.15 3.36952 18.875 3.52118C21.3306 4.03354 23.5158 5.24713 25.0788 6.9666C26.6419 8.68607 27.4918 10.8114 27.4918 13.0003C27.4918 15.1893 26.6419 17.3146 25.0788 19.0341C23.5158 20.7536 21.3306 21.9672 18.875 22.4795Z"
                                                    fill="white" />
                                                <path
                                                    d="M18.95 17.8862C18.2375 18.1895 17.5 17.6695 17.5 16.987V16.8353C17.5 16.3695 17.85 15.9687 18.2875 15.7303C18.8138 15.4357 19.2465 15.0315 19.5462 14.5546C19.8458 14.0777 20.0028 13.5433 20.0028 13.0003C20.0028 12.4573 19.8458 11.923 19.5462 11.4461C19.2465 10.9692 18.8138 10.565 18.2875 10.2703C17.85 10.0212 17.5 9.62032 17.5 9.16532V9.01365C17.5 8.33115 18.2375 7.82199 18.95 8.11449C20.0135 8.55564 20.9113 9.24848 21.5397 10.113C22.1682 10.9776 22.5016 11.9785 22.5016 13.0003C22.5016 14.0221 22.1682 15.0231 21.5397 15.8876C20.9113 16.7522 20.0135 17.445 18.95 17.8862Z"
                                                    fill="white" />
                                            </svg>
                                            <p>{{ chat.name }}</p>
                                        </article>
                                        <div class="p-2 bg-white rounded-full" v-if="chat.newMessage">
                                        </div>
                                    </section>
                                    <article class="flex flex-col gap-5"
                                        v-if="activeUserInWebRTCCall.length && chat.type == 'voice'">
                                        <section v-for="user in returnArrayUserCall(chat.id)"
                                            class="flex flex-row items-center gap-5 pl-5" :key="user.id">
                                            <img class="w-7 h-7 rounded-full" :src="user.logo" alt="">
                                            <p class="text-[15px] text-white font-bold">{{ user.userName }}</p>
                                            <article class="flex flex-row gap-5 items-center"
                                                v-if="!user.audio || user.muth || !user.video">
                                                <svg v-if="!user.audio" width="20" height="20" viewBox="0 0 40 40"
                                                    fill="none" xmlns="http://www.w3.org/2000/svg">
                                                    <path
                                                        d="M4.5 37.8335L37.8333 4.50016C38.0741 4.17921 38.1909 3.7822 38.1625 3.38202C38.134 2.98183 37.9622 2.60534 37.6785 2.32165C37.3948 2.03797 37.0183 1.86613 36.6181 1.83769C36.218 1.80925 35.821 1.92612 35.5 2.16683L2.16667 35.5002C1.97564 35.6434 1.81762 35.8261 1.70333 36.0357C1.58903 36.2454 1.52112 36.4772 1.50419 36.7154C1.48727 36.9535 1.52172 37.1926 1.60522 37.4163C1.68871 37.64 1.81931 37.8432 1.98816 38.012C2.157 38.1809 2.36016 38.3115 2.58387 38.395C2.80759 38.4784 3.04663 38.5129 3.28482 38.496C3.523 38.479 3.75477 38.4111 3.96442 38.2968C4.17408 38.1825 4.35673 38.0245 4.5 37.8335ZM18 28.8668C17.65 29.2168 17.8333 29.8335 18.3333 29.9002V33.3335H15C14.558 33.3335 14.134 33.5091 13.8215 33.8217C13.5089 34.1342 13.3333 34.5581 13.3333 35.0002C13.3333 35.4422 13.5089 35.8661 13.8215 36.1787C14.134 36.4912 14.558 36.6668 15 36.6668H25C25.442 36.6668 25.8659 36.4912 26.1785 36.1787C26.4911 35.8661 26.6667 35.4422 26.6667 35.0002C26.6667 34.5581 26.4911 34.1342 26.1785 33.8217C25.8659 33.5091 25.442 33.3335 25 33.3335H21.6667V29.9002C24.8895 29.4941 27.8533 27.9255 30.0015 25.4889C32.1497 23.0522 33.3344 19.9152 33.3333 16.6668C33.3333 16.2248 33.1577 15.8009 32.8452 15.4883C32.5326 15.1758 32.1087 15.0002 31.6667 15.0002C31.2246 15.0002 30.8007 15.1758 30.4882 15.4883C30.1756 15.8009 30 16.2248 30 16.6668C30 19.0835 29.1333 21.3168 27.7 23.0502L27.6667 23.0835C26.7881 24.1394 25.7012 25.0026 24.4737 25.6192C23.2463 26.2358 21.9049 26.5924 20.5333 26.6668C20.3201 26.6775 20.1183 26.7665 19.9667 26.9168L18 28.8835V28.8668ZM25.6 7.5335C25.85 7.2835 25.9167 6.90017 25.7333 6.60016C24.988 5.34406 23.8508 4.36748 22.4965 3.82053C21.1422 3.27359 19.6457 3.18654 18.2371 3.57276C16.8285 3.95899 15.5857 4.79712 14.6998 5.95834C13.8138 7.11955 13.3338 8.53958 13.3333 10.0002V16.6668C13.3333 17.1668 13.3833 17.6335 13.5 18.1002C13.6167 18.6668 14.3167 18.8168 14.7333 18.4002L25.6 7.5335ZM8.43333 23.3002C8.7 23.7668 9.31667 23.8168 9.68333 23.4502L10.9333 22.2002C11.2 21.9335 11.25 21.5335 11.0667 21.1835C10.3601 19.7831 9.99456 18.2354 10 16.6668C10 16.2248 9.8244 15.8009 9.51184 15.4883C9.19928 15.1758 8.77536 15.0002 8.33333 15.0002C7.89131 15.0002 7.46738 15.1758 7.15482 15.4883C6.84226 15.8009 6.66667 16.2248 6.66667 16.6668C6.66667 19.0835 7.31667 21.3502 8.43333 23.3002Z"
                                                        fill="#F23F42" />
                                                </svg>
                                                <svg v-if="user.muth" width="20" height="20" viewBox="0 0 40 40"
                                                    fill="none" xmlns="http://www.w3.org/2000/svg">
                                                    <path
                                                        d="M37.8333 4.50003C38.0741 4.17907 38.1909 3.78206 38.1625 3.38188C38.134 2.9817 37.9622 2.6052 37.6785 2.32152C37.3948 2.03783 37.0183 1.86599 36.6181 1.83755C36.218 1.80911 35.821 1.92598 35.5 2.16669L2.16667 35.5C1.97564 35.6433 1.81762 35.8259 1.70333 36.0356C1.58903 36.2453 1.52112 36.477 1.50419 36.7152C1.48727 36.9534 1.52172 37.1924 1.60522 37.4162C1.68871 37.6399 1.81931 37.843 1.98816 38.0119C2.157 38.1807 2.36016 38.3113 2.58387 38.3948C2.80759 38.4783 3.04663 38.5128 3.28482 38.4958C3.523 38.4789 3.75477 38.411 3.96442 38.2967C4.17408 38.1824 4.35673 38.0244 4.5 37.8334L37.8333 4.50003ZM28.4333 4.90003C28.5243 4.81211 28.5932 4.70387 28.6342 4.58419C28.6753 4.4645 28.6873 4.33679 28.6694 4.21153C28.6516 4.08628 28.6042 3.96705 28.5313 3.86365C28.4584 3.76025 28.362 3.67561 28.25 3.61669C24.8058 1.88407 20.903 1.28138 17.0965 1.89431C13.2901 2.50725 9.77374 4.30462 7.0475 7.03086C4.32126 9.7571 2.52389 13.2734 1.91095 17.0799C1.29801 20.8863 1.90071 24.7891 3.63333 28.2334C3.86667 28.7334 4.51667 28.8167 4.9 28.4334L10.2333 23.1C10.65 22.6834 10.4833 21.9667 9.9 21.8334C9.34054 21.7197 8.77088 21.6639 8.2 21.6667H5.08333C4.78795 19.0327 5.19577 16.3675 6.26527 13.9424C7.33476 11.5173 9.02775 9.41881 11.1719 7.86068C13.316 6.30254 15.8347 5.34036 18.4716 5.07207C21.1085 4.80379 23.7693 5.23899 26.1833 6.33336C26.5167 6.48336 26.9167 6.41669 27.1667 6.16669L28.4333 4.90003ZM33.6667 13.8C33.5935 13.6409 33.5703 13.4634 33.6001 13.2908C33.6298 13.1183 33.7112 12.9588 33.8333 12.8334L35.1 11.5667C35.1879 11.4757 35.2962 11.4068 35.4158 11.3658C35.5355 11.3248 35.6632 11.3127 35.7885 11.3306C35.9137 11.3485 36.033 11.3958 36.1364 11.4687C36.2398 11.5417 36.3244 11.6381 36.3833 11.75C38.5134 15.9824 38.9222 20.875 37.524 25.4022C36.1258 29.9293 33.0291 33.7393 28.8833 36.0334C26.7667 37.2167 24.3333 36.4167 22.9667 34.8C22.2306 33.9285 21.8036 32.8379 21.7523 31.6983C21.7011 30.5586 22.0284 29.4341 22.6833 28.5L24.9833 25.2167C25.7528 24.119 26.7758 23.2231 27.9654 22.6051C29.155 21.9871 30.4761 21.6652 31.8167 21.6667H34.9167C35.2083 18.9799 34.7768 16.264 33.6667 13.8ZM16.8333 29.8334C17.25 29.4167 17.9167 29.5334 18.0667 30.0667C18.2944 30.88 18.3193 31.7368 18.1392 32.562C17.959 33.3872 17.5793 34.1556 17.0333 34.8C16.3436 35.6604 15.376 36.2541 14.2965 36.4791C13.2169 36.7041 12.0927 36.5465 11.1167 36.0334C11.0809 36.0138 11.0501 35.9864 11.0266 35.9531C11.0031 35.9198 10.9875 35.8816 10.9811 35.8414C10.9746 35.8012 10.9775 35.76 10.9894 35.721C11.0014 35.6821 11.0221 35.6464 11.05 35.6167L16.85 29.8167L16.8333 29.8334Z"
                                                        fill="#F23F42" />
                                                </svg>
                                                <svg v-if="!user.video" width="20" height="20" viewBox="0 0 40 40"
                                                    fill="none" xmlns="http://www.w3.org/2000/svg">
                                                    <path class="fill-red-500"
                                                        d="M6.66699 6.66675C5.34091 6.66675 4.06914 7.19353 3.13146 8.13121C2.19378 9.0689 1.66699 10.3407 1.66699 11.6667V28.3334C1.66699 29.6595 2.19378 30.9313 3.13146 31.8689C4.06914 32.8066 5.34091 33.3334 6.66699 33.3334H25.0003C26.3264 33.3334 27.5982 32.8066 28.5359 31.8689C29.4735 30.9313 30.0003 29.6595 30.0003 28.3334V24.8001C29.9982 25.1107 30.0828 25.4157 30.2448 25.6808C30.4068 25.9458 30.6396 26.1603 30.917 26.3001L35.917 28.8001C36.172 28.9286 36.4557 28.9894 36.7409 28.9767C37.0262 28.9641 37.3034 28.8784 37.546 28.7278C37.7886 28.5772 37.9884 28.3668 38.1263 28.1168C38.2643 27.8668 38.3357 27.5856 38.3337 27.3001V12.7001C38.3357 12.4146 38.2643 12.1333 38.1263 11.8833C37.9884 11.6333 37.7886 11.4229 37.546 11.2724C37.3034 11.1218 37.0262 11.0361 36.7409 11.0234C36.4557 11.0108 36.172 11.0716 35.917 11.2001L30.917 13.7001C30.6396 13.8399 30.4068 14.0544 30.2448 14.3194C30.0828 14.5844 29.9982 14.8895 30.0003 15.2001V11.6667C30.0003 10.3407 29.4735 9.0689 28.5359 8.13121C27.5982 7.19353 26.3264 6.66675 25.0003 6.66675H6.66699Z" />
                                                </svg>
                                            </article>
                                        </section>
                                    </article>
                                </article>
                            </article>
                        </section>
                    </section>
                    <section
                        class="sticky bottom-0 left-[100%] w-fit py-4 px-2 flex flex-row items-center flex justify-end bg-inherit">
                        <article class="bg-body-900 p-2 rounded-full" @click.stop="activeCreateChatDialog = true">
                            <svg width="35" height="35" viewBox="0 0 35 35" fill="none"
                                xmlns="http://www.w3.org/2000/svg">
                                <path
                                    d="M27.7082 20.4166C28.0949 20.4166 28.4659 20.5703 28.7394 20.8438C29.0129 21.1173 29.1665 21.4882 29.1665 21.875V26.25H33.5415C33.9283 26.25 34.2992 26.4036 34.5727 26.6771C34.8462 26.9506 34.9998 27.3215 34.9998 27.7083C34.9998 28.0951 34.8462 28.466 34.5727 28.7395C34.2992 29.013 33.9283 29.1666 33.5415 29.1666H29.1665V33.5416C29.1665 33.9284 29.0129 34.2993 28.7394 34.5728C28.4659 34.8463 28.0949 35 27.7082 35C27.3214 35 26.9505 34.8463 26.677 34.5728C26.4035 34.2993 26.2498 33.9284 26.2498 33.5416V29.1666H21.8748C21.4881 29.1666 21.1171 29.013 20.8436 28.7395C20.5701 28.466 20.4165 28.0951 20.4165 27.7083C20.4165 27.3215 20.5701 26.9506 20.8436 26.6771C21.1171 26.4036 21.4881 26.25 21.8748 26.25H26.2498V21.875C26.2498 21.4882 26.4035 21.1173 26.677 20.8438C26.9505 20.5703 27.3214 20.4166 27.7082 20.4166Z"
                                    fill="white" />
                                <path
                                    d="M30.2751 18.3312C30.8584 18.7687 32.0689 18.5208 32.0834 17.7916V17.5C32.0842 15.2367 31.5582 13.0043 30.547 10.9794C29.5358 8.9546 28.0672 7.19291 26.2575 5.8338C24.4477 4.47469 22.3464 3.55546 20.12 3.14887C17.8935 2.74228 15.6029 2.85949 13.4296 3.49122C11.2563 4.12295 9.25981 5.25185 7.59822 6.7886C5.93663 8.32534 4.65552 10.2277 3.8563 12.3452C3.05707 14.4627 2.76167 16.7371 2.99346 18.9885C3.22525 21.2399 3.97788 23.4064 5.19178 25.3166C5.36678 25.5937 5.33761 25.9583 5.13344 26.2062L2.11469 29.6625C1.92972 29.873 1.80934 30.1324 1.76795 30.4096C1.72656 30.6868 1.76591 30.97 1.8813 31.2254C1.99669 31.4808 2.18323 31.6975 2.4186 31.8497C2.65396 32.0018 2.92818 32.0829 3.20844 32.0833H17.7918C18.5209 32.0687 18.7689 30.8583 18.3314 30.275C17.8583 29.6219 17.5749 28.8508 17.5125 28.0468C17.4501 27.2429 17.6111 26.4373 17.9778 25.7191C18.3444 25.0008 18.9024 24.3979 19.5902 23.9769C20.278 23.5559 21.0687 23.3332 21.8751 23.3333H22.6043C22.7977 23.3333 22.9831 23.2565 23.1199 23.1197C23.2566 22.983 23.3334 22.7975 23.3334 22.6041V21.875C23.3333 21.0686 23.556 20.2778 23.9771 19.59C24.3981 18.9023 25.001 18.3443 25.7192 17.9776C26.4374 17.6109 27.243 17.4499 28.047 17.5123C28.851 17.5747 29.6221 17.8581 30.2751 18.3312Z"
                                    fill="white" />
                            </svg>
                        </article>
                    </section>
                </article>
            </article>
        </section>
    </article>
</template>

<style lang="scss" scoped></style>
