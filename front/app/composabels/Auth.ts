import { useUserStore } from '@/stores/user'
import type { AuthRespons, TokenResponse, TwoFARequired } from "../../types/api/respons"
import type { UserData } from "../../types/forms"
import type { User } from '~~/types/other'
import { apiClient, ApiError, setTokens, clearTokens } from "./apiClient"

class Auth {
    #noValidateData: UserData = { email: "", password: "" }
    #userStore

    constructor() {
        this.#userStore = useUserStore()
    }

    // Registration sends ONLY the three required fields (username, email,
    // password); firstname/lastname are optional on the backend and omitted.
    async register(username: string, email: string, password: string): Promise<AuthRespons> {
        try {
            await apiClient.signup({ username, email, password })
            return { sucess: true, data: "Регистрация прошла успешно" }
        } catch (e) {
            if (e instanceof ApiError) return { sucess: false, error: e.message }
            return { sucess: false, error: "Возникла непредвиденная ошибка" }
        }
    }

    async startAuth(user: UserData): Promise<AuthRespons> {
        this.#noValidateData = user
        if (!this.#validationData()) {
            return this.#returnStatus({ sucess: false, error: "Использованы запрещённые или не верные символы" })
        }
        return this.#returnStatus(await this.#authUser())
    }

    #validationData(): boolean {
        let error = 0
        const regSql = /\sOR\s\d=\d;\s\-\-/

        if (Object.keys(this.#noValidateData).indexOf("email") != -1) {
            const regEx = /^([a-zA-Z0-9\.\_\%\+\-\=\#]+)@([a-zA-Z]+)\.([a-zA-z]+)$/

            if (!regEx.test(this.#noValidateData.email) && regSql.test(this.#noValidateData.email)) {
                error++;
            }
        }
        if (Object.keys(this.#noValidateData).indexOf("password")) {
            const regExp = /^([\w\.\_\%\+\-\#]{12,})$/

            if (!regExp.test(this.#noValidateData.password) && regSql.test(this.#noValidateData.password)) {
                error++;
            }
        }

        return error == 0
    }

    async #authUser(): Promise<AuthRespons> {
        try {
            const res = await apiClient.signin({
                email: this.#noValidateData.email,
                password: this.#noValidateData.password,
            })

            if ((res as TwoFARequired).requires_two_fa) {
                return { sucess: false, data: res, error: "Требуется код двухфакторной аутентификации" }
            }

            setTokens(res as TokenResponse)
            await this.#loadCurrentUser((res as TokenResponse).access_token)
            return { sucess: true, data: "Вы успешно вошли" }
        } catch (e) {
            if (e instanceof ApiError) {
                return { sucess: false, error: e.message }
            }
            return { sucess: false, error: "Возникла непредвиденная ошибка" }
        }
    }

    // Decodes the user_id from the access-token JWT, fetches the user and puts
    // it into the store (the API has no dedicated "me" endpoint).
    async #loadCurrentUser(accessToken: string) {
        const userId = this.#userIdFromToken(accessToken)
        if (!userId) return
        try {
            const u = await apiClient.getUser(userId)
            const mapped: User = {
                id: String(u.id),
                name: u.firstname ?? "",
                secondName: u.lastname ?? "",
                userName: u.username ?? "",
                email: u.email ?? "",
                phone: "",
                birthday: "",
                country: "",
                bio: "",
                logo: u.avatar ?? "",
                password: "",
            }
            this.#userStore.userData = mapped
        } catch {
            // keep tokens even if profile fetch fails
        }
    }

    #userIdFromToken(token: string): string | null {
        try {
            const payload = token.split(".")[1]
            if (!payload) return null
            const json = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")))
            return json.user_id != null ? String(json.user_id) : null
        } catch {
            return null
        }
    }

    // Restores the session after a page reload: the Pinia store resets but the
    // access token survives in localStorage, so re-load the user from it.
    async restoreSession(): Promise<boolean> {
        if (this.#userStore.userData != undefined) return true
        if (typeof localStorage === "undefined") return false
        const token = localStorage.getItem("access_token")
        if (!token) return false
        await this.#loadCurrentUser(token)
        return this.#userStore.userData != undefined
    }

    // Completes a 2FA login using the temp token returned by signin.
    async verifyTwoFA(tempToken: string, code: string): Promise<AuthRespons> {
        try {
            const tokens = await apiClient.verify({ temp_token: tempToken, code })
            setTokens(tokens)
            await this.#loadCurrentUser(tokens.access_token)
            return { sucess: true, data: "Вы успешно вошли" }
        } catch (e) {
            if (e instanceof ApiError) return { sucess: false, error: e.message }
            return { sucess: false, error: "Возникла непредвиденная ошибка" }
        }
    }

    async logout(password: string): Promise<AuthRespons> {
        try {
            await apiClient.logout(password)
        } catch {
            // even if the call fails, drop local credentials
        }
        clearTokens()
        this.#userStore.userData = undefined
        return { sucess: true }
    }

    async isAuthenticated(user: User): Promise<AuthRespons> {
        if (!user.id) {
            return { sucess: false }
        }
        try {
            await apiClient.getUser(String(user.id))
            return { sucess: true }
        } catch {
            return { sucess: false }
        }
    }

    #returnStatus(value: AuthRespons): AuthRespons {
        return value
    }
}

export { Auth }
