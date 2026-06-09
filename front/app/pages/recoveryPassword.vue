<script setup lang="ts">
    import { Auth } from '~/composabels/Auth';

    let emailUser = ref()
    let passwordUser = ref()
    const settingUser = useSettingUser()

    function dropPassword() {
        //const auth = new Auth()

        //auth.startAuth({email: emailUser.value, password: passwordUser.value})
    }

    
</script>


<template>
    <section class="flex flex-col gap-5 w-full h-fit justify-center items-center min-h-screen">
        <form class="flex flex-col gap-10 mb-5 px-5 py-5 rounded-md justify-center items-center">
            <input class="border-1 text-white border-white bg-inherit hover:bg-white/10 px-10 py-3 text-[20px] placeholder:text-white/10" type="email" :placeholder="settingUser.language == 'Englend' ? 'Email' : 'Почта'" required v-model="emailUser"></input>
            <button class="border-1 text-white/50 hover:text-white px-5 py-2 text-[20px]  border-white w-1/2 " @click.prevent="dropPassword()">{{settingUser.language == 'Englend' ? 'Send' : 'Отправить'}}</button>
        </form>
        <NuxtLink class="text-[20px] text-white/50 hover:text-white" to="auth">{{settingUser.language == 'Englend' ? 'Return!' : 'Вернуться!'}}</NuxtLink>
    </section>
</template>

<style scoped lang="scss">
   
    
</style>