<script setup lang="ts">
import { messageManagment } from '~/composabels/messageManagment';
import { User as UserComposable } from '~/composabels/User';
import type { genericRef } from '~~/types/other';
import type { BaseMessage } from '~~/types/api/respons';

const searchData = ref('')
const activeSearch = ref(false)
const editMessageStatus = ref(false)
const rightSideBarType = ref('')

// Client-side full-text search over loaded messages (the server search indexer
// pipeline isn't running). Highlights matching message bubbles.
const matchedIds = computed<Set<string>>(() => {
    const q = (searchData.value || '').trim().toLowerCase()
    if (!q) return new Set()
    const ids = messageUser.value
        .filter((m: any) => String(m.data ?? '').toLowerCase().includes(q))
        .map((m: any) => m.id)
    return new Set(ids)
})
const messageClass = new messageManagment()
const userClass = new UserComposable()
const user = useUserStore()
const props = defineProps(["Room"])
// Room prop is the [type, roomObject] tuple emitted by the sidebar.
const roomId = computed<string>(() => (props.Room?.[1]?.id ?? '') as string)

const messageUser = ref<any[]>([])
const userMap = ref<Record<string, string>>({})

// Resolve user_id → username so messages show the sender's name.
async function ensureUsers() {
    if (Object.keys(userMap.value).length) return
    const all = await userClass.getAllUser()
    const map: Record<string, string> = {}
    for (const u of all) map[String(u.id)] = u.username
    userMap.value = map
}

function mapMessage(m: BaseMessage) {
    const isMine = String(m.user_id) === String(user.userData?.id)
    return {
        id: m.id,
        userId: String(m.user_id),
        name: isMine ? (user.userData?.userName ?? 'Вы') : (userMap.value[String(m.user_id)] ?? String(m.user_id)),
        data: (m.payload as any)?.text ?? '',
        time: new Date(Math.floor(m.timestamp / 1e6)).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        srcImg: '',
    }
}

async function loadMessages() {
    if (!roomId.value) return
    await ensureUsers()
    const raw = await messageClass.getUserChatMessage(roomId.value)
    messageUser.value = (raw ?? []).map(mapMessage)
}

onMounted(loadMessages)
watch(roomId, () => {
    messageUser.value = []
    loadMessages()
})

// Light polling so the receiver sees incoming messages without a reload.
let pollTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => { pollTimer = setInterval(loadMessages, 4000) })
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

const messageData: genericRef<string> = ref('')
const idMessageEdit = ref('')

const funcMessage = async () => {
    const text = messageData.value ? messageData.value : ''
    if (!text || !roomId.value) return
    try {
        if (editMessageStatus.value) {
            await messageClass.editMessage(idMessageEdit.value, text, roomId.value)
            idMessageEdit.value = ''
            editMessageStatus.value = false
        } else {
            await messageClass.createNewMessage(text, roomId.value)
        }
        messageData.value = ''
        await loadMessages()
    } catch (e) {
        console.error("[chat] send failed", e)
    }
}


function editMessage(dataMessage: any) {
    editMessageStatus.value = true
    idMessageEdit.value = dataMessage.id
    messageData.value = dataMessage.data
}

</script>

