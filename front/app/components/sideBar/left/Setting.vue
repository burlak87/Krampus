<script lang="ts" setup>
import type { genericRef, Setting, User } from "../../../../types/other"
import { Auth } from "~/composabels/Auth"
import { apiClient } from "~/composabels/apiClient"

const router = useRouter()
const logoutPassword = ref("")
const logoutError = ref("")
const avatarInput = ref<HTMLInputElement | null>(null)

function pickAvatar() {
    avatarInput.value?.click()
}

async function onAvatarPick(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    const dataUrl: string = await new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result))
        reader.onerror = reject
        reader.readAsDataURL(file)
    })
    try {
        await apiClient.uploadAvatar(dataUrl)
        const store = useUserStore()
        if (store.userData) store.userData.logo = dataUrl
        if (userStor.value) userStor.value.logo = dataUrl
    } catch (err) {
        console.error("[avatar] upload failed", err)
    }
}

async function logout() {
    const auth = new Auth()
    const res = await auth.logout(logoutPassword.value)
    if (res.sucess) {
        logoutError.value = ""
        await router.push("/auth")
    } else {
        logoutError.value = res.error ?? "Ошибка выхода"
    }
}

const userStor: genericRef<User> = ref(useUserStore().userData)
const settingStore: genericRef<Setting> = ref(useUserStore().settingUser)
const settingUser = useSettingUser()
const newDataUser: genericRef<User> = ref({
    id: "0",
    name: "",
    secondName: "",
    userName: "",
    phone: "",
    birthday: "",
    country: "",
    bio: "",
    logo: "",
    email: "",
    password: ""
})

function openDropOption(el: HTMLElement) {
    if (el.parentElement?.parentElement?.children[1]?.classList.contains("hidden")) {
        el.parentElement?.parentElement?.children[1]?.classList.remove("hidden")
        el.classList.add("rotate-90")
    } else {
        el.parentElement?.parentElement?.children[1]?.classList.add("hidden")
        el.classList.remove("rotate-90")
    }
}

onUpdated(() => {
    userStor.value = useUserStore().userData
    settingStore.value = useUserStore().settingUser

    console.log(userStor.value?.name)
})


</script>

