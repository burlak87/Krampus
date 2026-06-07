import type { answerMessage, checkUserActive, startStreamMessage, StatusMessage } from "../../types/signaling"

export class Signaling {
    #webSoket
    #user = useUserStore()
    #idRoom
    #callStore = callStore()
    #pendingMessages: string[]
    #answerMessage: answerMessage = {
        type: "Answer",
        idUserAnswer: "",
        idUserTarget: "",
        action: null,
        stream_option: {},
        status: null
    }

    #startStreamMessage: startStreamMessage = {
        type: "StartStream",
        idRoom: "",
        maxUser: 0,
        system_option: {
            isAudio: true,
            isAudioRole: {},
            isVideo: true,
            isVideoRole: {},
            adminRoom: "",
            timeStartStream: ""
        }
    }

    #checkUserStatus: checkUserActive = {
        type: "checkUserActive",
        idUserTarget: "",
        idRoom: ""
    }

    constructor(webSoket: WebSocket, idUser: String, idRoom: String) {
        this.#webSoket = webSoket
        this.#idRoom = idRoom
        this.#pendingMessages = []
        this.#webSoket.onopen = () => {
            this.#pendingMessages.forEach(msg => this.#webSoket?.send(msg))
            this.#pendingMessages = []
        }
    }

    #reStructurAnswer(idUserAnswer: String, idUserTarget: String, action: String, statusMessage = "Sent", answer: object | "" = "") {
        const newAnswer = JSON.parse(JSON.stringify(this.#answerMessage))
        newAnswer.idUserAnswer = idUserAnswer
        newAnswer.idUserTarget = idUserTarget
        if (statusMessage == "Sent" && answer) {
            console.log("GenerateAnswerOffer")
            newAnswer.answer = answer
        }
        newAnswer.action = action
        //newAnswer.stream_option = streamOption.system_option
        newAnswer.status = statusMessage
        console.log(newAnswer)
        return JSON.stringify(newAnswer)
    }

    #reStructurStatus(statusUser: String, offer: object | "" = "", idUserAnswer: string | "" = "") {
        const newStatus = JSON.parse(JSON.stringify(this.#callStore.statusUser))
        newStatus.statusUser = statusUser
        newStatus.name = this.#user.userData?.userName
        newStatus.idUserTarget = this.#user.userData?.id
        newStatus.idRoom = this.#idRoom

        /*switch (Role) {
            case "Top":
                newStatus.Priority = 0.5
                break
        }*/

        newStatus.system_option = this.#callStore.settingCall

        if (statusUser == "Active" && offer) {
            newStatus.offer = offer
        }

        if(statusUser == "Active" && idUserAnswer) {
            newStatus.idUserAnswer = idUserAnswer
        }

        return JSON.stringify(newStatus)
    }

    #reStructureStartStream(): string {
        const newSignal = JSON.parse(JSON.stringify(this.#startStreamMessage))
        newSignal.idRoom = this.#idRoom
        newSignal.timeStartStream = (new Date()).getTime()

        return JSON.stringify(newSignal)
    }

    #reStructurCheckUser(preliminary: boolean) {
        const newStatus = JSON.parse(JSON.stringify(this.#checkUserStatus))
        newStatus.idUserTarget = this.#user.userData?.id
        newStatus.idRoom = this.#idRoom
        if (preliminary) {
            newStatus.preliminary = true
        } else {
            newStatus.preliminary = false
        }
        console.log(newStatus)
        return JSON.stringify(newStatus)
    }

    async createOfferRegenerate(pc, idUser: string) {
        try {
            console.log("Regenerate")
            const offer = await pc.createOffer();
            await pc.setLocalDescription(offer);
            this.#webSoket.send(this.#reStructurStatus("Active", offer, idUser))
        } catch (err) {
            console.error('Ошибка negotiation:', err);
        }
    }

    sendSignalStartStream() {
        if (this.#webSoket && this.#webSoket.readyState === WebSocket.OPEN) {
            this.#webSoket.send(this.#reStructureStartStream())
        } else if (this.#webSoket?.readyState === WebSocket.CONNECTING) {
            this.#pendingMessages.push(this.#reStructureStartStream())
        } else {
            console.warn('Невозможно отправить, сокет не открыт')
        }

    }

    sendSignalStatusUser(status: String, offer: object | "" = "") {
        if (this.#webSoket && this.#webSoket.readyState === WebSocket.OPEN) {
            if (offer != "") {
                this.#webSoket.send(this.#reStructurStatus(status, offer))
            } else {
                this.#webSoket.send(this.#reStructurStatus(status))
            }
        } else if (this.#webSoket?.readyState === WebSocket.CONNECTING) {
            if (offer != "") {
                this.#pendingMessages.push(this.#reStructurStatus(status, offer))
            } else {
                this.#pendingMessages.push(this.#reStructurStatus(status))
            }
        } else {
            console.warn('Невозможно отправить, сокет не открыт')
        }

    }

    sendSignalAnswer(idUserAnswer: string, idUserTraget: string, action: string, statusMessage?: string, answer?: object) {
        if (this.#webSoket && this.#webSoket.readyState === WebSocket.OPEN) {
            this.#webSoket.send(this.#reStructurAnswer(idUserAnswer, idUserTraget, action, statusMessage, answer))
        } else if (this.#webSoket?.readyState === WebSocket.CONNECTING) {
            this.#pendingMessages.push(this.#reStructurAnswer(idUserAnswer, idUserTraget, action, statusMessage, answer))
        } else {
            console.warn('Невозможно отправить, сокет не открыт')
        }
    }

    sendSignalCheckUser(preliminary: boolean) {
        if (this.#webSoket && this.#webSoket.readyState === WebSocket.OPEN) {
            this.#webSoket.send(this.#reStructurCheckUser(preliminary))
        } else if (this.#webSoket?.readyState === WebSocket.CONNECTING) {
            this.#pendingMessages.push(this.#reStructurCheckUser(preliminary))
        } else {
            console.warn('Невозможно отправить, сокет не открыт')
        }
    }

    sendSignalICECandidate(ice: string) {
        if (this.#webSoket && this.#webSoket.readyState === WebSocket.OPEN) {
            this.#webSoket.send(ice)
        } else if (this.#webSoket?.readyState === WebSocket.CONNECTING) {
            this.#pendingMessages.push(ice)
        } else {
            console.warn('Невозможно отправить, сокет не открыт')
        }
    }

    getIdRoom(): String {
        return this.#idRoom
    }
}