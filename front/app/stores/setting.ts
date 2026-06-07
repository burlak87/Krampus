import type { genericRef, Setting } from "~~/types/other"


export const useSettingUser = defineStore("settingUser", () => {
    const language: genericRef<"Russian" | "Englend"> = ref("Englend")

    return {language}
})