<template>
    <article class="flex flex-col gap-10 px-5 py-5 scrollbar-hide scroll-smooth overflow-y-auto">
        <section class="w-full text-center">
            <h1 class="text-[22px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Setting' : 'Настройки'}}</h1>
        </section>
        <section class="flex flex-col py-4 gap-5 px-7 bg-body-100 rounded-xl">
            <article class="flex flex-row justify-start items-center gap-5">
                <section class="w-10 h-fit">
                    <img @click.stop="pickAvatar" class="w-10 h-10 object-cover rounded-full cursor-pointer bg-white/10" :src="userStor?.logo ? userStor?.logo : ''" alt="logoUser" :title="settingUser.language == 'Englend' ? 'Change avatar' : 'Сменить аватар'">
                    <input ref="avatarInput" type="file" accept="image/*" class="hidden" @change="onAvatarPick">
                </section>
                <section class="flex flex-col gap-3">
                    <h2 class=" text-white text-[20px]">{{ userStor?.userName }}</h2>
                    <p class="text-white text-[16px] text-white/50">{{ userStor?.bio }}</p>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row justify-between items-center">
                    <article class="flex flex-row gap-10 items-center">
                        <section class="flex flex-row items-center gap-5">
                            <svg width="24" height="24" viewBox="0 0 24 24" fill="none"
                                xmlns="http://www.w3.org/2000/svg">
                                <path
                                    d="M8.28223 3.60742C9.67236 1.77036 11.9366 0.783493 14.2461 1.04004L14.3164 1.04785C14.9689 1.12038 15.6075 1.28924 16.2109 1.54785L17.0811 1.9209C18.3871 2.48079 19.5113 3.39446 20.3262 4.55859C21.9223 6.83917 22.0007 9.85332 20.5254 12.2139L20.4941 12.2627C20.6903 12.3928 20.8202 12.6153 20.8203 12.8682V13.8115C20.8202 15.155 20.0078 16.3436 18.7988 16.8516C18.5548 20.0141 15.9176 22.5418 12.6855 22.542H11.5547C8.32254 22.5419 5.68444 20.0142 5.44043 16.8516C4.23191 16.3434 3.42003 15.1547 3.41992 13.8115V12.8662C3.41992 12.8286 3.42514 12.7919 3.43066 12.7559C3.3066 12.5847 3.20364 12.4038 3.12012 12.2275C2.87091 11.7014 2.71679 11.085 2.61816 10.5264C2.51838 9.96092 2.46976 9.41675 2.44531 9.01758C2.43304 8.81715 2.42597 8.65044 2.42285 8.5332C2.4213 8.47469 2.4213 8.42798 2.4209 8.39551C2.4207 8.37941 2.41999 8.36644 2.41992 8.35742V8.3418C2.41992 5.51368 4.84854 3.29571 7.66504 3.55176L8.28223 3.60742ZM9.79102 7.92188C8.2741 7.37027 6.63768 8.418 6.52441 10.002L6.42383 11.4199C6.35506 12.3809 5.69776 13.1667 4.82031 13.4385V13.8115C4.82043 14.6834 5.41387 15.4438 6.25977 15.6553C6.59282 15.7388 6.82023 16.0378 6.82031 16.373C6.82043 18.9935 8.94556 21.1415 11.5547 21.1416H12.6855C15.2946 21.1414 17.4198 18.9934 17.4199 16.373C17.42 16.0377 17.6473 15.7387 17.9805 15.6553L18.1357 15.6094C18.8981 15.3485 19.4198 14.6291 19.4199 13.8115V13.4453C18.5355 13.1912 17.8454 12.45 17.6875 11.5039L17.5469 10.6621C17.4894 10.3177 17.345 9.9934 17.127 9.7207L16.0615 8.38965L16.0586 8.39355C15.3692 9.08293 14.5468 9.62518 13.6416 9.9873L12.4414 10.4668C12.0151 10.6373 11.5363 10.3926 11.4248 9.94727L11.3477 9.6377C11.1509 8.85076 10.5606 8.2019 9.79102 7.92188ZM9.9668 11.6895C10.6379 11.5098 11.3146 11.9057 11.5215 12.6777C11.7626 13.5781 11.2034 14.5038 10.2305 15.5762C10.1781 15.6331 10.1104 15.6943 10.0557 15.709C10.0031 15.723 9.91256 15.7046 9.83887 15.6816C8.45981 15.2394 7.51281 14.7169 7.27148 13.8164C7.06458 13.0442 7.4548 12.3625 8.12598 12.1826C8.54239 12.0712 8.92058 12.2199 9.2002 12.5146C9.29657 12.1169 9.54818 11.8016 9.9668 11.6895ZM12.5684 12.6777C12.7753 11.9056 13.4548 11.5106 14.126 11.6904C14.5421 11.8022 14.7948 12.1198 14.8896 12.5146C15.172 12.2183 15.5481 12.0714 15.9668 12.1836C16.6377 12.3636 17.0252 13.0444 16.8184 13.8164C16.5771 14.7169 15.6301 15.2393 14.251 15.6816C14.1773 15.7046 14.0888 15.7236 14.0342 15.709C13.9816 15.6949 13.9128 15.633 13.8604 15.5762C12.8872 14.5036 12.3271 13.5782 12.5684 12.6777ZM14.0918 2.43164C12.1727 2.2184 10.2906 3.09732 9.21875 4.70508C9.06662 4.93315 8.80168 5.06088 8.52734 5.03613L7.53809 4.94629C5.54152 4.76489 3.82031 6.33698 3.82031 8.3418V8.37793C3.82064 8.40428 3.82187 8.44456 3.82324 8.49609C3.82599 8.5995 3.83066 8.74964 3.8418 8.93164C3.86423 9.29792 3.90946 9.78563 3.99707 10.2822C4.08595 10.7858 4.21321 11.2637 4.38574 11.6279C4.46586 11.797 4.54499 11.9164 4.61621 12.001C4.84567 11.8553 5.00652 11.6098 5.02734 11.3203L5.12793 9.90234C5.3092 7.36512 7.90347 5.74508 10.2695 6.60547C11.3392 6.99455 12.1894 7.84051 12.582 8.90234L13.1211 8.6875C13.8503 8.39579 14.513 7.95868 15.0684 7.40332L15.625 6.84668C15.7658 6.70589 15.9604 6.63157 16.1592 6.64258C16.3578 6.65373 16.5427 6.74896 16.667 6.9043L18.2197 8.8457C18.5873 9.30522 18.831 9.85219 18.9277 10.4326L19.0684 11.2734C19.0921 11.4157 19.1448 11.5468 19.2188 11.6621L19.3389 11.4717C20.5164 9.58746 20.4528 7.18165 19.1787 5.36133C18.5133 4.41071 17.5958 3.66418 16.5293 3.20703L15.6592 2.83398C15.1824 2.62966 14.6777 2.4968 14.1621 2.43945L14.0918 2.43164Z"
                                    fill="white" />
                            </svg>
                            <p class="text-white text-[20px]">{{settingUser.language == 'Englend' ? 'Info' : 'Информация'}}</p>
                        </section>
                        <svg @click.stop='' width="22" height="24" viewBox="0 0 22 24" fill="none"
                            xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M9.90332 12.7578C10.6597 12.8346 11.25 13.4733 11.25 14.25V18.25L11.2422 18.4033C11.1705 19.1093 10.6093 19.6705 9.90332 19.7422L9.75 19.75H5.75C4.97334 19.75 4.33461 19.1597 4.25781 18.4033L4.25 18.25V14.25C4.25 13.4216 4.92157 12.75 5.75 12.75H9.75L9.90332 12.7578ZM13.75 17.75C14.3023 17.75 14.75 18.1977 14.75 18.75C14.75 19.3023 14.3023 19.75 13.75 19.75C13.1977 19.75 12.75 19.3023 12.75 18.75C12.75 18.1977 13.1977 17.75 13.75 17.75ZM18.75 17.75C19.3023 17.75 19.75 18.1977 19.75 18.75C19.75 19.3023 19.3023 19.75 18.75 19.75C18.1977 19.75 17.75 19.3023 17.75 18.75C17.75 18.1977 18.1977 17.75 18.75 17.75ZM5.90039 18.0996H9.59961V14.4004H5.90039V18.0996ZM7.75 15.25C8.30228 15.25 8.75 15.6977 8.75 16.25C8.75 16.8023 8.30228 17.25 7.75 17.25C7.19772 17.25 6.75 16.8023 6.75 16.25C6.75 15.6977 7.19772 15.25 7.75 15.25ZM16.25 15.25C16.8023 15.25 17.25 15.6977 17.25 16.25C17.25 16.8023 16.8023 17.25 16.25 17.25C15.6977 17.25 15.25 16.8023 15.25 16.25C15.25 15.6977 15.6977 15.25 16.25 15.25ZM18.9248 12.75C19.3804 12.75 19.75 13.1196 19.75 13.5752C19.7499 14.0307 19.3804 14.4004 18.9248 14.4004H13.5752C13.1196 14.4004 12.7501 14.0307 12.75 13.5752C12.75 13.1196 13.1196 12.75 13.5752 12.75H18.9248ZM9.90332 4.25781C10.6597 4.33461 11.25 4.97334 11.25 5.75V9.75L11.2422 9.90332C11.1705 10.6093 10.6093 11.1705 9.90332 11.2422L9.75 11.25H5.75C4.97334 11.25 4.33461 10.6597 4.25781 9.90332L4.25 9.75V5.75C4.25 4.92157 4.92157 4.25 5.75 4.25H9.75L9.90332 4.25781ZM18.4033 4.25781C19.1597 4.33461 19.75 4.97334 19.75 5.75V9.75L19.7422 9.90332C19.6705 10.6093 19.1093 11.1705 18.4033 11.2422L18.25 11.25H14.25C13.4733 11.25 12.8346 10.6597 12.7578 9.90332L12.75 9.75V5.75C12.75 4.92157 13.4216 4.25 14.25 4.25H18.25L18.4033 4.25781ZM14.4004 9.59961H18.0996V5.90039H14.4004V9.59961ZM5.91016 9.58984H9.58984V5.91016H5.91016V9.58984ZM7.75 6.75C8.30228 6.75 8.75 7.19772 8.75 7.75C8.75 8.30228 8.30228 8.75 7.75 8.75C7.19772 8.75 6.75 8.30228 6.75 7.75C6.75 7.19772 7.19772 6.75 7.75 6.75ZM16.25 6.75C16.8023 6.75 17.25 7.19772 17.25 7.75C17.25 8.30228 16.8023 8.75 16.25 8.75C15.6977 8.75 15.25 8.30228 15.25 7.75C15.25 7.19772 15.6977 6.75 16.25 6.75Z"
                                fill="white" />
                        </svg>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-5 hidden">
                    <input :placeholder="settingUser.language == 'Englend' ? 'New Name' : 'Новое имя'" type="text" v-model="(newDataUser as User).name"
                        class="w-full px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white" />
                    <input :placeholder="settingUser.language == 'Englend' ? 'New Secondname' : 'Новая фамилия'" type="text" v-model="(newDataUser as User).secondName"
                        class="w-full px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white" />
                    <input :placeholder="settingUser.language == 'Englend' ? 'New Username' : 'Новое имя пользователя'" type="text" v-model="(newDataUser as User).userName"
                        class="w-full px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white" />
                    <input :placeholder="settingUser.language == 'Englend' ? 'New Phone' : 'Новый телефон'" type="text" v-model="(newDataUser as User).phone"
                        class="w-full px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white" />
                    <input :placeholder="settingUser.language == 'Englend' ? 'New Birthday' : 'Новая дата рождения'" type="text" v-model="(newDataUser as User).birthday"
                        class="w-full px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white" />
                    <input :placeholder="settingUser.language == 'Englend' ? 'New Country' : 'Новая страна'" type="text" v-model="(newDataUser as User).country"
                        class="w-full px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white" />
                    <textarea :placeholder="settingUser.language == 'Englend' ? 'New Bio' : 'Новое био'" v-model="(newDataUser as User).bio"
                        class="scrollbar-hide scroll-smooth w-full h-15 px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white"></textarea>
                    <article class="flex flex-col gap-5">
                        <section class="flex flex-row justify-between w-full items-center">
                            <p class="text-[18px] text-white">{{settingUser.language == 'Englend' ? 'Chat Backup' : 'Сохранение чата'}}</p>
                            <article class="flex flex-row gap-5 items-center">
                                <p class="text-[18px] text-white">{{ settingStore?.chatBackup }}</p>
                                <svg width="24" height="24" viewBox="0 0 24 24" fill="none"
                                    xmlns="http://www.w3.org/2000/svg" @click.stop='
                                        ($event) => {
                                            const el = ($event.currentTarget as HTMLElement)

                                            if (el?.parentElement?.parentElement?.parentElement?.children[1]?.classList.contains("hidden")) {
                                                el?.parentElement?.parentElement?.parentElement?.children[1]?.classList.remove("hidden")
                                                el.classList.add("rotate-90")
                                            } else {
                                                el?.parentElement?.parentElement?.parentElement?.children[1]?.classList.add("hidden")
                                                el.classList.remove("rotate-90")
                                            }
                                        }
                                    '>
                                    <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8"
                                        stroke-linecap="round" stroke-linejoin="round" />
                                </svg>
                            </article>
                        </section>
                        <form class="flex flex-col gap-5 pl-5 hidden">
                            <section name="chatBackup" class="flex flex-row gap-5 items-center">
                                <input name="ChatBackup" type="radio" class="w-4 h-4 bg-inherit border-1 border-white"
                                    @click="" value="on">
                                <label class="text-[18px] text-white font-bold">{{settingUser.language == 'Englend' ? 'On' : 'Включить'}}</label>
                            </section>
                            <section class="flex flex-row gap-5 items-center">
                                <input name="ChatBackup" type="radio" class="w-4 h-4 bg-inherit border-1 border-white"
                                    @click="" value="off">
                                <label class="text-[18px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Off' : 'Выключить'}}</label>
                            </section>
                        </form>
                    </article>
                </section>

            </article>
        </section>
        <section class="flex flex-col gap-5 bg-body-100 px-7 py-4 rounded-xl">
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M19 7.30029C19.9388 7.3004 20.7002 8.06167 20.7002 9.00049V19.0005C20.7001 19.9392 19.9387 20.7006 19 20.7007H5C4.06118 20.7007 3.29991 19.9393 3.2998 19.0005V9.00049C3.2998 8.0616 4.06112 7.30029 5 7.30029H19ZM5 8.70068C4.83431 8.70068 4.7002 8.8348 4.7002 9.00049V19.0005C4.7003 19.1661 4.83438 19.3003 5 19.3003H6.12207C6.19848 19.0821 6.30925 18.8674 6.45605 18.6606C6.75752 18.2361 7.19989 17.8508 7.75684 17.5259C8.31399 17.2009 8.97615 16.943 9.7041 16.7671C10.432 16.5912 11.2122 16.5005 12 16.5005C12.7879 16.5005 13.568 16.5912 14.2959 16.7671C15.0238 16.943 15.6851 17.2009 16.2422 17.5259C16.7992 17.8508 17.2414 18.2361 17.543 18.6606C17.6898 18.8674 17.8015 19.0821 17.8779 19.3003H19C19.1655 19.3002 19.2997 19.166 19.2998 19.0005V9.00049C19.2998 8.83487 19.1656 8.70079 19 8.70068H5ZM12 9.25049C13.5187 9.2506 14.75 10.5937 14.75 12.2505C14.75 13.9073 13.5187 15.2504 12 15.2505C10.4812 15.2505 9.25 13.9073 9.25 12.2505C9.25 10.5936 10.4812 9.25049 12 9.25049ZM17.8799 4.50049C18.4984 4.50052 19 5.00207 19 5.62061C18.9999 5.77519 18.8743 5.90088 18.7197 5.90088H5.28027C5.12567 5.90088 5.00006 5.77519 5 5.62061C5 5.00205 5.50156 4.50049 6.12012 4.50049H17.8799ZM16.3799 2.00049C16.9984 2.00052 17.5 2.50207 17.5 3.12061C17.4999 3.27519 17.3743 3.40088 17.2197 3.40088H6.78027C6.62567 3.40088 6.50006 3.27519 6.5 3.12061C6.5 2.50205 7.00156 2.00049 7.62012 2.00049H16.3799Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'List' : 'Список'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Archived Chats' : 'Архивные чаты'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Chat Sorting' : 'Сортировка чатов'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Unread Messages' : 'Непрочитанные сообщения'}}</p>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <g clip-path="url(#clip0_120_5799)">
                                <path
                                    d="M19 2.7998C20.2149 2.79991 21.2002 3.78504 21.2002 5V17.5C21.2001 18.7149 20.2149 19.7001 19 19.7002C17.9632 19.7002 17.096 18.982 16.8633 18.0166C15.2283 16.9107 13.0966 16.321 11.29 16.0156C10.9853 15.9641 10.6916 15.9218 10.4141 15.8857L10.9746 18.4111C11.1666 19.2754 10.7459 20.1597 9.9541 20.5557C8.97594 21.0446 7.78753 20.6234 7.33496 19.6279L5.53418 15.6689C3.71067 15.4398 2.29987 13.8856 2.2998 12V10.5C2.2998 8.45655 3.95655 6.7998 6 6.7998H8.05762C8.09833 6.79922 8.16008 6.79832 8.24023 6.7959C8.40132 6.79104 8.6379 6.78045 8.93359 6.76074C9.52638 6.72122 10.3547 6.64245 11.29 6.48438C13.0968 6.17897 15.2283 5.58848 16.8633 4.48242C17.0962 3.51729 17.9634 2.79982 19 2.7998ZM7.08789 15.7002L8.60938 19.0488C8.73363 19.3218 9.05976 19.4377 9.32812 19.3037C9.54532 19.195 9.66098 18.952 9.6084 18.7148L8.94727 15.7402C8.94269 15.7399 8.93814 15.7396 8.93359 15.7393C8.63792 15.7196 8.40132 15.7099 8.24023 15.7051C8.16008 15.7027 8.09833 15.7008 8.05762 15.7002H7.08789ZM19 4.2002C18.5582 4.20021 18.2002 4.55818 18.2002 5V17.5C18.2003 17.9417 18.5582 18.2998 19 18.2998C19.4417 18.2997 19.7997 17.9417 19.7998 17.5V5C19.7998 4.55823 19.4417 4.2003 19 4.2002ZM16.7998 6.15918C15.0877 7.0934 13.1265 7.59328 11.5234 7.86426C10.6199 8.01697 9.81206 8.09917 9.2002 8.14453V14.3555C9.81207 14.4007 10.6199 14.483 11.5234 14.6357C13.1263 14.9067 15.0878 15.4056 16.7998 16.3398V6.15918ZM6 8.2002C4.72975 8.2002 3.7002 9.22975 3.7002 10.5V12C3.70027 13.2702 4.72979 14.2998 6 14.2998H7.2998V8.2002H6Z"
                                    fill="white" />
                            </g>
                            <defs>
                                <clipPath id="clip0_120_5799">
                                    <rect width="24" height="24" fill="white" />
                                </clipPath>
                            </defs>
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Broadcast messages' : 'Рассылка'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'My Broadcasts' : 'Мои рассылки'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Privacy Settings' : 'Приватность'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Statistics' : 'Статистика'}}</p>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M11.1914 2.86719C11.5025 2.18983 12.4974 2.18985 12.8086 2.86719L12.8643 3.02344L14.4785 9.08008H21.125C21.984 9.08033 22.3483 10.1739 21.6611 10.6895L16.6807 14.4248L18.3564 20.707C18.5748 21.527 17.635 22.1614 16.9561 21.6523L12 17.9355L7.04395 21.6523C6.36501 22.1613 5.42525 21.5269 5.64355 20.707L7.31836 14.4238L2.33887 10.6895C1.6518 10.1739 2.01608 9.08041 2.875 9.08008H9.52148L11.1357 3.02344L11.1914 2.86719ZM10.7734 9.81641C10.6691 10.2078 10.3143 10.4805 9.90918 10.4805H4.39355L8.43262 13.5098C8.72426 13.7286 8.85351 14.1028 8.75977 14.4551L7.37305 19.6553L11.4639 16.5879L11.5879 16.5098C11.8888 16.3534 12.258 16.3795 12.5361 16.5879L16.627 19.6553L15.2402 14.4551C15.1465 14.1028 15.2758 13.7286 15.5674 13.5098L19.6064 10.4805H14.0908C13.6857 10.4805 13.3309 10.2078 13.2266 9.81641L12 5.21582L10.7734 9.81641Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Starred messages' : 'Избранные сообщения'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Starred Search' : 'Поиск по избранному'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Export Starred' : 'Экспорт избранных'}}</p>
                    <article class="flex flex-row justify-between items-center" @click.stop="">
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Auto-Cleanup' : 'Авто-очистка'}}</p>
                        <p class="text-[16px] text-white/50 ">1 {{settingUser.language == 'Englend' ? 'month' : 'месяц'}}</p>
                    </article>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M18.9004 5.79944C19.1687 5.79944 19.4125 5.79956 19.6143 5.81604C19.824 5.83321 20.0497 5.87206 20.2715 5.98499C20.5913 6.14792 20.8517 6.40842 21.0146 6.72815C21.1276 6.94985 21.1664 7.17565 21.1836 7.38538C21.2001 7.58705 21.2002 7.83108 21.2002 8.09924V17.5006H23C23.5523 17.5006 24 17.9483 24 18.5006C23.9999 19.0528 23.5522 19.5006 23 19.5006H1C0.447779 19.5006 0.000104097 19.0528 0 18.5006C0 17.9483 0.447715 17.5006 1 17.5006H2.7998V8.09924C2.7998 7.83108 2.79993 7.58705 2.81641 7.38538C2.8336 7.17565 2.87244 6.94985 2.98535 6.72815C3.14835 6.40842 3.40874 6.14792 3.72852 5.98499C3.95031 5.87206 4.17596 5.83321 4.38574 5.81604C4.58751 5.79956 4.8313 5.79944 5.09961 5.79944H18.9004ZM5.09961 7.19983C4.80829 7.19983 4.63157 7.19982 4.5 7.21057C4.40797 7.2181 4.37274 7.22893 4.36426 7.23206C4.30791 7.26076 4.2612 7.30758 4.23242 7.36389C4.22936 7.37214 4.2185 7.40743 4.21094 7.49963C4.20019 7.63115 4.2002 7.80813 4.2002 8.09924V17.5006H19.7998V8.09924C19.7998 7.80813 19.7998 7.63115 19.7891 7.49963C19.7815 7.40743 19.7706 7.37214 19.7676 7.36389C19.7388 7.30758 19.6921 7.26076 19.6357 7.23206C19.6273 7.22893 19.592 7.2181 19.5 7.21057C19.3684 7.19982 19.1917 7.19983 18.9004 7.19983H5.09961Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Linked devices' : 'Подключённые устройства'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Add Device' : 'Добавить устройство'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Active Sessions' : 'Активные устройства'}}</p>
                    <label @click.stop=""
                        class="flex flex-row justify-between w-full items-center text-[20px] text-white">
                        {{settingUser.language == 'Englend' ? 'Login Alerts' : 'Оповещение о входе'}}
                        <input class="bg-inherit w-4 h-4 border-1 border-white" type="radio">
                    </label>
                    <p class="text-[20px] text-red-500">{{settingUser.language == 'Englend' ? 'Terminate All' : 'Отключить все'}}</p>
                </section>
            </article>
        </section>
        <section class="flex flex-col gap-5 bg-body-100 px-7 py-4 rounded-xl">
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M12.001 1.6001C15.2043 1.60026 17.8018 4.19747 17.8018 7.40088C17.8016 9.86759 16.2612 11.9719 14.0918 12.811V13.1392L15.7412 14.7886C16.0144 15.0619 16.0145 15.5045 15.7412 15.7778L13.918 17.6011L15.2783 18.9614C15.5442 19.2277 15.552 19.6573 15.2959 19.9331L12.2822 23.1782C12.1474 23.3234 11.9569 23.4047 11.7588 23.4019C11.5608 23.3989 11.3727 23.3125 11.2422 23.1636L9.61914 21.3081C9.50768 21.1806 9.44634 21.0165 9.44629 20.8472V12.6089C7.52484 11.6647 6.20031 9.68837 6.2002 7.40088C6.2002 4.19737 8.79746 1.6001 12.001 1.6001ZM12.001 3.00049C9.57066 3.00049 7.60059 4.97056 7.60059 7.40088C7.60071 9.2653 8.76035 10.8608 10.4004 11.5015C10.6688 11.6064 10.8457 11.8656 10.8457 12.1538V20.5835L11.7842 21.6567L13.8105 19.4741L12.4336 18.0972C12.1602 17.8238 12.1602 17.3803 12.4336 17.1069L14.2568 15.2827L12.8965 13.9233C12.7655 13.7921 12.6924 13.6136 12.6924 13.4282V12.3091C12.6925 11.996 12.8999 11.7205 13.2012 11.6353C15.0484 11.1127 16.4012 9.41395 16.4014 7.40088C16.4014 4.97067 14.4311 3.00066 12.001 3.00049ZM12.001 4.61865C12.641 4.61882 13.1602 5.13773 13.1602 5.77783C13.1601 6.41787 12.641 6.93684 12.001 6.93701C11.3608 6.93701 10.8419 6.41797 10.8418 5.77783C10.8418 5.13762 11.3608 4.61865 12.001 4.61865Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Account' : 'Аккаунт'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Export Data' : 'Экспортировать данные'}}</p>
                    <p class="text-[20px] text-red-500" @click.stop="">{{settingUser.language == 'Englend' ? 'Delete Account' : 'Удалить аккаунт'}}</p>
                    <article class="flex flex-col gap-5">
                        <section class="flex flex-row justify-between w-full items-center">
                            <p class="text-[18px] text-white">{{settingUser.language == 'Englend' ? 'Language' : 'Язык'}}</p>
                            <article class="flex flex-row gap-5 items-center">
                                <p class="text-[18px] text-white">{{ settingUser.language }}</p>
                                <svg width="24" height="24" viewBox="0 0 24 24" fill="none"
                                    xmlns="http://www.w3.org/2000/svg" @click.stop='
                                        ($event) => {
                                            const el = ($event.currentTarget as HTMLElement)

                                            if (el?.parentElement?.parentElement?.parentElement?.children[1]?.classList.contains("hidden")) {
                                                el?.parentElement?.parentElement?.parentElement?.children[1]?.classList.remove("hidden")
                                                el.classList.add("rotate-90")
                                            } else {
                                                el?.parentElement?.parentElement?.parentElement?.children[1]?.classList.add("hidden")
                                                el.classList.remove("rotate-90")
                                            }
                                        }
                                    '>
                                    <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8"
                                        stroke-linecap="round" stroke-linejoin="round" />
                                </svg>
                            </article>
                        </section>
                        <form class="flex flex-col gap-5 pl-5 hidden">
                            <section name="chatBackup" class="flex flex-row gap-5 items-center">
                                <input name="ChatBackup" type="radio" class="w-4 h-4 bg-inherit border-1 border-white"
                                    @click="settingUser.language = 'Russian'" value="on">
                                <label class="text-[18px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Russian' : 'Русский'}}</label>
                            </section>
                            <section class="flex flex-row gap-5 items-center">
                                <input name="ChatBackup" type="radio" class="w-4 h-4 bg-inherit border-1 border-white"
                                    @click="settingUser.language = 'Englend'" value="off">
                                <label class="text-[18px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Englend' : 'Англиский'}}</label>
                            </section>
                        </form>
                    </article>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M12 3.7998C14.3195 3.79991 16.2002 5.68047 16.2002 8V10.3125C17.0446 10.4117 17.7002 11.129 17.7002 12V18.5C17.7001 19.4387 16.9387 20.2001 16 20.2002H8C7.06118 20.2002 6.29991 19.4388 6.2998 18.5V12C6.2998 11.1288 6.95521 10.4116 7.7998 10.3125V8C7.7998 5.6804 9.6804 3.7998 12 3.7998ZM8 11.7002C7.83431 11.7002 7.7002 11.8343 7.7002 12V18.5C7.7003 18.6656 7.83438 18.7998 8 18.7998H16C16.1655 18.7997 16.2997 18.6655 16.2998 18.5V12C16.2998 11.8344 16.1656 11.7003 16 11.7002H8ZM12 5.2002C10.4536 5.2002 9.2002 6.4536 9.2002 8V10.2998H14.7998V8C14.7998 6.45367 13.5463 5.2003 12 5.2002Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Safety' : 'Безопасность'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Blocked User' : 'Заблокированные пользователи'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Two-Step Verification' : 'Двойная аутентификация'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Passkeys' : 'Ключ доступа'}}</p>
                    <article class="flex flex-row justify-between items-center">
                        <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Delete My Account If Away For' : 'Удалить аккаунт через (неактивность)'}}</p>
                        <p class="text-[16px] text-white/50" @click.stop="">18 {{settingUser.language == 'Englend' ? 'month' : 'месяцев'}}</p>
                    </article>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M11.9707 3.30005C14.8205 3.30009 17.4386 4.10033 19.3623 5.5022C21.2929 6.90917 22.5409 8.94163 22.541 11.3479C22.5409 16.1652 17.6416 19.8029 11.9707 19.803C11.6313 19.803 11.2954 19.7889 10.9639 19.7639C9.71597 20.964 8.44552 21.5752 7.47266 21.884C6.98316 22.0394 6.56934 22.1183 6.27246 22.1584C6.12434 22.1785 6.00475 22.1892 5.91895 22.1946C5.8761 22.1973 5.84092 22.1987 5.81543 22.1995C5.80286 22.1998 5.79229 22.2003 5.78418 22.2004H5.7666C5.52139 22.2004 5.29371 22.0715 5.16699 21.8616C5.04051 21.6517 5.03328 21.3909 5.14746 21.1741L5.14844 21.1731C5.14901 21.172 5.15003 21.1698 5.15137 21.1672C5.15406 21.1621 5.15754 21.1536 5.16309 21.1428C5.17427 21.1212 5.19172 21.0882 5.21289 21.0461C5.25528 20.962 5.31507 20.84 5.38477 20.6926C5.52524 20.3956 5.70155 20.0009 5.85059 19.5989C6.0034 19.1866 6.11148 18.8096 6.1416 18.5354C6.14728 18.4835 6.14747 18.4403 6.14746 18.4055C5.88795 18.2685 5.63389 18.1256 5.39062 17.9709C3.00286 16.4528 1.40047 14.0776 1.40039 11.3479C1.40047 8.94154 2.64834 6.90918 4.5791 5.5022C6.50289 4.10032 9.12081 3.30005 11.9707 3.30005ZM11.9707 4.70044C9.36952 4.70044 7.05181 5.43277 5.40332 6.63403C3.76217 7.83012 2.80085 9.47158 2.80078 11.3479C2.80086 13.4905 4.05808 15.4641 6.14258 16.7893C6.39374 16.949 6.65639 17.0995 6.92969 17.2385C7.25937 17.4064 7.42457 17.6981 7.49512 17.9602C7.56173 18.2079 7.55759 18.4665 7.5332 18.6887C7.48396 19.1369 7.32738 19.643 7.16309 20.0862C7.10148 20.2523 7.03487 20.4154 6.96973 20.5715C6.99556 20.5637 7.02239 20.5575 7.04883 20.5491C7.85428 20.2934 8.94124 19.7775 10.0215 18.7297L10.125 18.6389C10.3418 18.4691 10.6091 18.3709 10.8877 18.3625L11.0273 18.3665L11.4961 18.3938C11.6532 18.4 11.8116 18.4036 11.9707 18.4036C17.2017 18.4035 21.1405 15.0963 21.1406 11.3479C21.1406 9.47166 20.1791 7.8301 18.5381 6.63403C16.8896 5.43278 14.5718 4.70048 11.9707 4.70044Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Chats' : 'Чаты'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <article class="flex flex-row justify-between items-center">
                        <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Change Wallpaper' : 'Сменить обои'}}</p>
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                            @click.stop=''>
                            <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                                stroke-linejoin="round" />
                        </svg>
                    </article>
                    <article class="flex flex-row justify-between items-center">
                        <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Chat Backup' : 'Резервное копирование'}}</p>
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                            @click.stop=''>
                            <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                                stroke-linejoin="round" />
                        </svg>
                    </article>
                    <form class="flex flex-col gap-4">
                        <label class="flex flex-row justify-between items-center text-[20px] text-white">
                            {{settingUser.language == 'Englend' ? 'Light Theme' : 'Светлая тема'}}
                            <input name="theme" type="radio" class="w-4 h-4">
                        </label>
                        <label class="flex flex-row justify-between items-center text-[20px] text-white">
                            {{settingUser.language == 'Englend' ? 'Dark Theme' : 'Тёмная тема'}}
                            <input name="theme" type="radio" class="w-4 h-4">
                        </label>
                    </form>
                    <p class="text-[20px] text-blue-500" @click.stop="">{{settingUser.language == 'Englend' ? 'Archive All Chats' : 'Архивировать все чаты'}}</p>
                    <p class="text-[20px] text-red-500" @click.stop="">{{settingUser.language == 'Englend' ? 'Clear All Chats' : 'Очистить все чаты'}}</p>
                    <p class="text-[20px] text-red-500" @click.stop="">{{settingUser.language == 'Englend' ? 'Delete All Chats' : 'Удалить все чаты'}}</p>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M14.0107 4.30029C13.8874 4.71526 13.8204 5.15485 13.8203 5.60986C13.8203 5.63993 13.8217 5.67076 13.8223 5.70068H7C6.28203 5.70068 5.7002 6.28252 5.7002 7.00049V17.0005C5.70052 17.7182 6.28223 18.3003 7 18.3003H17C17.7177 18.3002 18.2995 17.7181 18.2998 17.0005V10.1968C18.3364 10.1976 18.3734 10.1997 18.4102 10.1997C18.8582 10.1997 19.2908 10.1339 19.7002 10.0142V17.0005C19.6999 18.4913 18.4909 19.7006 17 19.7007H7C5.50903 19.7007 4.30012 18.4914 4.2998 17.0005V7.00049C4.2998 5.50932 5.50883 4.30029 7 4.30029H14.0107ZM18.4102 2.60986C20.0668 2.61013 21.4102 3.95317 21.4102 5.60986C21.4102 7.26655 20.0668 8.6096 18.4102 8.60986C16.7533 8.60986 15.4102 7.26672 15.4102 5.60986C15.4102 3.95301 16.7533 2.60986 18.4102 2.60986Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Notification' : 'Уведомления'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <form class="flex flex-col gap-4 pl-5 hidden">
                    <label class="flex flex-row justify-between items-center text-[20px] text-white">
                        {{settingUser.language == 'Englend' ? 'Notification for chat' : 'Уведомления чата'}}
                        <input name="Notification" type="radio" class="w-4 h-4">
                    </label>
                    <label class="flex flex-row justify-between items-center text-[20px] text-white">
                        {{settingUser.language == 'Englend' ? 'Sound' : 'Звук'}}
                        <input name="Sound" type="radio" class="w-4 h-4">
                    </label>
                </form>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M7.38379 4.56372C7.51608 4.58937 7.64268 4.65371 7.74512 4.7561L12.2451 9.2561C12.5183 9.52945 12.5183 9.97303 12.2451 10.2463C11.9718 10.5196 11.5283 10.5195 11.2549 10.2463L7.9502 6.94165V19.7512C7.95009 20.1377 7.63644 20.4513 7.25 20.4514C6.86347 20.4514 6.54991 20.1377 6.5498 19.7512V6.94165L3.24512 10.2463C2.97182 10.5196 2.52827 10.5195 2.25488 10.2463C1.98161 9.97296 1.98155 9.52944 2.25488 9.2561L6.75488 4.7561C6.85742 4.65361 6.98378 4.58932 7.11621 4.56372C7.2045 4.54666 7.29551 4.54661 7.38379 4.56372ZM16.75 4.55005C17.1364 4.55026 17.4501 4.86387 17.4502 5.25024V18.0598L20.7549 14.7551C21.0282 14.4818 21.4717 14.482 21.7451 14.7551C22.0181 15.0285 22.0184 15.4721 21.7451 15.7454L17.2451 20.2454C17.1426 20.3478 17.0161 20.4121 16.8838 20.4377C16.7956 20.4548 16.7044 20.4548 16.6162 20.4377C16.484 20.4121 16.3573 20.3476 16.2549 20.2454L11.7549 15.7454C11.4817 15.472 11.4816 15.0284 11.7549 14.7551C12.0282 14.4818 12.4717 14.482 12.7451 14.7551L16.0498 18.0598V5.25024C16.0499 4.8638 16.3636 4.55015 16.75 4.55005Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Storage and data' : 'Хранение и передача данных'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Cache and Files' : 'Кэш и файлы'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Usage Stats' : 'Статистика использования'}}</p>
                    <article class="flex flex-row justify-between items-center">
                        <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Auto-Cleanup' : 'Авто-очистка'}}</p>
                        <p class="text-[16px] text-white/50" @click.stop="">1 {{settingUser.language == 'Englend' ? 'month' : 'месяц'}}</p>
                    </article>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Backup' : 'Резервное копирование'}}</p>
                </section>
            </article>
        </section>
        <section class="flex flex-col gap-5 bg-body-100 px-7 py-4 rounded-xl"> 
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M12 3.7998C16.8048 3.79991 20.7002 7.69519 20.7002 12.5C20.7001 17.3047 16.8047 21.2001 12 21.2002C7.19519 21.2002 3.29991 17.3048 3.2998 12.5C3.2998 7.69512 7.19512 3.7998 12 3.7998ZM12 5.2002C7.96832 5.2002 4.7002 8.46832 4.7002 12.5C4.7003 16.5316 7.96839 19.7998 12 19.7998C16.0315 19.7997 19.2997 16.5315 19.2998 12.5C19.2998 8.46839 16.0316 5.2003 12 5.2002ZM12.4492 10.8008C12.8358 10.8008 13.1494 11.1144 13.1494 11.501V15.7998H14.3496C14.7361 15.7999 15.0498 16.1135 15.0498 16.5C15.0497 16.8864 14.7361 17.2001 14.3496 17.2002H12.459C12.4559 17.2003 12.4523 17.2012 12.4492 17.2012C12.4461 17.2011 12.4426 17.2002 12.4395 17.2002H10.3496C9.96324 17.2 9.64952 16.8864 9.64941 16.5C9.64941 16.1135 9.96317 15.8 10.3496 15.7998H11.749V12.2012H10.8994C10.5132 12.2008 10.1994 11.8872 10.1992 11.501C10.1992 11.1146 10.5131 10.8011 10.8994 10.8008H12.4492ZM12.0498 7C12.7402 7 13.2998 7.55964 13.2998 8.25C13.2998 8.94035 12.7402 9.5 12.0498 9.5C11.3595 9.5 10.7998 8.94035 10.7998 8.25C10.7998 7.55964 11.3594 7 12.0498 7Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Help' : 'Помощь'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <article class="flex flex-row justify-between items-center">
                        <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Cache and Files' : 'Кеш и файлы'}}</p>
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                            @click.stop=''>
                            <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                                stroke-linejoin="round" />
                        </svg>
                    </article>
                    <article class="flex flex-row justify-between items-center">
                        <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Contact Support' : 'Контакты для помощи'}}</p>
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                            @click.stop=''>
                            <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                                stroke-linejoin="round" />
                        </svg>
                    </article>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'App Version' : 'Версия приложения'}}</p>
                    <article class="flex flex-row justify-between items-center">
                        <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Report Bug' : 'Сообщить о ошибке'}}</p>
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                            @click.stop=''>
                            <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                                stroke-linejoin="round" />
                        </svg>
                    </article>
                </section>
            </article>
            <article class="flex flex-col gap-5">
                <section class="flex flex-row items-center justify-between">
                    <article class="flex flex-row gap-5 items-center">
                        <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M15.4229 3.89014C18.7703 3.89035 21.292 6.60241 21.292 10.1577C21.2919 12.26 20.4006 14.0803 19.0068 15.6938C17.6305 17.2871 15.7062 18.748 13.4854 20.1782L13.4766 20.1841C13.3241 20.2806 13.1285 20.3942 12.915 20.4878C12.7318 20.5681 12.3946 20.6986 12.001 20.6987C11.609 20.6987 11.2702 20.5665 11.0938 20.4897C10.8818 20.3975 10.6838 20.2844 10.5254 20.1841L10.5166 20.1782C8.29589 18.7481 6.3714 17.287 4.99512 15.6938C3.60138 14.0803 2.71007 12.26 2.70996 10.1577C2.70996 6.59898 5.24233 3.89014 8.58691 3.89014C9.9662 3.89023 11.116 4.40031 12 5.19678C12.8852 4.39789 14.0359 3.89014 15.4229 3.89014ZM15.4229 5.29053C13.8682 5.29053 12.7041 6.1499 12.001 7.43896C11.3057 6.15782 10.1336 5.29067 8.58691 5.29053C6.09473 5.29053 4.11035 7.29053 4.11035 10.1577C4.11057 13.5013 6.9074 16.1891 11.2744 19.0015C11.5087 19.1498 11.8058 19.2983 12.001 19.2983C12.2041 19.2982 12.4933 19.1498 12.7275 19.0015C17.0945 16.1891 19.8914 13.5013 19.8916 10.1577C19.8916 7.29068 17.9149 5.29074 15.4229 5.29053Z"
                                fill="white" />
                        </svg>
                        <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Invite a friend' : 'Пригласи друга'}}</p>
                    </article>
                    <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"
                        @click.stop='
                            openDropOption(($event.currentTarget as HTMLElement))
                            '>
                        <path d="M10.5 7.5L15 12L10.5 16.5" stroke="white" stroke-width="1.8" stroke-linecap="round"
                            stroke-linejoin="round" />
                    </svg>
                </section>
                <section class="flex flex-col gap-4 pl-5 hidden">
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Share Link' : 'Поделиться ссылкой'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Invite Contacts' : 'Пригласить контакт'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Referral Bonuses' : 'Реферальные бонусы'}}</p>
                    <p class="text-[20px] text-white" @click.stop="">{{settingUser.language == 'Englend' ? 'Invite History' : 'История приглашений'}}</p>
                </section>
            </article>
        </section>

        <!-- Logout -->
        <section class="flex flex-col gap-4 bg-body-100 px-7 py-4 rounded-xl">
            <article class="flex flex-row gap-5 items-center">
                <svg width="34" height="34" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M15 12H3"
                        stroke="#F23F42" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
                <p class="text-[20px] text-red-500 font-bold">{{settingUser.language == 'Englend' ? 'Logout' : 'Выйти'}}</p>
            </article>
            <input type="password" v-model="logoutPassword"
                :placeholder="settingUser.language == 'Englend' ? 'Password' : 'Пароль'"
                class="w-full px-5 py-2 text-white text-[18px] bg-body-900 rounded-full placeholder:text-white/40 focus:outline-none">
            <button type="button" @click.stop="logout"
                class="border-1 border-red-500 text-red-500 hover:bg-red-500/10 px-5 py-2 text-[18px] rounded-full">
                {{settingUser.language == 'Englend' ? 'Logout' : 'Выйти'}}
            </button>
            <p v-if="logoutError" class="text-red-400 text-[14px]">{{ logoutError }}</p>
        </section>
    </article>
</template>

<style lang="scss" scoped></style>