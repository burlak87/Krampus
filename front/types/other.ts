import type { User } from "./api/respons";


export interface CallParticipant extends User {
    audio: boolean;
    video: boolean;
    muth: boolean;  
}

export interface Setting {
    chatBackup: "on" | "off",
    theme: "light" | "dark"
}



export interface genericRef<typeData> extends Ref {
    value: typeData | undefined
}

