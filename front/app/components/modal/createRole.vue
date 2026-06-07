<script setup lang="ts">
    const nameRole = ref()
    const settingRole = ref({
        color: "",
        adminstrator: false
    })
    const settingUser = useSettingUser()
    
</script>

<template>
    <section @click.stop="$emit('dropDialog')" class="w-screen z-20 h-screen top-0 flex-row left-0 bg-body-900/80 absolute z-3 flex justify-center items-center">
        <article @click.stop="" class="px-10 py-15 flex flex-col z-30 gap-20 bg-body-900 justify-start items-start rounded-xl border-3 border-white">
            <section class="text-center w-full">
                <h1 class="text-[35px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Create role' : 'Создать роль'}}</h1>
            </section>
            <form class="flex flex-col gap-15 justify-center items-center">
                <input v-model="nameRole" class="border-1 text-white border-white bg-inherit hover:bg-white/10 px-10 py-3 text-[20px] placeholder:text-white/30" :placeholder="settingUser.language == 'Englend' ? 'Name' : 'Название'"></input>
                <section class="w-full px-5 flex flex-row justify-between items-center">
                    <input v-model="settingRole.color" type="color">
                    <p class="text-[20px] text-white font-bold" :style="{color: settingRole.color}">#{{ nameRole }}</p>
                </section>
                <section class="flex flex-row gap-10 w-full">
                    <p class="text-[20px] text-white">{{settingUser.language == 'Englend' ? 'Administrator(\')s authority' : 'Права администратора'}}</p>
                    <input @click.stop="settingRole.adminstrator ? settingRole.adminstrator = false : settingRole.adminstrator = true" type="checkbox">
                </section>
                <button @click.stop="$emit('createRole', nameRole, settingRole)" class="border-1 text-white/50 hover:text-white px-5 py-2 text-[20px]  border-white w-1/2 ">{{settingUser.language == 'Englend' ? 'Create' : 'Создать'}}</button>
            </form>
        </article>
    </section>
</template>

<style lang="scss">
body {
    position: relative;
}
</style>