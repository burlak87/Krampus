<script setup lang="ts">
    import { Auth } from '~/composabels/Auth';

    let emailUser = ref(),
        passwordUser = ref(),
        loginUser = ref()
    const settingUser = useSettingUser()

    async function register() {
        const auth = new Auth()


        const res = await auth.register(loginUser.value, emailUser.value, passwordUser.value)
        if (res.sucess) {
            navigateTo('auth')
        } else {
            console.error(res.error)
        }
    }
</script>


<template>
    <section class="flex flex-col gap-5 w-full h-fit justify-center items-center min-h-screen">
        <form class="flex flex-col gap-10 mb-5 px-5 py-5 rounded-md justify-center items-center">
            <input class="border-1 text-white border-white bg-inherit hover:bg-white/10 px-10 py-3 text-[20px] placeholder:text-white/10" type="text" :placeholder="settingUser.language == 'Englend' ? 'Login' : 'Логин'" required v-model="loginUser"></input>
            <input class="border-1 text-white border-white bg-inherit hover:bg-white/10 px-10 py-3 text-[20px] placeholder:text-white/10" type="email" :placeholder="settingUser.language == 'Englend' ? 'Email' : 'Почта'" required v-model="emailUser"></input>
            <input class="border-1 text-white border-white bg-inherit hover:bg-white/10 px-10 py-3 text-[20px] placeholder:text-white/10" type="password" :placeholder="settingUser.language == 'Englend' ? 'Password' : 'Пароль'" required minlength="8" v-model="passwordUser"></input>
            <button class="border-1 text-white/50 hover:text-white px-5 py-2 text-[20px]  border-white w-fit " @click.prevent="register()">{{settingUser.language == 'Englend' ? 'Register' : 'Зарегистрироваться'}}</button>
        </form>
        <NuxtLink class="text-[20px] text-white/50 hover:text-white" to="auth">{{settingUser.language == "Englend"? 'Do you have an account? Enter!' : 'Есть аккаунт? Войдите!'}}</NuxtLink>
    </section>
</template>

<style scoped lang="scss">
    
    
</style>