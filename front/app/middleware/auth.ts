import { Auth } from "~/composabels/Auth"
import { useUserStore } from "#imports"

export default defineNuxtRouteMiddleware(async () => {
    if (import.meta.server) return

    const userStore = useUserStore()
    const auth = new Auth()

   
    if (userStore.userData == undefined) {
        await auth.restoreSession()
    }

    const user = userStore.userData
    if (user == undefined) {
        return navigateTo('/auth')
    }
    if ((await auth.isAuthenticated(user)).sucess === false) {
        return navigateTo('/auth')
    }
})
