<script setup lang="ts">
    const nameGroup = ref()
    const settingUser = useSettingUser()
</script>

<template>
    <section @click.stop="$emit('dropDialog')" class="w-screen h-screen top-0 flex-row left-0 bg-body-900/80 absolute z-3 flex justify-center items-center">
        <article @click.stop="" class="px-10 py-12 flex flex-col z-10 gap-10 bg-body-900 justify-start items-center rounded-xl border-3 border-white">
            <section class="text-center w-full">
                <h1 class="text-[35px] text-white font-bold">{{settingUser.language == 'Englend' ? 'Create group' : 'Создать группу'}}</h1>
            </section>
            <form class="flex flex-col gap-8 justify-center items-center">
                <input v-model="nameGroup" class="border-1 text-white border-white bg-inherit hover:bg-white/10 px-10 py-3 text-[20px] placeholder:text-white/30" :placeholder="settingUser.language == 'Englend' ? 'Name' : 'Название'"></input>
                <button type="button" @click.prevent="$emit('createGroup', nameGroup)" class="border-1 text-white/50 hover:text-white px-5 py-2 text-[20px]  border-white w-1/2 ">{{settingUser.language == 'Englend' ? 'Create' : 'Создать'}}</button>
            </form>
        </article>
    </section>
</template>

<style lang="scss">
body {
    position: relative;
}
</style>