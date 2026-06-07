import { Auth } from "~/composabels/Auth"
import { useUserStore } from "#imports"

export default defineNuxtRouteMiddleware(async () => {
    // The access token lives in localStorage (client only). On the server there
    // is no token to restore from, so skip the check and let the client run it.
    if (import.meta.server) return

    const userStore = useUserStore()
    const auth = new Auth()

    // After a reload the Pinia store is empty but the token may still be in
    // localStorage — try to restore the session before redirecting.
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