<template>
    <section class="w-full border-l-6 border-body-100 h-screen rounded-tl-2xl box-border flex flex-col">
        <article class="grid grid-cols-10 w-full ">
            <section
                class="flex justify-center col-start-1 bg-body-500 col-span-9 gap-5 jsutify-center items-center border-r-5 border-body-500 rounded-tr-2xl">
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
                        <path d="M9.33536 41.5L9.33536 1.5M1.50031 13.5635L9.33536 1.5L17.1704 13.5635" stroke="white"
                            stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
                    </svg>
                </article>
            </section>
            <section class=" col-span-1 w-full bg-body-900 flex flex-row justify-end items-center px-5">
                <svg width="30" height="30" viewBox="0 0 43 43" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path
                        d="M14.463 1.5H31.5C37.0228 1.5 41.5 5.97715 41.5 11.5V31.5C41.5 37.0228 37.0229 41.5 31.5 41.5H14.463M14.463 1.5H11.5C5.97716 1.5 1.5 5.97715 1.5 11.5V31.5C1.5 37.0228 5.97715 41.5 11.5 41.5H14.463M14.463 1.5V41.5"
                        stroke="white" stroke-width="3" />
                </svg>
                <svg @click.stop="rightSideBarType === 'user' ? rightSideBarType = '' : rightSideBarType = 'user'" class="cursor-pointer" width="30" height="30" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" clip-rule="evenodd"
                        d="M16.6665 6.66659C16.6665 7.55064 17.0177 8.39849 17.6428 9.02361C18.2679 9.64873 19.1158 9.99992 19.9998 9.99992C20.8839 9.99992 21.7317 9.64873 22.3569 9.02361C22.982 8.39849 23.3332 7.55064 23.3332 6.66659C23.3332 5.78253 22.982 4.93468 22.3569 4.30956C21.7317 3.68444 20.8839 3.33325 19.9998 3.33325C19.1158 3.33325 18.2679 3.68444 17.6428 4.30956C17.0177 4.93468 16.6665 5.78253 16.6665 6.66659ZM19.9998 23.3333C19.1158 23.3333 18.2679 22.9821 17.6428 22.3569C17.0177 21.7318 16.6665 20.884 16.6665 19.9999C16.6665 19.1159 17.0177 18.268 17.6428 17.6429C18.2679 17.0178 19.1158 16.6666 19.9998 16.6666C20.8839 16.6666 21.7317 17.0178 22.3569 17.6429C22.982 18.268 23.3332 19.1159 23.3332 19.9999C23.3332 20.884 22.982 21.7318 22.3569 22.3569C21.7317 22.9821 20.8839 23.3333 19.9998 23.3333ZM19.9998 36.6666C19.1158 36.6666 18.2679 36.3154 17.6428 35.6903C17.0177 35.0652 16.6665 34.2173 16.6665 33.3333C16.6665 32.4492 17.0177 31.6014 17.6428 30.9762C18.2679 30.3511 19.1158 29.9999 19.9998 29.9999C20.8839 29.9999 21.7317 30.3511 22.3569 30.9762C22.982 31.6014 23.3332 32.4492 23.3332 33.3333C23.3332 34.2173 22.982 35.0652 22.3569 35.6903C21.7317 36.3154 20.8839 36.6666 19.9998 36.6666Z"
                        fill="white" />
                </svg>
            </section>
        </article>
        <article class="grid grid-flow-col grid-cols-4 w-full h-full bg-body-500">
            <section
                :class='{ "w-full h-full col-span-4 flex flex-col pb-10 justify-between relative scrollbar-hide scroll-smooth overflow-y-auto": !Boolean(rightSideBarType), "w-full h-full col-span-3 flex pb-10 flex-col justify-between relative scrollbar-hide scroll-smooth overflow-y-auto": Boolean(rightSideBarType) }'>
                <article
                    class="h-fit px-30 py-15 flex pb-50 w-full flex-col gap-10 scrollbar-hide scroll-smooth overflow-y-auto">
                    <MessageUserChat @deleteMessage="(idMessage: string) => {messageClass.deleteMessage(idMessage, roomId)}" @editMessage="editMessage" :message-data="el" :highlight="matchedIds.has(el.id)" v-for="el in messageUser" />
                </article>
                <article class="left-[50%] bottom-0 pb-10 fixed">
                    <section class="flex felx-row w-fit bg-body-100 gap-5 px-4 py-2 items-center rounded-4xl">
                        <article v-if="false"
                            class="flex felx-row justify-center gap-2 px-2 py-1 rounded-xl items-center bg-body-900">
                            <svg width="40" height="40" viewBox="0 0 40 40" fill="none"
                                xmlns="http://www.w3.org/2000/svg">
                                <path
                                    d="M30.1159 22.9941L22.5616 30.5485C18.9413 34.1687 13.0717 34.1687 9.45142 30.5485C5.83116 26.9282 5.83116 21.0586 9.45143 17.4383L19.6201 7.26963C22.0355 4.85428 25.9515 4.85428 28.3669 7.26962C30.7822 9.68497 30.7822 13.601 28.3669 16.0164L17.8778 26.5054C16.743 27.6403 14.9031 27.6403 13.7683 26.5054C12.6335 25.3706 12.6335 23.5307 13.7683 22.3959L21.643 14.5212C22.0429 14.1213 22.0429 13.473 21.643 13.073C21.2431 12.6731 20.5947 12.6731 20.1948 13.073L12.3201 20.9477C10.3855 22.8823 10.3855 26.019 12.3201 27.9536C14.2547 29.8882 17.3914 29.8882 19.326 27.9536L29.8151 17.4645C33.0302 14.2494 33.0302 9.0366 29.8151 5.82145C26.5999 2.60629 21.3871 2.60629 18.172 5.82145L8.00325 15.9902C3.58317 20.4102 3.58317 27.5766 8.00324 31.9967C12.4233 36.4167 19.5897 36.4167 24.0097 31.9966L31.5641 24.4423C31.964 24.0424 31.964 23.3941 31.5641 22.9941C31.1642 22.5942 30.5158 22.5942 30.1159 22.9941Z"
                                    fill="white" />
                            </svg>
                            <svg width="40" height="40" viewBox="0 0 70 70" fill="none"
                                xmlns="http://www.w3.org/2000/svg">
                                <path fill-rule="evenodd" clip-rule="evenodd"
                                    d="M20.9998 11.6667H48.9998C51.4752 11.6667 53.8492 12.6501 55.5995 14.4004C57.3498 16.1508 58.3332 18.5247 58.3332 21.0001V38.5001C58.3332 38.8095 58.2103 39.1062 57.9915 39.325C57.7727 39.5438 57.4759 39.6667 57.1665 39.6667H51.3332C48.239 39.6667 45.2715 40.8959 43.0836 43.0838C40.8957 45.2718 39.6665 48.2392 39.6665 51.3334V57.1667C39.6665 57.4762 39.5436 57.7729 39.3248 57.9917C39.106 58.2105 38.8093 58.3334 38.4998 58.3334H20.9998C18.5245 58.3334 16.1505 57.3501 14.4002 55.5997C12.6498 53.8494 11.6665 51.4754 11.6665 49.0001V21.0001C11.6665 18.5247 12.6498 16.1508 14.4002 14.4004C16.1505 12.6501 18.5245 11.6667 20.9998 11.6667ZM22.1665 30.3334C23.0948 30.3334 23.985 29.9647 24.6414 29.3083C25.2978 28.6519 25.6665 27.7617 25.6665 26.8334C25.6665 25.9052 25.2978 25.0149 24.6414 24.3585C23.985 23.7022 23.0948 23.3334 22.1665 23.3334C21.2382 23.3334 20.348 23.7022 19.6916 24.3585C19.0353 25.0149 18.6665 25.9052 18.6665 26.8334C18.6665 27.7617 19.0353 28.6519 19.6916 29.3083C20.348 29.9647 21.2382 30.3334 22.1665 30.3334ZM51.3332 26.8334C51.3332 27.7617 50.9644 28.6519 50.308 29.3083C49.6517 29.9647 48.7614 30.3334 47.8332 30.3334C46.9049 30.3334 46.0147 29.9647 45.3583 29.3083C44.7019 28.6519 44.3332 27.7617 44.3332 26.8334C44.3332 25.9052 44.7019 25.0149 45.3583 24.3585C46.0147 23.7022 46.9049 23.3334 47.8332 23.3334C48.7614 23.3334 49.6517 23.7022 50.308 24.3585C50.9644 25.0149 51.3332 25.9052 51.3332 26.8334ZM28.2098 33.6934C27.8633 33.1798 27.3269 32.8248 26.7186 32.7067C26.1104 32.5885 25.4801 32.7169 24.9665 33.0634C24.4529 33.41 24.0979 33.9464 23.9798 34.5546C23.8616 35.1628 23.99 35.7931 24.3365 36.3067C25.5096 38.0585 27.0961 39.4943 28.956 40.4872C30.8158 41.4801 32.8916 41.9995 34.9998 41.9995C37.1081 41.9995 39.1839 41.4801 41.0437 40.4872C42.9035 39.4943 44.4901 38.0585 45.6632 36.3067C45.8348 36.0524 45.9546 35.7668 46.0158 35.4662C46.077 35.1655 46.0784 34.8558 46.0199 34.5546C45.9614 34.2534 45.8441 33.9667 45.6748 33.7109C45.5055 33.455 45.2875 33.235 45.0332 33.0634C44.7788 32.8918 44.4932 32.772 44.1926 32.7108C43.8919 32.6496 43.5822 32.6482 43.281 32.7067C42.9799 32.7652 42.6932 32.8824 42.4373 33.0517C42.1814 33.221 41.9614 33.4391 41.7898 33.6934C41.0437 34.8099 40.0337 35.7253 38.8493 36.3583C37.6649 36.9913 36.3428 37.3224 34.9998 37.3224C33.6569 37.3224 32.3347 36.9913 31.1504 36.3583C29.966 35.7253 28.956 34.8099 28.2098 33.6934Z"
                                    fill="white" />
                                <path
                                    d="M57.5401 44.3335C57.6101 44.3335 57.6568 44.4035 57.6334 44.4735C57.2931 45.1828 56.8355 45.8296 56.2801 46.3868L46.3868 56.2802C45.8296 56.8356 45.1827 57.2931 44.4734 57.6335C44.4583 57.6425 44.441 57.647 44.4234 57.6465C44.4058 57.646 44.3887 57.6406 44.3741 57.6309C44.3595 57.6211 44.3479 57.6074 44.3407 57.5914C44.3335 57.5753 44.331 57.5576 44.3334 57.5402V51.3335C44.3334 49.477 45.0709 47.6965 46.3837 46.3837C47.6964 45.071 49.4769 44.3335 51.3334 44.3335H57.5401Z"
                                    fill="white" />
                            </svg>
                        </article>
                        <textarea type="text" placeholder="Message@here" v-model="messageData" autocomplete="off"
                            class="w-100 h-15 box-border scrollbar-hide scroll-smooth resize-none p-4 overflow-y-auto flex text-white text-[20px] placeholder:text-white placeholder:text-[20px] focus:outline-none"></textarea>
                        <svg @click.stop="funcMessage" width="39" height="41" viewBox="0 0 59 61" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M0.0078125 1.12415C-0.0902225 0.326385 0.744903 -0.255541 1.45898 0.113403L58.0566 29.3556C58.7779 29.7283 58.7779 30.7603 58.0566 31.1329L1.45898 60.3751C0.744937 60.7439 -0.0902032 60.1621 0.0078125 59.3644L3.01465 34.8898C3.07036 34.4365 3.42692 34.0782 3.87988 34.0197L33.1094 30.2443L3.87988 26.4689C3.42694 26.4104 3.07037 26.052 3.01465 25.5988L0.0078125 1.12415Z"
                                fill="white" />
                        </svg>
                    </section>
                </article>
            </section>
            <section :class='{ "hidden": !Boolean(rightSideBarType), "col-span-1 w-full h-full bg-body-500 border-l-4 border-body-100 overflow-y-auto scrollbar-hide": Boolean(rightSideBarType) }'>
                <SideBarRightUsers v-if="rightSideBarType === 'user'" :RoomData="props.Room[1]" />
            </section>
        </article>
    </section>
</template>

<style></style>